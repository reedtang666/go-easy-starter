package migration

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/lib/pq"
)

var ErrBaselineRequired = errors.New("existing users table has no migration history; verify it with db-baseline before db-up")

// Status describes both local SQL and historical rows. Drift is returned as a
// state so db-status can display it without changing the database.
type Status struct {
	Version   int64      `json:"version"`
	Filename  string     `json:"filename"`
	SHA256    string     `json:"sha256"`
	State     string     `json:"state"`
	AppliedAt *time.Time `json:"appliedAt,omitempty"`
	Detail    string     `json:"detail,omitempty"`
}

type record struct {
	Version   int64
	Filename  string
	SHA256    string
	AppliedAt time.Time
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (r *Runner) Status(ctx context.Context, db *sql.DB) ([]Status, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect for migration status: %w", err)
	}
	defer conn.Close()
	schema, err := currentSchema(ctx, conn)
	if err != nil {
		return nil, err
	}
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, fmt.Errorf("start read-only migration status: %w", err)
	}
	defer tx.Rollback()
	records, err := loadRecords(ctx, tx, schema)
	if err != nil {
		return nil, err
	}
	statuses := compareHistory(r.files, records)
	if len(records) == 0 {
		exists, err := tableExists(ctx, tx, schema, "users")
		if err != nil {
			return nil, err
		}
		if exists {
			for i := range statuses {
				statuses[i].Detail = ErrBaselineRequired.Error()
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("finish migration status: %w", err)
	}
	return statuses, nil
}

// Up executes all pending migrations. Each script and its history entry commit
// together; the returned count remains useful if a later migration fails.
func (r *Runner) Up(ctx context.Context, db *sql.DB) (int, error) {
	return r.run(ctx, db, false)
}

// Init retains the original db-init contract: only the two built-in migrations
// on a database that has no users table. Later migrations require Up.
func (r *Runner) Init(ctx context.Context, db *sql.DB) (int, error) {
	if _, err := r.initialFiles(); err != nil {
		return 0, err
	}
	return r.run(ctx, db, true)
}

func (r *Runner) run(ctx context.Context, db *sql.DB, initialOnly bool) (int, error) {
	count := 0
	err := withLock(ctx, db, func(conn *sql.Conn, schema string) error {
		records, err := loadRecords(ctx, conn, schema)
		if err != nil {
			return err
		}
		statuses := compareHistory(r.files, records)
		if err := validateHistory(statuses); err != nil {
			return err
		}
		exists, err := tableExists(ctx, conn, schema, "users")
		if err != nil {
			return err
		}
		if initialOnly && (exists || len(records) != 0) {
			return errors.New("database already initialized; use db-status and db-up for later migrations")
		}
		if exists && len(records) == 0 {
			return ErrBaselineRequired
		}
		pending := make([]file, 0, len(r.files))
		for i, status := range statuses {
			if status.State == "pending" && (!initialOnly || status.Version <= 2) {
				pending = append(pending, r.files[i])
			}
		}
		// Validate all pending transaction boundaries before any schema mutation.
		for i := range pending {
			pending[i].SQL, err = prepareSQL(pending[i].SQL)
			if err != nil {
				return sqlError(pending[i].Filename, err)
			}
		}
		if len(pending) == 0 {
			return nil
		}
		if err := ensureLedger(ctx, conn, schema); err != nil {
			return err
		}
		for _, f := range pending {
			if err := apply(ctx, conn, schema, f); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

func (r *Runner) initialFiles() ([]file, error) {
	var initial []file
	for _, f := range r.files {
		if f.Version == 1 || f.Version == 2 {
			expected := "001_init.sql"
			if f.Version == 2 {
				expected = "002_example.sql"
			}
			if f.Filename != expected {
				return nil, errors.New("initialization requires the original 001_init.sql and 002_example.sql filenames")
			}
			initial = append(initial, f)
		}
	}
	if len(initial) != 2 {
		return nil, errors.New("initialization requires 001_init.sql and 002_example.sql")
	}
	return initial, nil
}

func apply(ctx context.Context, conn *sql.Conn, schema string, f file) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return sqlError(f.Filename, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SET LOCAL standard_conforming_strings = on"); err != nil {
		return sqlError(f.Filename, err)
	}
	if _, err := tx.ExecContext(ctx, f.SQL); err != nil {
		return sqlError(f.Filename, err)
	}
	if err := insertRecord(ctx, tx, schema, f); err != nil {
		return sqlError(f.Filename, err)
	}
	if err := tx.Commit(); err != nil {
		return sqlError(f.Filename, err)
	}
	return nil
}

func currentSchema(ctx context.Context, q queryer) (string, error) {
	var schema sql.NullString
	if err := q.QueryRowContext(ctx, "SELECT current_schema()").Scan(&schema); err != nil {
		return "", fmt.Errorf("read migration schema: %w", err)
	}
	if !schema.Valid || schema.String == "" {
		return "", errors.New("migration requires an existing schema in search_path")
	}
	return schema.String, nil
}

func withLock(ctx context.Context, db *sql.DB, fn func(*sql.Conn, string) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("connect for migration: %w", err)
	}
	defer conn.Close()
	schema, err := currentSchema(ctx, conn)
	if err != nil {
		return err
	}
	// Two int32 keys isolate the lock by database and target schema. Unlike a
	// transaction lock, this remains held across each separately committed file.
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(hashtext(current_database()), hashtext($1))", "go-easy-starter:migrations:"+schema); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		err := conn.QueryRowContext(unlockCtx, "SELECT pg_advisory_unlock(hashtext(current_database()), hashtext($1))", "go-easy-starter:migrations:"+schema).Scan(&unlocked)
		if err != nil || !unlocked {
			// A session lock must never return to the pool if cleanup failed.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	return fn(conn, schema)
}

func qualified(schema, name string) string {
	return pq.QuoteIdentifier(schema) + "." + pq.QuoteIdentifier(name)
}

func tableExists(ctx context.Context, q queryer, schema, table string) (bool, error) {
	var exists bool
	err := q.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", qualified(schema, table)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("inspect migration table %s: %w", table, err)
	}
	return exists, nil
}

func ensureLedger(ctx context.Context, conn execer, schema string) error {
	_, err := conn.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+qualified(schema, "schema_migrations")+` (
version BIGINT PRIMARY KEY CHECK (version > 0),
filename TEXT NOT NULL UNIQUE,
sha256 VARCHAR(64) NOT NULL CHECK (length(sha256) = 64),
applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
 )`)
	if err != nil {
		return fmt.Errorf("create migration history: %w", err)
	}
	return nil
}

func insertRecord(ctx context.Context, tx *sql.Tx, schema string, f file) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO "+qualified(schema, "schema_migrations")+" (version,filename,sha256) VALUES ($1,$2,$3)", f.Version, f.Filename, f.SHA256)
	return err
}

func loadRecords(ctx context.Context, q queryer, schema string) ([]record, error) {
	exists, err := tableExists(ctx, q, schema, "schema_migrations")
	if err != nil || !exists {
		return nil, err
	}
	rows, err := q.QueryContext(ctx, "SELECT version,filename,sha256,applied_at FROM "+qualified(schema, "schema_migrations")+" ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read migration history: %w", err)
	}
	defer rows.Close()
	var result []record
	for rows.Next() {
		var v record
		if err := rows.Scan(&v.Version, &v.Filename, &v.SHA256, &v.AppliedAt); err != nil {
			return nil, fmt.Errorf("decode migration history: %w", err)
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func compareHistory(files []file, records []record) []Status {
	byVersion := make(map[int64]record, len(records))
	for _, record := range records {
		byVersion[record.Version] = record
	}
	statuses := make([]Status, 0, len(files)+len(records))
	for _, f := range files {
		s := Status{Version: f.Version, Filename: f.Filename, SHA256: f.SHA256, State: "pending"}
		if record, ok := byVersion[f.Version]; ok {
			s.State, s.AppliedAt = "applied", &record.AppliedAt
			if record.Filename != f.Filename {
				s.State, s.Detail = "renamed", "recorded filename: "+record.Filename
			} else if record.SHA256 != f.SHA256 {
				s.State, s.Detail = "modified", "recorded SHA256 differs from local SQL"
			}
			delete(byVersion, f.Version)
		}
		statuses = append(statuses, s)
	}
	for _, record := range byVersion {
		statuses = append(statuses, Status{Version: record.Version, Filename: record.Filename, SHA256: record.SHA256, State: "missing", AppliedAt: &record.AppliedAt, Detail: "applied SQL file is missing"})
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Version < statuses[j].Version })
	return statuses
}

func validateHistory(statuses []Status) error {
	var lastApplied int64
	for _, s := range statuses {
		if s.State != "applied" && s.State != "pending" {
			return fmt.Errorf("migration history differs at %s (%s); restore the original applied SQL", s.Filename, s.State)
		}
		if s.State == "applied" && s.Version > lastApplied {
			lastApplied = s.Version
		}
	}
	for _, s := range statuses {
		if s.State == "pending" && s.Version < lastApplied {
			return fmt.Errorf("pending migration %s precedes an applied version; create a new higher-numbered migration", s.Filename)
		}
	}
	return nil
}
