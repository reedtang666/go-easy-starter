//go:build integration

package scaffold

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
)

// TestGeneratedModuleAcceptance exercises the files users actually receive,
// including registration, build, migrations and authenticated HTTP requests.
func TestGeneratedModuleAcceptance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	clone := t.TempDir()
	copyProject(t, root, clone)
	env := isolatedEnvironment()
	binary := filepath.Join(t.TempDir(), "starter")
	runInClone(t, ctx, clone, env, "go", "build", "-o", binary, ".")
	// Exercise dispatch and --root from outside the project, without configuration.
	runInClone(t, ctx, t.TempDir(), env, binary, "module", "--root", clone, "--name", "article")
	if _, err := os.Stat(filepath.Join(clone, "manifest/sql/003_create_article_items.sql")); err != nil {
		t.Fatalf("module command did not create its numbered migration: %v", err)
	}
	runInClone(t, ctx, clone, env, "go", "build", "-o", binary, ".")
	t.Log("generated module builds before DAO generation")
	if gf := os.Getenv("STARTER_TEST_GF"); gf != "" {
		if !filepath.IsAbs(gf) {
			t.Fatal("STARTER_TEST_GF must be an absolute path to the pinned GoFrame CLI")
		}
		b, err := os.ReadFile(filepath.Join(clone, "hack/config.example.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(clone, "hack/config.yaml"), b, 0600); err != nil {
			t.Fatal(err)
		}
		runInClone(t, ctx, clone, env, gf, "gen", "ctrl")
		runInClone(t, ctx, clone, env, gf, "gen", "service")
		runInClone(t, ctx, clone, env, "go", "build", "-o", filepath.Join(t.TempDir(), "starter"), ".")
		t.Log("GoFrame gen ctrl/service preserves a compilable generated module")
	} else {
		t.Log("STARTER_TEST_GF is unset; GoFrame CLI compatibility was not executed")
	}
	databaseURL, db := moduleDatabase(t, ctx)
	env = append(env, "STARTER_DATABASE_URL="+databaseURL)
	runInClone(t, ctx, clone, env, "make", "setup")
	runInClone(t, ctx, clone, env, "make", "db-init")
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 2 {
		t.Fatalf("db-init must apply only the two original migrations: count=%d err=%v", count, err)
	}
	status := runInClone(t, ctx, clone, env, "make", "db-status")
	if !strings.Contains(status, "pending") || !strings.Contains(status, "003_create_article_items.sql") {
		t.Fatalf("generated module must appear as pending: %s", status)
	}
	runInClone(t, ctx, clone, env, "make", "db-up")
	runInClone(t, ctx, clone, env, "make", "db-new", "NAME=add_article_status")
	if err := os.WriteFile(filepath.Join(clone, "manifest/sql/004_add_article_status.sql"), []byte("ALTER TABLE article_items ADD COLUMN status TEXT NOT NULL DEFAULT 'draft';\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runInClone(t, ctx, clone, env, "make", "db-up")
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count); err != nil || count != 4 {
		t.Fatalf("expected four applied migrations: count=%d err=%v", count, err)
	}
	if output := runInClone(t, ctx, clone, env, "make", "db-up"); !strings.Contains(output, "applied 0 migration(s)") {
		t.Fatalf("repeated db-up must be a no-op: %s", output)
	}
	if output := runInClone(t, ctx, clone, env, "make", "db-status"); strings.Contains(output, "pending") || strings.Count(output, "applied") != 4 {
		t.Fatalf("expected four applied statuses: %s", output)
	}
	t.Log("clean-project CLI setup/init/status/up/new and repeat upgrade passed")
	if address := os.Getenv("STARTER_TEST_REDIS_ADDRESS"); address == "" {
		t.Fatal("STARTER_TEST_REDIS_ADDRESS must explicitly name a test Redis instance")
	}
	env = append(env, "STARTER_TEST_DATABASE_URL="+databaseURL, "STARTER_TEST_REDIS_ADDRESS="+os.Getenv("STARTER_TEST_REDIS_ADDRESS"))
	fixture, err := os.ReadFile(filepath.Join(root, "internal/scaffold/testdata/generated_acceptance_test.go.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "internal/bootstrap/generated_acceptance_test.go"), fixture, 0644); err != nil {
		t.Fatal(err)
	}
	runInClone(t, ctx, clone, env, "go", "test", "-tags=integration", "./internal/bootstrap", "-run", "^TestGeneratedModuleAcceptance$", "-count=1", "-timeout=90s")
	t.Log("generated module migration and real PostgreSQL/Redis HTTP acceptance passed")
}

func isolatedEnvironment() []string {
	var env []string
	for _, value := range os.Environ() {
		name := strings.SplitN(value, "=", 2)[0]
		if strings.HasPrefix(name, "STARTER_") || strings.HasPrefix(name, "GF_GCFG_") {
			continue
		}
		env = append(env, value)
	}
	return env
}

func runInClone(t *testing.T, ctx context.Context, root string, env []string, binary string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir, cmd.Env = root, env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", filepath.Base(binary), strings.Join(args, " "), err, output)
	}
	return string(output)
}

func copyProject(t *testing.T, root, destination string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".tools" || entry.Name() == "bin" || entry.Name() == "storage") {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if rel == "api" || strings.HasPrefix(rel, "api/") || rel == "internal" || strings.HasPrefix(rel, "internal/") || rel == "manifest" || rel == "manifest/sql" || rel == "hack" {
				return os.MkdirAll(filepath.Join(destination, filepath.FromSlash(rel)), 0755)
			}
			return filepath.SkipDir
		}
		if entry.Name() == "config.yaml" || strings.HasPrefix(entry.Name(), ".env") {
			return nil
		}
		if rel != "go.mod" && rel != "go.sum" && rel != "main.go" && rel != "Makefile" && !strings.HasPrefix(rel, "api/") && !strings.HasPrefix(rel, "internal/") && !strings.HasPrefix(rel, "manifest/sql/") && rel != "hack/config.example.yaml" {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".lock") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(destination, filepath.FromSlash(rel)), b, 0644)
	})
	if err != nil {
		t.Fatalf("copy source-only project: %v", err)
	}
}

