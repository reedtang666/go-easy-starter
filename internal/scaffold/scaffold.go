// Package scaffold creates a user-owned CRUD module without touching existing modules.
package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"go/format"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/gofrs/flock"
	"github.com/reedtang666/go-easy-starter/internal/migration"
)

//go:embed templates/*
var templates embed.FS

type Result struct {
	Files     []string
	Migration string
}

type names struct{ Module, Name, Camel, Lower, Action string }
type output struct {
	path string
	data []byte
}
type registration struct {
	output
	original []byte
	mode     fs.FileMode
	exists   bool
}

var validName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)
var validModule = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._~+/-]*$`)

// Generate creates API, controller, logic, service, view and an ordered SQL migration.
// It only changes the dedicated module registration files among existing sources.
func Generate(root, name string) (_ *Result, err error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return nil, errors.New("project root must be an existing directory")
	}
	lock := flock.New(filepath.Join(root, ".scaffold.lock"))
	if err := lock.Lock(); err != nil {
		return nil, fmt.Errorf("lock scaffold: %w", err)
	}
	defer lock.Close()
	module, err := readModule(root)
	if err != nil {
		return nil, err
	}
	n := names{Module: module, Name: name, Camel: camel(name)}
	n.Lower = strings.ToLower(n.Camel[:1]) + n.Camel[1:]
	files, sql, err := render(n)
	if err != nil {
		return nil, err
	}
	// An existing module directory is a conflict even if it is currently empty.
	for _, path := range []string{"api/" + name, "internal/controller/" + name, "internal/logic/" + name} {
		if err := requireAbsent(root, path); err != nil {
			return nil, err
		}
	}
	for _, file := range files {
		if err := requireAbsent(root, file.path); err != nil {
			return nil, err
		}
	}
	if err := checkPackageSymbols(root, files); err != nil {
		return nil, err
	}
	regs, err := prepareRegistrations(root, n)
	if err != nil {
		return nil, err
	}
	sqlDir := filepath.Join(root, "manifest/sql")
	if err := safeAncestors(root, "manifest/sql/placeholder.sql"); err != nil {
		return nil, err
	}
	if _, err := migration.New(sqlDir); err != nil {
		return nil, fmt.Errorf("check existing migrations: %w", err)
	}

	var createdFiles, createdDirs []string
	var changedRegs []registration
	// Roll back only files made by this call. Existing module files were never opened for writing.
	defer func() {
		if err == nil {
			return
		}
		var rollbackErr error
		for i := len(changedRegs) - 1; i >= 0; i-- {
			r := changedRegs[i]
			if r.exists {
				if e := replace(filepath.Join(root, r.path), r.original, r.mode); e != nil {
					rollbackErr = errors.Join(rollbackErr, e)
				}
			} else if e := os.Remove(filepath.Join(root, r.path)); e != nil && !os.IsNotExist(e) {
				rollbackErr = errors.Join(rollbackErr, e)
			}
		}
		for i := len(createdFiles) - 1; i >= 0; i-- {
			if e := os.Remove(createdFiles[i]); e != nil && !os.IsNotExist(e) {
				rollbackErr = errors.Join(rollbackErr, e)
			}
		}
		for i := len(createdDirs) - 1; i >= 0; i-- {
			_ = os.Remove(createdDirs[i])
		}
		if rollbackErr != nil {
			err = errors.Join(err, fmt.Errorf("rollback incomplete: %w", rollbackErr))
		}
	}()
	for _, file := range files {
		if err = mkdirParents(root, filepath.Dir(file.path), &createdDirs); err != nil {
			return nil, err
		}
		full := filepath.Join(root, file.path)
		if err = writeExclusive(full, file.data, 0644); err != nil {
			return nil, err
		}
		createdFiles = append(createdFiles, full)
	}
	fullMigration, err := migration.CreateWithContent(sqlDir, "create_"+name+"_items", string(sql))
	if err != nil {
		return nil, err
	}
	createdFiles = append(createdFiles, fullMigration)
	for _, reg := range regs {
		if err = mkdirParents(root, filepath.Dir(reg.path), &createdDirs); err != nil {
			return nil, err
		}
		full := filepath.Join(root, reg.path)
		if reg.exists {
			// Do not replace code edited by another tool while rendering this module.
			current, e := os.ReadFile(full)
			if e != nil {
				return nil, e
			}
			if !bytes.Equal(current, reg.original) {
				return nil, fmt.Errorf("registration changed during generation: %s", reg.path)
			}
			err = replace(full, reg.data, reg.mode)
		} else {
			err = writeExclusive(full, reg.data, 0644)
		}
		if err != nil {
			return nil, err
		}
		changedRegs = append(changedRegs, reg)
	}
	result := &Result{}
	for _, file := range files {
		result.Files = append(result.Files, filepath.ToSlash(file.path))
	}
	for _, reg := range regs {
		result.Files = append(result.Files, filepath.ToSlash(reg.path))
	}
	result.Migration, err = filepath.Rel(root, fullMigration)
	if err != nil {
		return nil, err
	}
	result.Migration = filepath.ToSlash(result.Migration)
	result.Files = append(result.Files, result.Migration)
	sort.Strings(result.Files)
	return result, nil
}

func validateName(name string) error {
	if len(name) > 40 || !validName.MatchString(name) || token.Lookup(name).IsKeyword() {
		return errors.New("module name must be snake_case, 1-40 characters, and not a Go keyword")
	}
	for _, reserved := range strings.Fields("api internal main auth user file example health websocket queue migration scaffold query cache model dao service bootstrap router config library cmd contexts response runtime") {
		if name == reserved {
			return fmt.Errorf("module name %q is reserved by the starter", name)
		}
	}
	return nil
}

func camel(name string) string {
	var out strings.Builder
	for _, word := range strings.Split(name, "_") {
		out.WriteString(strings.ToUpper(word[:1]))
		out.WriteString(word[1:])
	}
	return out.String()
}

func readModule(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read project go.mod: %w", err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(strings.SplitN(line, "//", 2)[0])
		if len(fields) == 0 || fields[0] != "module" {
			continue
		}
		if len(fields) != 2 {
			return "", errors.New("invalid module directive in go.mod")
		}
		module := fields[1]
		if strings.HasPrefix(module, `"`) {
			module, err = strconv.Unquote(module)
		}
		if err != nil || !validModule.MatchString(module) || strings.HasSuffix(module, "/") {
			return "", errors.New("invalid module path in go.mod")
		}
		return module, nil
	}
	return "", errors.New("go.mod does not declare a module path")
}

