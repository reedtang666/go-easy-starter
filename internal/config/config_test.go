package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetupPreservesSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Setup(path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Setup(path); err == nil {
		t.Fatal("must refuse to overwrite configuration")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("configuration overwritten")
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Auth.AccessSecretKey) < 32 || c.Auth.AccessSecretKey == c.Auth.RefreshHashKey {
		t.Fatal("invalid independent secrets")
	}
}
func TestRoleValidation(t *testing.T) {
	c := Defaults()
	if err := c.Validate("scheduler"); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate("http"); err == nil {
		t.Fatal("http must reject missing auth secrets")
	}
	if err := c.Validate("bogus"); err == nil {
		t.Fatal("unknown role must fail")
	}
}
func TestEnvironmentOverride(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := Setup(p); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STARTER_SERVER_ADDRESS", ":9999")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Address != ":9999" {
		t.Fatal(c.Server.Address)
	}
	t.Setenv("STARTER_AUTH_ACCESS_SECONDS", "invalid")
	if _, err = Load(p); err == nil {
		t.Fatal("invalid override must fail")
	}
}
