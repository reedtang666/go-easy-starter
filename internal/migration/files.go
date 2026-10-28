package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gofrs/flock"
)

var migrationName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)
var migrationFile = regexp.MustCompile(`^([0-9]{3,})_([a-z][a-z0-9]*(?:_[a-z0-9]+)*)\.sql$`)

type file struct {
	Version  int64
	Filename string
	SHA256   string
	SQL      string
}

// Runner holds a snapshot of the numbered SQL files in a migration directory.
type Runner struct {
	files []file
}

func New(dir string) (*Runner, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	r := &Runner{}
	versions := map[int64]bool{}
	for _, entry := range entries {
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			continue
		}
		match := migrationFile.FindStringSubmatch(entry.Name())
		if len(match) == 0 {
			return nil, fmt.Errorf("invalid migration filename %q; expected NNN_snake_case.sql", entry.Name())
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid migration version: %s", entry.Name())
		}
		if versions[version] {
			return nil, fmt.Errorf("duplicate migration version %d", version)
		}
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("migration %s must be a regular file", entry.Name())
		}
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		versions[version] = true
		r.files = append(r.files, file{Version: version, Filename: entry.Name(), SHA256: checksum(b), SQL: string(b)})
	}
	sort.Slice(r.files, func(i, j int) bool { return r.files[i].Version < r.files[j].Version })
	return r, nil
}

func checksum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Create allocates a version for an editable SQL migration. It never overwrites
// an existing file. A comment-only draft must be filled before db-up can run it.
func Create(dir, name string) (string, error) {
	return CreateWithContent(dir, name, "-- "+name+"\n-- Write SQL below. Each file runs in its own transaction; do not add BEGIN/COMMIT.\n")
}

// CreateWithContent allocates and writes one complete migration while holding
// the same process-independent lock used by Create and the module generator.
func CreateWithContent(dir, name, body string) (string, error) {
	if !migrationName.MatchString(name) {
		return "", errors.New("migration name must use lower snake_case, starting with a letter")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create migration directory: %w", err)
	}
	lock := flock.New(filepath.Join(dir, ".migration.lock"))
	if err := lock.Lock(); err != nil {
		return "", fmt.Errorf("lock migration directory: %w", err)
	}
	defer lock.Close()
	r, err := New(dir)
	if err != nil {
		return "", err
	}
	var next int64 = 1
	if len(r.files) > 0 {
		last := r.files[len(r.files)-1].Version
		if last == math.MaxInt64 {
			return "", errors.New("migration version exhausted")
		}
		next = last + 1
	}
	path := filepath.Join(dir, fmt.Sprintf("%03d_%s.sql", next, name))
	// Write a hidden temporary file first. Linking the completed inode publishes
	// it atomically and refuses an existing destination, even outside our lock.
	f, err := os.CreateTemp(dir, ".migration-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create migration file: %w", err)
	}
	defer os.Remove(f.Name())
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return "", fmt.Errorf("write migration file: %w", err)
	}
	if err := f.Chmod(0644); err != nil {
		f.Close()
		return "", fmt.Errorf("set migration permissions: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return "", fmt.Errorf("sync migration file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close migration file: %w", err)
	}
	if err := os.Link(f.Name(), path); err != nil {
		return "", fmt.Errorf("publish migration without overwriting: %w", err)
	}
	return path, nil
}