func render(n names) ([]output, []byte, error) {
	definitions := []struct{ source, path, action string }{
		{"api.gotmpl", "api/" + n.Name + "/v1/" + n.Name + ".go", ""},
		{"interface.gotmpl", "api/" + n.Name + "/" + n.Name + ".go", ""},
		{"controller_new.gotmpl", "internal/controller/" + n.Name + "/" + n.Name + "_new.go", ""},
		{"controller.gotmpl", "internal/controller/" + n.Name + "/" + n.Name + "_v1_list.go", "List"},
		{"controller.gotmpl", "internal/controller/" + n.Name + "/" + n.Name + "_v1_create.go", "Create"},
		{"controller.gotmpl", "internal/controller/" + n.Name + "/" + n.Name + "_v1_get.go", "Get"},
		{"controller.gotmpl", "internal/controller/" + n.Name + "/" + n.Name + "_v1_update.go", "Update"},
		{"controller.gotmpl", "internal/controller/" + n.Name + "/" + n.Name + "_v1_delete.go", "Delete"},
		{"logic.gotmpl", "internal/logic/" + n.Name + "/" + n.Name + ".go", ""},
		{"service.gotmpl", "internal/service/" + n.Name + ".go", ""},
		{"view.gotmpl", "internal/model/view/" + n.Name + ".go", ""},
	}
	files := make([]output, 0, len(definitions))
	for _, def := range definitions {
		params := n
		params.Action = def.action
		data, err := executeTemplate(def.source, params)
		if err != nil {
			return nil, nil, err
		}
		data, err = format.Source(data)
		if err != nil {
			return nil, nil, fmt.Errorf("format %s: %w", def.path, err)
		}
		files = append(files, output{path: def.path, data: data})
	}
	sql, err := executeTemplate("migration.sql.tmpl", n)
	return files, sql, err
}

func executeTemplate(name string, n names) ([]byte, error) {
	b, err := templates.ReadFile("templates/" + name)
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Option("missingkey=error").Parse(string(b))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, n); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func requireAbsent(root, path string) error {
	if err := safeAncestors(root, path); err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(root, path)); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("refusing to overwrite existing module path: %s", path)
	}
	return nil
}

// Existing directory symlinks are rejected so a generated path cannot escape the checkout.
func safeAncestors(root, path string) error {
	parent := filepath.Dir(filepath.FromSlash(path))
	for parent != "." {
		info, err := os.Lstat(filepath.Join(root, parent))
		if err == nil && !info.IsDir() {
			return fmt.Errorf("parent is not a directory: %s", parent)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		parent = filepath.Dir(parent)
	}
	return nil
}

func mkdirParents(root, path string, made *[]string) error {
	current := root
	for _, part := range strings.Split(filepath.FromSlash(path), string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		if err := os.Mkdir(current, 0755); err == nil {
			*made = append(*made, current)
		} else if !os.IsExist(err) {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("parent is not a directory: %s", current)
		}
	}
	return nil
}

func writeExclusive(path string, data []byte, mode fs.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func replace(path string, data []byte, mode fs.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".scaffold-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode.Perm()); err != nil {
		_ = f.Close()
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
