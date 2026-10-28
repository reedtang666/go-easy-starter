package cmd

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/lib/pq"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"github.com/reedtang666/go-easy-starter/internal/migration"
	"github.com/reedtang666/go-easy-starter/internal/scaffold"
)

func runTool(command string, args []string) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	var name, root, dir, path string
	var timeout time.Duration
	switch command {
	case "module":
		fs.StringVar(&name, "name", "", "business module name in lower snake_case")
		fs.StringVar(&root, "root", ".", "project root containing go.mod")
	case "db-new":
		fs.StringVar(&name, "name", "", "migration name in lower snake_case")
		fs.StringVar(&dir, "dir", "manifest/sql", "numbered SQL directory")
	default:
		fs.StringVar(&path, "config", "manifest/config/config.yaml", "configuration path")
		fs.StringVar(&dir, "dir", "manifest/sql", "numbered SQL directory")
		fs.DurationVar(&timeout, "timeout", 30*time.Second, "total database command timeout")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments; use COMMAND --help")
	}
	switch command {
	case "module":
		if name == "" {
			return errors.New("module requires --name, for example: module --name article")
		}
		result, err := scaffold.Generate(root, name)
		if err != nil {
			return err
		}
		for _, file := range result.Files {
			fmt.Println("generated:", file)
		}
		fmt.Println("module registered in the authenticated API group")
		fmt.Println("next: run db-status and db-up before calling /api/v1/" + name + "/items")
		return nil
	case "db-new":
		if name == "" {
			return errors.New("db-new requires --name, for example: db-new --name add_article_status")
		}
		file, err := migration.Create(dir, name)
		if err != nil {
			return err
		}
		fmt.Println("created:", file)
		fmt.Println("write SQL in this file, then run db-status and db-up; do not edit applied files")
		return nil
	}
	if timeout <= 0 {
		return errors.New("database command timeout must be positive")
	}
	runner, err := migration.New(dir)
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if cfg.Database.URL == "" {
		return errors.New("database.url is required")
	}
	// Parse before connecting: sql.Open defers lib/pq URL parsing until Ping,
	// whose raw parse error can include the complete URL and its credentials.
	connector, err := pq.NewConnector(cfg.Database.URL)
	if err != nil {
		return errors.New("invalid database configuration")
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database unavailable: %w", err)
	}
	if command == "db-status" {
		statuses, err := runner.Status(ctx, db)
		if err != nil {
			return err
		}
		return printMigrationStatus(statuses)
	}
	var count int
	switch command {
	case "db-init":
		count, err = runner.Init(ctx, db)
	case "db-up":
		count, err = runner.Up(ctx, db)
	case "db-baseline":
		count, err = runner.Baseline(ctx, db)
	}
	if err != nil {
		if count > 0 {
			fmt.Printf("committed %d migration(s) before failure\n", count)
		}
		return err
	}
	switch command {
	case "db-init":
		fmt.Println("database initialized")
	case "db-up":
		fmt.Printf("database up to date; applied %d migration(s)\n", count)
	case "db-baseline":
		fmt.Printf("database baseline recorded: %d migration(s)\n", count)
	}
	return nil
}

func printMigrationStatus(statuses []migration.Status) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "VERSION\tSTATE\tFILE\tAPPLIED AT")
	var invalid bool
	for _, status := range statuses {
		applied := "-"
		if status.AppliedAt != nil {
			applied = status.AppliedAt.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%03d\t%s\t%s\t%s\n", status.Version, status.State, status.Filename, applied)
		if status.Detail != "" {
			fmt.Fprintln(w, "\t\t"+status.Detail)
		}
		if status.State != "pending" && status.State != "applied" {
			invalid = true
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if invalid {
		return errors.New("migration history differs from local files; restore applied SQL before upgrading")
	}
	return nil
}
