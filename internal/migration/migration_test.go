package migration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func writeSQL(t *testing.T, dir, name, sql string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(sql), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestNewLoadsOrderedVersionsAndOriginalChecksums(t *testing.T) {
	dir := t.TempDir()
	writeSQL(t, dir, "010_later.sql", "SELECT 2;\n")
	writeSQL(t, dir, "002_first.sql", "BEGIN;\nSELECT 1;\nCOMMIT;\n")
	writeSQL(t, dir, "notes.md", "ignored")
	r, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.files) != 2 || r.files[0].Version != 2 || r.files[1].Version != 10 {
		t.Fatalf("wrong order: %#v", r.files)
	}
	// Compare the source bytes rather than the transaction-stripped payload.
	if r.files[0].SHA256 != checksum([]byte("BEGIN;\nSELECT 1;\nCOMMIT;\n")) {
		t.Fatal("checksum does not cover original SQL")
	}
}

func TestCreateWithContentWritesOnceAndPreservesBody(t *testing.T) {
	dir := t.TempDir()
	body := "CREATE TABLE articles (id BIGINT PRIMARY KEY);\n"
	path, err := CreateWithContent(dir, "create_articles", body)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != body {
		t.Fatalf("body changed: %q %v", b, err)
	}
}

func TestBaselineColumnDefaultsAndConstraintNormalization(t *testing.T) {
	for _, tc := range []struct{ actual, expected string }{
		{"''::character varying", "empty"}, {"''::text", "empty"}, {"1", "1"},
		{"nextval('users_id_seq'::regclass)", "serial"}, {"CURRENT_TIMESTAMP", "now"}, {"true", "true"},
	} {
		if !defaultMatches(tc.actual, tc.expected) {
			t.Fatalf("default %q should match %q", tc.actual, tc.expected)
		}
	}
	if defaultMatches("0", "1") || defaultMatches("", "serial") || defaultMatches("'unexpected'::text", "empty") {
		t.Fatal("incompatible default accepted")
	}
	if normalizeCheck("CHECK ((username)::text = lower((username)::text))") != "checkusername=lowerusername" {
		t.Fatal("unexpected check normalization")
	}
}

func TestNewRejectsInvalidMigrations(t *testing.T) {
	for _, tc := range []struct{ name, first, second string }{
		{"duplicate", "001_one.sql", "01_two.sql"},
		{"zero", "000_invalid.sql", ""},
		{"bad name", "001_Upper.sql", ""},
		{"missing version", "invalid.sql", ""},
		{"overflow", "999999999999999999999999999_bad.sql", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeSQL(t, dir, tc.first, "SELECT 1;")
			if tc.second != "" {
				writeSQL(t, dir, tc.second, "SELECT 2;")
			}
			if _, err := New(dir); err == nil {
				t.Fatal("invalid SQL filename accepted")
			}
		})
	}
}

func TestPrepareSQLKeepsQuotedTransactionsAndComments(t *testing.T) {
	for _, source := range []string{
		"BEGIN; SELECT 'COMMIT; ROLLBACK;'; COMMIT;",
		"-- initial\nBEGIN; /* nested /* BEGIN; */ comment */ SELECT 1; COMMIT; -- final",
		"BEGIN WORK; CREATE FUNCTION f() RETURNS void LANGUAGE plpgsql AS $body$ BEGIN RAISE NOTICE 'COMMIT;'; END; $body$; COMMIT WORK;",
		"SELECT $$BEGIN; COMMIT;$$, $tag$ROLLBACK;$tag$;",
		"SELECT $标签$COMMIT; BEGIN;$标签$;",
		`SELECT E'escaped \' COMMIT; still quoted';`,
		"CREATE TABLE \"COMMIT;\" (\"BEGIN\" text);",
	} {
		t.Run(source, func(t *testing.T) {
			out, err := prepareSQL(source)
			if err != nil || strings.TrimSpace(out) == "" {
				t.Fatalf("valid SQL rejected: %v", err)
			}
		})
	}
}

func TestPrepareSQLRejectsTransactionEscapesAndMalformedSQL(t *testing.T) {
	for _, source := range []string{
		"", "-- nothing", "BEGIN; COMMIT;", "BEGIN; SELECT 1;", "SELECT 1; COMMIT;",
		"BEGIN; SELECT 1; COMMIT; SELECT 2;", "SELECT 1; ROLLBACK;", "SELECT 1; END;",
		"START TRANSACTION; SELECT 1; COMMIT;", "BEGIN; SELECT 1; SAVEPOINT mine; COMMIT;",
		"PREPARE TRANSACTION 'mine';", "SET TRANSACTION READ ONLY;", "ABORT;",
		"SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;", "RELEASE SAVEPOINT mine;",
		"SELECT 'unterminated", "SELECT $$unterminated", "/* unterminated", "SELECT \"unterminated",
		"SELECT 1; /* COMMIT */ ROLLBACK TO SAVEPOINT x;",
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := prepareSQL(source); err == nil {
				t.Fatal("unsafe or malformed SQL accepted")
			}
		})
	}
}

