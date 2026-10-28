//go:build integration

package migration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func isolatedDatabase(t *testing.T) *sql.DB {
	t.Helper()
	raw := os.Getenv("STARTER_TEST_DATABASE_URL")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || !strings.HasSuffix(strings.TrimPrefix(u.Path, "/"), "_test") {
		t.Fatal("STARTER_TEST_DATABASE_URL must explicitly name a PostgreSQL database ending in _test")
	}
	admin, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal("open isolated migration test database")
	}
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	schema := "migration_test_" + hex.EncodeToString(b[:])
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+qualifiedSchema(schema)); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal("open schema-scoped migration database")
	}
	t.Cleanup(func() {
		db.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(cleanupCtx, "DROP SCHEMA "+qualifiedSchema(schema)+" CASCADE"); err != nil {
			t.Errorf("clean isolated migration schema: %v", err)
		}
		admin.Close()
	})
	return db
}

func qualifiedSchema(schema string) string { return `"` + schema + `"` }

func originalDirectory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"001_init.sql", "002_example.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "manifest", "sql", name))
		if err != nil {
			t.Fatal(err)
		}
		writeSQL(t, dir, name, string(b))
	}
	return dir
}

func mustRunner(t *testing.T, dir string) *Runner {
	t.Helper()
	r, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func mustExec(t *testing.T, db *sql.DB, source string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), source); err != nil {
		t.Fatal(err)
	}
}

func existsInTest(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRowContext(context.Background(), "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	return exists
}

func TestIntegrationInitUpStatusAndAppliedDrift(t *testing.T) {
	db := isolatedDatabase(t)
	dir := originalDirectory(t)
	writeSQL(t, dir, "003_articles.sql", "CREATE TABLE articles(id BIGINT PRIMARY KEY);")
	r := mustRunner(t, dir)
	ctx := context.Background()
	statuses, err := r.Status(ctx, db)
	if err != nil || len(statuses) != 3 || statuses[0].State != "pending" || existsInTest(t, db, "schema_migrations") {
		t.Fatalf("status must be read-only and pending: %#v %v", statuses, err)
	}
	if n, err := r.Init(ctx, db); err != nil || n != 2 || existsInTest(t, db, "articles") {
		t.Fatalf("Init must apply only the two initial files: %d %v", n, err)
	}
	if _, err := r.Init(ctx, db); err == nil {
		t.Fatal("Init accepted an initialized database")
	}
	if n, err := r.Up(ctx, db); err != nil || n != 1 {
		t.Fatalf("Up did not apply remaining migration: %d %v", n, err)
	}
	if n, err := r.Up(ctx, db); err != nil || n != 0 {
		t.Fatalf("Up must be repeatable: %d %v", n, err)
	}
	statuses, err = r.Status(ctx, db)
	if err != nil || statuses[2].AppliedAt == nil || statuses[2].State != "applied" {
		t.Fatalf("missing applied status: %#v %v", statuses, err)
	}
	writeSQL(t, dir, "003_articles.sql", "CREATE TABLE articles(id BIGINT PRIMARY KEY); -- edited")
	r = mustRunner(t, dir)
	statuses, err = r.Status(ctx, db)
	if err != nil || statuses[2].State != "modified" {
		t.Fatalf("modified SQL not visible in status: %#v %v", statuses, err)
	}
	if _, err := r.Up(ctx, db); err == nil {
		t.Fatal("Up accepted modified applied SQL")
	}
	if err := os.Rename(filepath.Join(dir, "003_articles.sql"), filepath.Join(dir, "003_renamed.sql")); err != nil {
		t.Fatal(err)
	}
	r = mustRunner(t, dir)
	statuses, err = r.Status(ctx, db)
	if err != nil || statuses[2].State != "renamed" {
		t.Fatalf("renamed SQL not visible: %#v %v", statuses, err)
	}
	if _, err := r.Up(ctx, db); err == nil {
		t.Fatal("Up accepted renamed applied SQL")
	}
	if err := os.Remove(filepath.Join(dir, "003_renamed.sql")); err != nil {
		t.Fatal(err)
	}
	r = mustRunner(t, dir)
	statuses, err = r.Status(ctx, db)
	if err != nil || statuses[2].State != "missing" {
		t.Fatalf("missing SQL not visible: %#v %v", statuses, err)
	}
	if _, err := r.Up(ctx, db); err == nil {
		t.Fatal("Up accepted missing applied SQL")
	}
}

func TestIntegrationFailedMigrationRollsBackSQLAndHistory(t *testing.T) {
	db := isolatedDatabase(t)
	dir := t.TempDir()
	writeSQL(t, dir, "001_first.sql", "CREATE TABLE first_item(id BIGINT PRIMARY KEY);")
	writeSQL(t, dir, "002_failure.sql", "CREATE TABLE must_rollback(id BIGINT); INSERT INTO absent_table VALUES(1);")
	r := mustRunner(t, dir)
	if n, err := r.Up(context.Background(), db); err == nil || n != 1 {
		t.Fatalf("expected one committed file followed by error: %d %v", n, err)
	}
	if !existsInTest(t, db, "first_item") || existsInTest(t, db, "must_rollback") {
		t.Fatal("failed migration left partial business tables")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed migration was recorded: %d %v", count, err)
	}
}

func TestIntegrationLegacyBaselineThenUpgrade(t *testing.T) {
	db := isolatedDatabase(t)
	dir := originalDirectory(t)
	r := mustRunner(t, dir)
	for _, f := range r.files {
		mustExec(t, db, f.SQL)
	}
	ctx := context.Background()
	if _, err := r.Up(ctx, db); !errors.Is(err, ErrBaselineRequired) || existsInTest(t, db, "schema_migrations") {
		t.Fatalf("old untracked schema must require explicit adoption: %v", err)
	}
	if n, err := r.Baseline(ctx, db); err != nil || n != 2 {
		t.Fatalf("baseline failed: %d %v", n, err)
	}
	if _, err := r.Baseline(ctx, db); err == nil {
		t.Fatal("baseline accepted existing history")
	}
	writeSQL(t, dir, "003_add_title.sql", "ALTER TABLE users ADD COLUMN display_title TEXT;")
	r = mustRunner(t, dir)
	if n, err := r.Up(ctx, db); err != nil || n != 1 {
		t.Fatalf("post-baseline upgrade failed: %d %v", n, err)
	}
}

func TestIntegrationBaselineRejectsIncompatibleSchema(t *testing.T) {
	for _, change := range []string{
		"ALTER TABLE users ALTER COLUMN username TYPE VARCHAR(64)",
		"ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL",
		"ALTER TABLE users ALTER COLUMN status SET DEFAULT 0",
		"ALTER TABLE users DROP CONSTRAINT users_username_key",
		"ALTER TABLE users DROP CONSTRAINT user_avatar_owner_fk",
		"ALTER TABLE attachments DROP CONSTRAINT attachments_size_check",
		"ALTER TABLE attachments DROP CONSTRAINT attachments_size_check; ALTER TABLE attachments ADD CHECK(size >= 0) NOT VALID",
		"DROP TABLE example_task_receipts",
		"ALTER TABLE example_task_receipts DROP CONSTRAINT example_task_receipts_item_id_fkey; ALTER TABLE example_task_receipts ADD FOREIGN KEY(item_id) REFERENCES example_items(id)",
	} {
		t.Run(change, func(t *testing.T) {
			db := isolatedDatabase(t)
			r := mustRunner(t, originalDirectory(t))
			for _, f := range r.files {
				mustExec(t, db, f.SQL)
			}
			mustExec(t, db, change)
			if _, err := r.Baseline(context.Background(), db); err == nil || existsInTest(t, db, "schema_migrations") {
				t.Fatalf("incompatible schema adopted or ledger left behind: %v", err)
			}
		})
	}
}

func TestIntegrationConcurrentUpExecutesEachFileOnce(t *testing.T) {
	db := isolatedDatabase(t)
	dir := t.TempDir()
	writeSQL(t, dir, "001_once.sql", "CREATE TABLE once_only(id BIGINT PRIMARY KEY); SELECT pg_sleep(0.1); INSERT INTO once_only VALUES(1);")
	r := mustRunner(t, dir)
	const workers = 4
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	counts := make(chan int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := r.Up(context.Background(), db)
			errs <- err
			counts <- n
		}()
	}
	wg.Wait()
	close(errs)
	close(counts)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var total int
	for n := range counts {
		total += n
	}
	if total != 1 {
		t.Fatalf("same migration executed more than once: %d", total)
	}
}

