package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationCreateWithoutRuntimeConfig(t *testing.T) {
	dir := t.TempDir()
	if err := Run([]string{"db-new", "--dir", dir, "--name", "add_article_status"}); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*_add_article_status.sql"))
	if err != nil || len(files) != 1 {
		t.Fatalf("migration was not created: %v %v", files, err)
	}
	if info, err := os.Stat(files[0]); err != nil || info.Size() == 0 {
		t.Fatalf("missing SQL template: %v", err)
	}
}

func TestToolCommandsHelp(t *testing.T) {
	for _, name := range []string{"module", "db-new", "db-status", "db-up", "db-baseline", "db-init"} {
		t.Run(name, func(t *testing.T) {
			if err := Run([]string{name, "--help"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestToolCommandValidation(t *testing.T) {
	for _, args := range [][]string{
		{"module"},
		{"db-new"},
		{"db-new", "--name", "../outside", "--dir", t.TempDir()},
		{"db-up", "--timeout", "0s"},
		{"db-status", "unexpected"},
	} {
		if err := Run(args); err == nil {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
}

func TestModuleValidationWithoutRuntimeConfig(t *testing.T) {
	err := Run([]string{"module", "--root", t.TempDir(), "--name", "../outside"})
	if err == nil {
		t.Fatal("invalid module accepted")
	}
	if strings.Contains(err.Error(), "configuration") {
		t.Fatalf("file-only command loaded runtime configuration: %v", err)
	}
}

func TestDatabaseCommandDoesNotExposeMalformedURL(t *testing.T) {
	for _, command := range []string{"db-init", "db-up", "db-status", "db-baseline"} {
		for name, databaseURL := range map[string]string{
			"host escape":     "postgres://user:must-not-leak@bad%host/starter",
			"invalid port":    "postgres://user:must-not-leak@localhost:invalid/starter",
			"password escape": "postgres://user:must-not-leak%zz@localhost/starter",
		} {
			t.Run(command+"/"+name, func(t *testing.T) {
				t.Setenv("STARTER_DATABASE_URL", databaseURL)
				dir := t.TempDir()
				p := filepath.Join(dir, "config.yaml")
				b, err := json.Marshal(map[string]any{"database": map[string]string{"url": databaseURL}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, b, 0600); err != nil {
					t.Fatal(err)
				}
				err = Run([]string{command, "--config", p, "--dir", dir})
				if err == nil {
					t.Fatal("malformed database URL accepted")
				}
				if strings.Contains(err.Error(), "must-not-leak") {
					t.Fatal("database URL password exposed in command error")
				}
				if err.Error() != "invalid database configuration" {
					t.Fatal("expected a configuration error before attempting a connection")
				}
			})
		}
	}
}
