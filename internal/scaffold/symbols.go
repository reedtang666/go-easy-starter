package scaffold

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Service and view files extend shared packages. An unused module directory does
// not prove that its exported identifiers are free (for example Current or IUser).
func checkPackageSymbols(root string, files []output) error {
	for _, generated := range files {
		dir := filepath.ToSlash(filepath.Dir(generated.path))
		if dir != "internal/service" && dir != "internal/model/view" {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), generated.path, generated.data, parser.AllErrors)
		if err != nil {
			return err
		}
		wanted := packageSymbols(file)
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				continue
			}
			path := filepath.Join(root, dir, name)
			existing, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
			if err != nil {
				return fmt.Errorf("parse existing package file %s: %w", filepath.Join(dir, name), err)
			}
			for symbol := range packageSymbols(existing) {
				if wanted[symbol] {
					return fmt.Errorf("generated symbol %s already exists in %s", symbol, filepath.ToSlash(filepath.Join(dir, name)))
				}
			}
		}
	}
	return nil
}

func packageSymbols(file *ast.File) map[string]bool {
	symbols := make(map[string]bool)
	add := func(name string) {
		if name != "_" {
			symbols[name] = true
		}
	}
	for _, declaration := range file.Decls {
		switch d := declaration.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name != "init" {
				add(d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					add(s.Name.Name)
				case *ast.ValueSpec:
					for _, name := range s.Names {
						add(name.Name)
					}
				}
			}
		}
	}
	return symbols
}