func moduleDatabase(t *testing.T, ctx context.Context) (string, *sql.DB) {
	t.Helper()
	raw := os.Getenv("STARTER_TEST_DATABASE_URL")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || !strings.HasSuffix(strings.TrimPrefix(u.Path, "/"), "_test") {
		t.Fatal("STARTER_TEST_DATABASE_URL must explicitly name an isolated PostgreSQL *_test database")
	}
	admin, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal("open PostgreSQL test connection failed")
	}
	var allowed bool
	if err := admin.QueryRowContext(ctx, "SELECT rolcreatedb OR rolsuper FROM pg_roles WHERE rolname=current_user").Scan(&allowed); err != nil {
		admin.Close()
		t.Fatal("cannot inspect test database role permissions")
	}
	if !allowed {
		admin.Close()
		t.Fatal("generated-module acceptance requires CREATEDB permission to create its own isolated random *_test database")
	}
	var random [10]byte
	if _, err := rand.Read(random[:]); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	name := "starter_module_" + hex.EncodeToString(random[:]) + "_test"
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+pq.QuoteIdentifier(name)); err != nil {
		admin.Close()
		t.Fatalf("create isolated module database failed: %v", err)
	}
	var db *sql.DB
	t.Cleanup(func() {
		if db != nil {
			db.Close()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.ExecContext(cleanupCtx, "DROP DATABASE "+pq.QuoteIdentifier(name)+" WITH (FORCE)"); err != nil {
			t.Errorf("drop only this test's random module database: %v", err)
		}
		admin.Close()
	})
	u.Path, u.RawPath = "/"+name, ""
	query := u.Query()
	query.Del("search_path")
	u.RawQuery = query.Encode()
	db, err = sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal("open isolated module database failed")
	}
	return u.String(), db
}
