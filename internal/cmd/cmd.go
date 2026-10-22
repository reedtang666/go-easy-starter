package cmd

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/reedtang666/go-easy-starter/internal/bootstrap"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func Run(args []string) error {
	role := "all"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		role = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("starter", flag.ContinueOnError)
	path := fs.String("config", "manifest/config/config.yaml", "configuration path")
	if e := fs.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			return nil
		}
		return e
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	if role == "setup" {
		return config.Setup(*path)
	}
	c, e := config.Load(*path)
	if e != nil {
		return e
	}
	if role == "db-init" {
		return initDB(c)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	app, e := bootstrap.New(ctx, c, role)
	if e != nil {
		return e
	}
	defer app.Close()
	if e = app.Start(); e != nil {
		return e
	}
	fmt.Printf("starter role=%s started\n", role)
	<-ctx.Done()
	return nil
}
func initDB(c config.Config) error {
	db, e := sql.Open("postgres", c.Database.URL)
	if e != nil {
		return errors.New("invalid database configuration")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return errors.New("database unavailable")
	}
	defer tx.Rollback()
	var exists sql.NullString
	if e = tx.QueryRowContext(ctx, "SELECT to_regclass('public.users')::text").Scan(&exists); e != nil {
		return e
	}
	if exists.Valid {
		return errors.New("database already initialized; apply later numbered migrations explicitly")
	}
	for _, file := range []string{"manifest/sql/001_init.sql", "manifest/sql/002_example.sql"} {
		b, e := os.ReadFile(file)
		if e != nil {
			return e
		}
		s := strings.TrimSpace(string(b))
		s = strings.TrimPrefix(s, "BEGIN;")
		s = strings.TrimSuffix(s, "COMMIT;")
		if _, e = tx.ExecContext(ctx, s); e != nil {
			return e
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	fmt.Println("database initialized")
	return nil
}