func TestCreateAllocatesUniqueVersionsAndDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	writeSQL(t, dir, "009_existing.sql", "SELECT 1;")
	const count = 8
	paths := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := Create(dir, "add_articles")
			paths <- p
			errs <- err
		}()
	}
	wg.Wait()
	close(paths)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for p := range paths {
		if seen[p] {
			t.Fatalf("duplicate file: %s", p)
		}
		seen[p] = true
	}
	r, err := New(dir)
	if err != nil || len(r.files) != count+1 || r.files[count].Version != 17 {
		t.Fatalf("wrong allocations: %#v %v", r, err)
	}
	for _, name := range []string{"../escape", "CamelCase", "bad-name", "_leading", "trailing_", "double__word", "", "123_name"} {
		if _, err := Create(dir, name); err == nil {
			t.Fatalf("invalid name %q accepted", name)
		}
	}
}

func TestCompareHistoryReportsDriftAndMissingVersions(t *testing.T) {
	files := []file{{Version: 1, Filename: "001_init.sql", SHA256: "new"}, {Version: 3, Filename: "003_added.sql", SHA256: "match"}}
	records := []record{{Version: 1, Filename: "001_init.sql", SHA256: "old"}, {Version: 2, Filename: "002_missing.sql", SHA256: "old"}, {Version: 3, Filename: "003_old_name.sql", SHA256: "match"}}
	statuses := compareHistory(files, records)
	if len(statuses) != 3 || statuses[0].State != "modified" || statuses[1].State != "missing" || statuses[2].State != "renamed" {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
	if err := validateHistory(statuses); err == nil {
		t.Fatal("drift accepted")
	}
}

func TestHistoryRejectsOutOfOrderAndAcceptsPendingTail(t *testing.T) {
	if err := validateHistory([]Status{{Version: 1, Filename: "001_old.sql", State: "pending"}, {Version: 2, Filename: "002_applied.sql", State: "applied"}}); err == nil {
		t.Fatal("out-of-order migration accepted")
	}
	if err := validateHistory([]Status{{Version: 2, State: "applied"}, {Version: 3, State: "pending"}}); err != nil {
		t.Fatal(err)
	}
}

func TestInitialFilesRequireExactBuiltinNames(t *testing.T) {
	for _, files := range [][]file{
		{}, {{Version: 1, Filename: "001_init.sql"}},
		{{Version: 1, Filename: "001_custom.sql"}, {Version: 2, Filename: "002_example.sql"}},
		{{Version: 1, Filename: "001_init.sql"}, {Version: 2, Filename: "002_custom.sql"}},
	} {
		if _, err := (&Runner{files: files}).initialFiles(); err == nil {
			t.Fatalf("invalid initial files accepted: %#v", files)
		}
	}
	r := &Runner{files: []file{{Version: 1, Filename: "001_init.sql"}, {Version: 2, Filename: "002_example.sql"}, {Version: 3, Filename: "003_later.sql"}}}
	if files, err := r.initialFiles(); err != nil || len(files) != 2 {
		t.Fatalf("built-in files with later SQL should work: %#v %v", files, err)
	}
}

func TestBaselineRequiresCanonicalInitialSources(t *testing.T) {
	r, err := New(filepath.Join("..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	initial, err := r.initialFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range initial {
		if f.SHA256 != initialChecksums[f.Version] {
			t.Fatalf("built-in baseline fingerprint is stale for %s", f.Filename)
		}
	}
	initial[0].SHA256 = "changed"
	if _, err := (&Runner{files: initial}).Baseline(context.Background(), nil); err == nil {
		t.Fatal("baseline accepted changed initial SQL")
	}
}

func TestNewRejectsSymlinksAndMissingDirectory(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing directory accepted")
	}
	writeSQL(t, dir, "source.txt", "SELECT 1;")
	if err := os.Symlink(filepath.Join(dir, "source.txt"), filepath.Join(dir, "001_link.sql")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir); err == nil {
		t.Fatal("migration symlink accepted")
	}
}
