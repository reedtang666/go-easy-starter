package scaffold

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

func prepareRegistrations(root string, n names) ([]registration, error) {
	definitions := []struct {
		path, pkg, initial, paramName, paramPackage, paramType, statement string
		imports                                                           map[string]string
	}{
		{
			path: "internal/bootstrap/modules.go", pkg: "bootstrap",
			initial:   "package bootstrap\nimport \"" + n.Module + "/internal/dao\"\n\n// registerModules is maintained by the module scaffold.\nfunc registerModules(store *dao.Store) {}\n",
			paramName: "store", paramPackage: "dao", paramType: "Store",
			statement: "service.Register" + n.Camel + "(" + n.Lower + "Logic.New(store))",
			imports:   map[string]string{"service": n.Module + "/internal/service", n.Lower + "Logic": n.Module + "/internal/logic/" + n.Name},
		},
		{
			path: "internal/router/modules.go", pkg: "router",
			initial:   "package router\nimport \"github.com/gogf/gf/v2/net/ghttp\"\n\n// registerModules binds generated modules within the authenticated API group.\nfunc registerModules(group *ghttp.RouterGroup) {}\n",
			paramName: "group", paramPackage: "ghttp", paramType: "RouterGroup",
			statement: "group.Bind(" + n.Lower + "Ctrl.NewV1())",
			imports:   map[string]string{n.Lower + "Ctrl": n.Module + "/internal/controller/" + n.Name},
		},
	}
	regs := make([]registration, 0, len(definitions))
	for _, def := range definitions {
		if err := safeAncestors(root, def.path); err != nil {
			return nil, err
		}
		reg := registration{output: output{path: def.path}, mode: 0644}
		data := []byte(def.initial)
		info, err := os.Lstat(filepath.Join(root, def.path))
		if err == nil {
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("registration is not a regular file: %s", def.path)
			}
			data, err = os.ReadFile(filepath.Join(root, def.path))
			if err != nil {
				return nil, err
			}
			reg.original, reg.exists, reg.mode = data, true, info.Mode()
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, def.path, data, parser.ParseComments|parser.AllErrors)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", def.path, err)
		}
		if file.Name.Name != def.pkg {
			return nil, fmt.Errorf("wrong package in %s", def.path)
		}
		var fn *ast.FuncDecl
		for _, decl := range file.Decls {
			if candidate, ok := decl.(*ast.FuncDecl); ok && candidate.Recv == nil && candidate.Name.Name == "registerModules" {
				if fn != nil {
					return nil, fmt.Errorf("duplicate registerModules in %s", def.path)
				}
				fn = candidate
			}
		}
		if fn == nil || fn.Body == nil || !matchesParameter(fn, def.paramName, def.paramPackage, def.paramType) {
			return nil, fmt.Errorf("expected func registerModules(%s *%s.%s) in %s", def.paramName, def.paramPackage, def.paramType, def.path)
		}
		// Verify the parameter's imported package as well as its spelling.
		parameterPath := "github.com/gogf/gf/v2/net/ghttp"
		if def.pkg == "bootstrap" {
			parameterPath = n.Module + "/internal/dao"
		}
		if err := checkImport(file, def.paramPackage, parameterPath); err != nil {
			return nil, fmt.Errorf("%s: %w", def.path, err)
		}
		aliases := make([]string, 0, len(def.imports))
		for alias := range def.imports {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)
		var importLines bytes.Buffer
		for _, alias := range aliases {
			needed, err := needsImport(file, fn, alias, def.imports[alias])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", def.path, err)
			}
			if needed {
				fmt.Fprintf(&importLines, "import %s %q\n", alias, def.imports[alias])
			}
		}
		expr, err := parser.ParseExpr(def.statement)
		if err != nil {
			return nil, err
		}
		var statement bytes.Buffer
		if err := format.Node(&statement, token.NewFileSet(), expr); err != nil {
			return nil, err
		}
		// Source offsets come from the parsed AST. Keeping the original source around
		// those offsets preserves unrelated declarations and their comments exactly.
		edits := []sourceEdit{{offset: fset.Position(fn.Body.Rbrace).Offset, text: "\n\t" + statement.String() + "\n"}}
		if importLines.Len() != 0 {
			last := file.Imports[len(file.Imports)-1]
			end := fset.Position(last.End()).Offset
			// For grouped imports, insert after the whole import declaration.
			for _, decl := range file.Decls {
				if g, ok := decl.(*ast.GenDecl); ok && g.Tok == token.IMPORT && g.Pos() <= last.Pos() && g.End() >= last.End() {
					end = fset.Position(g.End()).Offset
				}
			}
			if next := bytes.IndexByte(data[end:], '\n'); next >= 0 {
				end += next + 1
			}
			edits = append(edits, sourceEdit{offset: end, text: "\n" + importLines.String()})
		}
		sort.Slice(edits, func(i, j int) bool { return edits[i].offset > edits[j].offset })
		modified := append([]byte(nil), data...)
		for _, edit := range edits {
			modified = append(modified[:edit.offset], append([]byte(edit.text), modified[edit.offset:]...)...)
		}
		reg.data, err = format.Source(modified)
		if err != nil {
			return nil, fmt.Errorf("format %s: %w", def.path, err)
		}
		regs = append(regs, reg)
	}
	return regs, nil
}

type sourceEdit struct {
	offset int
	text   string
}

func matchesParameter(fn *ast.FuncDecl, name, pkg, typ string) bool {
	if fn.Type.Results != nil || fn.Type.TypeParams != nil || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	field := fn.Type.Params.List[0]
	if len(field.Names) != 1 || field.Names[0].Name != name {
		return false
	}
	star, ok := field.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := star.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != typ {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == pkg
}

func importAlias(spec *ast.ImportSpec) (string, string) {
	path, _ := strconv.Unquote(spec.Path.Value)
	if spec.Name != nil {
		return spec.Name.Name, path
	}
	return filepath.Base(path), path
}

func checkImport(file *ast.File, alias, path string) error {
	for _, spec := range file.Imports {
		name, target := importAlias(spec)
		if name == alias && target == path {
			return nil
		}
	}
	return fmt.Errorf("required import %s %q is missing or aliased differently", alias, path)
}

func needsImport(file *ast.File, fn *ast.FuncDecl, alias, path string) (bool, error) {
	if file.Scope.Lookup(alias) != nil {
		return false, fmt.Errorf("import alias %s conflicts with an existing declaration", alias)
	}
	var shadowed bool
	ast.Inspect(fn, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok && id.Name == alias && id.Obj != nil {
			shadowed = true
		}
		return true
	})
	if shadowed {
		return false, fmt.Errorf("import alias %s is shadowed inside registerModules", alias)
	}
	for _, spec := range file.Imports {
		name, target := importAlias(spec)
		if name == alias {
			if target != path {
				return false, fmt.Errorf("import alias %s is already used by %q", alias, target)
			}
			return false, nil
		}
		if target == path {
			return false, fmt.Errorf("import %q already uses alias %s", path, name)
		}
	}
	return true, nil
}
