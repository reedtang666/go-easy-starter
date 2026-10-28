package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/lib/pq"
)

// Baseline is deliberately restricted to the unmodified built-in SQL. Generic
// force/version flags cannot prove that an existing schema matches a script.
var initialChecksums = map[int64]string{
	1: "6ce8f3d92eed9203ce28075691255315235055b87bbef6647ef60ee295d02175",
	2: "337d98094e6789d78c8287b830cfdf47624d059700c354f158d0e4ffdfb83463",
}

// Baseline adopts only the original four-table starter schema. It validates
// columns and constraints, then records migrations 1 and 2 atomically without
// running their SQL or modifying the application's tables.
func (r *Runner) Baseline(ctx context.Context, db *sql.DB) (int, error) {
	initial, err := r.initialFiles()
	if err != nil {
		return 0, err
	}
	for _, f := range initial {
		if f.SHA256 != initialChecksums[f.Version] {
			return 0, fmt.Errorf("db-baseline requires the unmodified built-in %s; automatic adoption of custom SQL is unsupported", f.Filename)
		}
	}
	err = withLock(ctx, db, func(conn *sql.Conn, schema string) error {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("start baseline validation: %w", err)
		}
		defer tx.Rollback()
		records, err := loadRecords(ctx, tx, schema)
		if err != nil {
			return err
		}
		if len(records) != 0 {
			return errors.New("migration history already exists; use db-status and db-up")
		}
		for _, table := range originalTables {
			exists, err := tableExists(ctx, tx, schema, table.name)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("baseline schema mismatch: required table %s is missing", table.name)
			}
		}
		var locked []string
		for _, table := range originalTables {
			locked = append(locked, qualified(schema, table.name))
		}
		if _, err := tx.ExecContext(ctx, "LOCK TABLE "+strings.Join(locked, ",")+" IN ACCESS SHARE MODE"); err != nil {
			return fmt.Errorf("lock baseline tables: %w", err)
		}
		for _, table := range originalTables {
			if err := validateColumns(ctx, tx, schema, table); err != nil {
				return err
			}
			if err := validateConstraints(ctx, tx, schema, table); err != nil {
				return err
			}
		}
		if err := ensureLedger(ctx, tx, schema); err != nil {
			return err
		}
		for _, f := range initial {
			if err := insertRecord(ctx, tx, schema, f); err != nil {
				return fmt.Errorf("record baseline %s: %w", f.Filename, err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit baseline: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(initial), nil
}

type columnSpec struct {
	name, typ string
	notNull   bool
	def       string
}

type constraintSpec struct {
	kind, columns, references, referencedColumns, onDelete, check string
}

type tableSpec struct {
	name        string
	columns     []columnSpec
	constraints []constraintSpec
}

var originalTables = []tableSpec{
	{"users", []columnSpec{
		{"id", "bigint", true, "serial"}, {"username", "character varying(32)", true, ""},
		{"password_hash", "text", true, ""}, {"nickname", "character varying(80)", true, "empty"},
		{"avatar_id", "bigint", false, ""}, {"status", "smallint", true, "1"},
		{"credential_version", "bigint", true, "1"}, {"created_at", "timestamp with time zone", true, "now"},
		{"updated_at", "timestamp with time zone", true, "now"},
	}, []constraintSpec{
		{kind: "p", columns: "id"}, {kind: "u", columns: "username"},
		{kind: "f", columns: "avatar_id,id", references: "attachments", referencedColumns: "id,owner_id", onDelete: "r"},
		{kind: "c", check: "checkusername=lowerusername"}, {kind: "c", check: "checkstatus=anyarray[0,1]"},
		{kind: "c", check: "checkcredential_version>0"},
	}},
	{"attachments", []columnSpec{
		{"id", "bigint", true, "serial"}, {"owner_id", "bigint", true, ""}, {"storage_name", "text", true, ""},
		{"driver", "character varying(16)", true, ""}, {"bucket", "text", true, "empty"}, {"object_key", "text", true, ""},
		{"name", "text", true, ""}, {"mime_type", "text", true, ""}, {"size", "bigint", true, ""},
		{"digest", "character varying(64)", true, ""}, {"upload_id", "character varying(64)", false, ""},
		{"private", "boolean", true, "true"}, {"created_at", "timestamp with time zone", true, "now"},
	}, []constraintSpec{
		{kind: "p", columns: "id"}, {kind: "u", columns: "upload_id"},
		{kind: "u", columns: "storage_name,object_key"}, {kind: "u", columns: "id,owner_id"},
		{kind: "f", columns: "owner_id", references: "users", referencedColumns: "id", onDelete: "a"},
		{kind: "c", check: "checksize>=0"},
	}},
	{"example_items", []columnSpec{
		{"id", "bigint", true, "serial"}, {"owner_id", "bigint", true, ""}, {"title", "character varying(200)", true, ""},
		{"content", "text", true, "empty"}, {"task_runs", "integer", true, "0"},
		{"created_at", "timestamp with time zone", true, "now"}, {"updated_at", "timestamp with time zone", true, "now"},
	}, []constraintSpec{
		{kind: "p", columns: "id"}, {kind: "f", columns: "owner_id", references: "users", referencedColumns: "id", onDelete: "a"},
	}},
	{"example_task_receipts", []columnSpec{
		{"task_key", "character varying(128)", true, ""}, {"item_id", "bigint", true, ""},
		{"created_at", "timestamp with time zone", true, "now"},
	}, []constraintSpec{
		{kind: "p", columns: "task_key"}, {kind: "f", columns: "item_id", references: "example_items", referencedColumns: "id", onDelete: "c"},
	}},
}

func validateColumns(ctx context.Context, q queryer, schema string, table tableSpec) error {
	rows, err := q.QueryContext(ctx, `SELECT a.attname, format_type(a.atttypid,a.atttypmod), a.attnotnull,
COALESCE(pg_get_expr(d.adbin,d.adrelid),'')
FROM pg_attribute a JOIN pg_class t ON t.oid=a.attrelid
LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
WHERE t.oid=to_regclass($1) AND t.relkind IN ('r','p') AND a.attnum>0 AND NOT a.attisdropped`, qualified(schema, table.name))
	if err != nil {
		return fmt.Errorf("inspect baseline columns %s: %w", table.name, err)
	}
	defer rows.Close()
	actual := map[string]columnSpec{}
	for rows.Next() {
		var c columnSpec
		if err := rows.Scan(&c.name, &c.typ, &c.notNull, &c.def); err != nil {
			return fmt.Errorf("decode baseline columns %s: %w", table.name, err)
		}
		actual[c.name] = c
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read baseline columns %s: %w", table.name, err)
	}
	for _, expected := range table.columns {
		c, ok := actual[expected.name]
		if !ok || c.typ != expected.typ || c.notNull != expected.notNull || !defaultMatches(c.def, expected.def) {
			return fmt.Errorf("baseline schema mismatch: %s.%s must match the built-in type, nullability and default", table.name, expected.name)
		}
	}
	return nil
}

type actualConstraint struct {
	kind, columns, referenceSchema, referenceTable, referenceColumns, onDelete, onUpdate, match, check string
	validated, enforced, deferrable, deferred                                                          bool
}

func validateConstraints(ctx context.Context, q queryer, schema string, table tableSpec) error {
	rows, err := q.QueryContext(ctx, `SELECT c.contype::text,
ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY k(num,pos) JOIN pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.num ORDER BY k.pos),
COALESCE(n.nspname,''), COALESCE(t.relname,''),
ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY k(num,pos) JOIN pg_attribute a ON a.attrelid=c.confrelid AND a.attnum=k.num ORDER BY k.pos),
c.confdeltype::text,c.confupdtype::text,c.confmatchtype::text,c.convalidated,COALESCE((to_jsonb(c)->>'conenforced')::boolean,true),c.condeferrable,c.condeferred,pg_get_constraintdef(c.oid)
FROM pg_constraint c LEFT JOIN pg_class t ON t.oid=c.confrelid LEFT JOIN pg_namespace n ON n.oid=t.relnamespace
WHERE c.conrelid=to_regclass($1)`, qualified(schema, table.name))
	if err != nil {
		return fmt.Errorf("inspect baseline constraints %s: %w", table.name, err)
	}
	defer rows.Close()
	var actual []actualConstraint
	for rows.Next() {
		var c actualConstraint
		var columns, referenceColumns pq.StringArray
		if err := rows.Scan(&c.kind, &columns, &c.referenceSchema, &c.referenceTable, &referenceColumns, &c.onDelete, &c.onUpdate, &c.match, &c.validated, &c.enforced, &c.deferrable, &c.deferred, &c.check); err != nil {
			return fmt.Errorf("decode baseline constraints %s: %w", table.name, err)
		}
		c.columns, c.referenceColumns, c.check = strings.Join(columns, ","), strings.Join(referenceColumns, ","), normalizeCheck(c.check)
		actual = append(actual, c)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read baseline constraints %s: %w", table.name, err)
	}
	for _, expected := range table.constraints {
		found := false
		for _, c := range actual {
			if c.kind != expected.kind || !c.validated || !c.enforced || c.deferrable || c.deferred {
				continue
			}
			if expected.kind == "c" {
				found = c.check == expected.check
			} else if c.columns == expected.columns {
				found = expected.kind != "f" || c.referenceSchema == schema && c.referenceTable == expected.references && c.referenceColumns == expected.referencedColumns && c.onDelete == expected.onDelete && c.onUpdate == "a" && c.match == "s"
			}
			if found {
				break
			}
		}
		if !found {
			return fmt.Errorf("baseline schema mismatch: %s is missing its original %s constraint (%s%s)", table.name, expected.kind, expected.columns, expected.check)
		}
	}
	return nil
}

var defaultCast = regexp.MustCompile(`::(?:character varying(?:\([0-9]+\))?|text|smallint|bigint|integer|boolean)`)

func defaultMatches(actual, expected string) bool {
	s := strings.ToLower(strings.TrimSpace(actual))
	switch expected {
	case "serial":
		return strings.HasPrefix(s, "nextval('") && strings.HasSuffix(s, "'::regclass)")
	case "now":
		return s == "current_timestamp" || s == "now()"
	case "empty":
		return defaultCast.ReplaceAllString(s, "") == "''"
	default:
		return defaultCast.ReplaceAllString(s, "") == expected
	}
}

func normalizeCheck(source string) string {
	source = defaultCast.ReplaceAllString(strings.ToLower(source), "")
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '(' || r == ')' {
			return -1
		}
		return r
	}, source)
}