func TestIntegrationFunctionBodyAndOuterTransaction(t *testing.T) {
	db := isolatedDatabase(t)
	dir := t.TempDir()
	writeSQL(t, dir, "001_function.sql", `-- comments are outside the wrapper
BEGIN;
CREATE FUNCTION quoted_transaction() RETURNS TEXT LANGUAGE plpgsql AS $body$
BEGIN
 RETURN 'COMMIT; BEGIN;';
END;
$body$;
CREATE TABLE function_result(value TEXT);
INSERT INTO function_result VALUES(quoted_transaction());
COMMIT;`)
	if n, err := mustRunner(t, dir).Up(context.Background(), db); err != nil || n != 1 {
		t.Fatalf("quoted function SQL rejected: %d %v", n, err)
	}
	var result string
	if err := db.QueryRow("SELECT value FROM function_result").Scan(&result); err != nil || result != "COMMIT; BEGIN;" {
		t.Fatalf("function body damaged: %q %v", result, err)
	}
}

func TestIntegrationCancelledLockWaitDoesNotRunMigration(t *testing.T) {
	db := isolatedDatabase(t)
	dir := t.TempDir()
	writeSQL(t, dir, "001_wait.sql", "CREATE TABLE after_wait(id BIGINT);")
	ctx := context.Background()
	blocker, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	schema, err := currentSchema(ctx, blocker)
	if err != nil {
		t.Fatal(err)
	}
	key := "go-easy-starter:migrations:" + schema
	if _, err := blocker.ExecContext(ctx, "SELECT pg_advisory_lock(hashtext(current_database()), hashtext($1))", key); err != nil {
		t.Fatal(err)
	}
	defer blocker.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtext(current_database()), hashtext($1))", key)
	waitCtx, cancel := context.WithTimeout(ctx, 40*time.Millisecond)
	defer cancel()
	if _, err := mustRunner(t, dir).Up(waitCtx, db); err == nil || existsInTest(t, db, "after_wait") {
		t.Fatalf("cancelled lock waiter executed SQL: %v", err)
	}
	if _, err := blocker.ExecContext(ctx, "SELECT pg_advisory_unlock(hashtext(current_database()), hashtext($1))", key); err != nil {
		t.Fatal(err)
	}
	if n, err := mustRunner(t, dir).Up(ctx, db); err != nil || n != 1 {
		t.Fatalf("cancelled wait left a stuck session lock: %d %v", n, err)
	}
}
