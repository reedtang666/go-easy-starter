package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	put(t, dir, "go.mod", "module example.com/team/custom-api\n\ngo 1.26.10\n")
	put(t, dir, "manifest/sql/002_example.sql", "CREATE TABLE existing(id bigint);\n")
	return dir
}

func put(t *testing.T, root, path, data string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGenerateRejectsInvalidAndReservedNames(t *testing.T) {
	root := fixture(t)
	for _, name := range []string{"", "../article", "article/post", "Article", "a-b", "1article", "_article", "article_", "article__post", strings.Repeat("a", 41), "func", "type", "main", "auth", "user", "file", "example", "health", "websocket", "queue", "migration", "scaffold", "query", "model"} {
		if _, err := Generate(root, name); err == nil {
			t.Errorf("accepted invalid/reserved name %q", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "api")); !os.IsNotExist(err) {
		t.Fatal("invalid input created module files")
	}
}

func TestGenerateCreatesRunnableTypedCRUDAndNumberedSQL(t *testing.T) {
	root := fixture(t)
	result, err := Generate(root, "article_comment")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 14 || filepath.Base(result.Migration) != "003_create_article_comment_items.sql" {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, path := range result.Files {
		data := read(t, root, path)
		if strings.HasSuffix(path, ".go") {
			if _, err := parser.ParseFile(token.NewFileSet(), path, data, parser.AllErrors); err != nil {
				t.Fatalf("invalid Go %s: %v", path, err)
			}
			if strings.Contains(data, "github.com/reedtang666/go-easy-starter") {
				t.Fatalf("hard-coded module in %s", path)
			}
		}
	}
	logic := read(t, root, "internal/logic/article_comment/article_comment.go")
	for _, expected := range []string{"type sArticleComment struct", `Model("article_comment_items").Ctx(ctx).Safe()`, "query.Apply", "input.Page.Normalized()", "Where(\"owner_id\", owner)", "InsertID"} {
		if !strings.Contains(logic, expected) {
			t.Errorf("logic missing %s", expected)
		}
	}
	if !strings.Contains(read(t, root, "api/article_comment/v1/article_comment.go"), `path:"/article_comment/items"`) {
		t.Fatal("incorrect route")
	}
	if !strings.Contains(read(t, root, "internal/bootstrap/modules.go"), "service.RegisterArticleComment(articleCommentLogic.New(store))") {
		t.Fatal("service not registered")
	}
	if !strings.Contains(read(t, root, "internal/router/modules.go"), "group.Bind(articleCommentCtrl.NewV1())") {
		t.Fatal("route not bound:\n" + read(t, root, "internal/router/modules.go"))
	}
	if !strings.Contains(read(t, root, result.Migration), "owner_id BIGINT NOT NULL REFERENCES users(id)") {
		t.Fatal("missing ownership foreign key")
	}
}

func TestGeneratePreservesRegistrationsAndRejectsOverwrite(t *testing.T) {
	root := fixture(t)
	put(t, root, "internal/bootstrap/modules.go", `package bootstrap
import "example.com/team/custom-api/internal/dao"
// Existing registrations must stay.
func registerModules(store *dao.Store) {
 // Custom registration marker.
 _ = store
}
func keepMe() string { return "unchanged" }
`)
	if _, err := Generate(root, "article"); err != nil {
		t.Fatal(err)
	}
	before := read(t, root, "internal/bootstrap/modules.go")
	for _, text := range []string{"Existing registrations must stay.", "Custom registration marker.", "keepMe", `_ = store`, `return "unchanged"`} {
		if !strings.Contains(before, text) {
			t.Errorf("lost %s", text)
		}
	}
	if _, err := Generate(root, "article"); err == nil {
		t.Fatal("overwrote existing module")
	}
	if after := read(t, root, "internal/bootstrap/modules.go"); after != before {
		t.Fatal("duplicate generation mutated registrations")
	}
	if _, err := Generate(root, "category"); err != nil {
		t.Fatal(err)
	}
	updated := read(t, root, "internal/bootstrap/modules.go")
	if strings.Count(updated, "service.RegisterArticle(") != 1 || strings.Count(updated, "service.RegisterCategory(") != 1 {
		t.Fatal("incorrect cumulative registration")
	}
}

func TestGeneratePreflightsEveryTargetAndImportConflict(t *testing.T) {
	for _, conflict := range []string{"api/article", "internal/controller/article", "internal/logic/article", "internal/service/article.go", "internal/model/view/article.go", "internal/router/modules.go", "alias", "missing function", "wrong parameter", "symlink"} {
		t.Run(conflict, func(t *testing.T) {
			root := fixture(t)
			switch conflict {
			case "alias":
				put(t, root, "internal/router/modules.go", `package router
import ("github.com/gogf/gf/v2/net/ghttp"; articleCtrl "elsewhere/controller")
func registerModules(group *ghttp.RouterGroup) { _ = articleCtrl.Keep }
`)
			case "missing function":
				put(t, root, "internal/router/modules.go", "package router\nfunc other() {}\n")
			case "wrong parameter":
				put(t, root, "internal/router/modules.go", "package router\nfunc registerModules(other int) {}\n")
			case "symlink":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "api")); err != nil {
					t.Fatal(err)
				}
			default:
				put(t, root, conflict, "existing contents")
			}
			if _, err := Generate(root, "article"); err == nil {
				t.Fatal("conflict accepted")
			}
			if _, err := os.Stat(filepath.Join(root, "internal/bootstrap/modules.go")); !os.IsNotExist(err) {
				t.Fatal("left partial registration")
			}
			matches, err := filepath.Glob(filepath.Join(root, "manifest/sql/003_*.sql"))
			if err != nil || len(matches) != 0 {
				t.Fatal("left partial migration")
			}
		})
	}
}

func TestGenerateConcurrentModulesAreSerialized(t *testing.T) {
	root := fixture(t)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"article", "category"} {
		wg.Add(1)
		go func(name string) { defer wg.Done(); _, err := Generate(root, name); errs <- err }(name)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	text := read(t, root, "internal/router/modules.go")
	if !strings.Contains(text, "articleCtrl.NewV1()") || !strings.Contains(text, "categoryCtrl.NewV1()") {
		t.Fatal("lost concurrent registration:\n" + text)
	}
	files, _ := filepath.Glob(filepath.Join(root, "manifest/sql/00[34]_*.sql"))
	if len(files) != 2 {
		t.Fatal("migration allocation collided")
	}
}

func TestGenerateRollsBackFilesWhenMigrationCreationFails(t *testing.T) {
	root := fixture(t)
	put(t, root, "manifest/sql/9223372036854775807_last.sql", "SELECT 1;\n")
	if _, err := Generate(root, "article"); err == nil || !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("expected migration creation failure: %v", err)
	}
	for _, path := range []string{"api", "internal", "manifest/sql/003_create_article_items.sql"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Errorf("left partial output %s", path)
		}
	}
}

func TestGenerateHandlesQuotedModuleAndGroupedImports(t *testing.T) {
	root := fixture(t)
	put(t, root, "go.mod", "module \"example.com/renamed/project\" // retained\n\ngo 1.26.10\n")
	put(t, root, "internal/bootstrap/modules.go", `package bootstrap
import (
 // Keep the original import comment.
 "example.com/renamed/project/internal/dao"
 service "example.com/renamed/project/internal/service"
)
func registerModules(store *dao.Store) {
 // Existing service registry statement.
 _ = service.User
}
`)
	if _, err := Generate(root, "article"); err != nil {
		t.Fatal(err)
	}
	text := read(t, root, "internal/bootstrap/modules.go")
	if !strings.Contains(text, "Keep the original import comment.") || strings.Count(text, `"example.com/renamed/project/internal/service"`) != 1 {
		t.Fatalf("lost comment or duplicated import:\n%s", text)
	}
	if !strings.Contains(text, "service.RegisterArticle(articleLogic.New(store))") {
		t.Fatal("missing registry entry")
	}
}

func TestGenerateRejectsShadowedOrConflictingSymbols(t *testing.T) {
	for _, contents := range []string{
		`package bootstrap
import "example.com/team/custom-api/internal/dao"
import service "example.com/team/custom-api/internal/service"
func registerModules(store *dao.Store) { service := 1; _ = service }
`,
		`package bootstrap
import "example.com/team/custom-api/internal/dao"
type articleLogic struct{}
func registerModules(store *dao.Store) {}
`,
		`package bootstrap
import "example.com/team/custom-api/internal/dao"
import articleLogic "example.com/team/custom-api/internal/logic/article"
func registerModules(store *dao.Store) { articleLogic := 1; _ = articleLogic }
`,
	} {
		root := fixture(t)
		put(t, root, "internal/bootstrap/modules.go", contents)
		if _, err := Generate(root, "article"); err == nil {
			t.Fatal("symbol conflict accepted")
		}
		if _, err := os.Stat(filepath.Join(root, "api")); !os.IsNotExist(err) {
			t.Fatal("symbol conflict left module files")
		}
		if read(t, root, "internal/bootstrap/modules.go") != contents {
			t.Fatal("conflict mutated existing code")
		}
	}
}

func TestGenerateRejectsInvalidProjectRootAndModule(t *testing.T) {
	if _, err := Generate(filepath.Join(t.TempDir(), "missing"), "article"); err == nil {
		t.Fatal("missing root accepted")
	}
	for _, contents := range []string{"", "go 1.26.10\n", "module too many fields", "module \"bad\\npath\"", "module ../bad", "module example.com/bad/"} {
		root := fixture(t)
		put(t, root, "go.mod", contents)
		if _, err := Generate(root, "article"); err == nil {
			t.Errorf("invalid go.mod accepted: %q", contents)
		}
	}
}

func TestGenerateRejectsExistingServicePackageSymbolsBeforeWriting(t *testing.T) {
	for _, tc := range []struct{ name, declaration string }{
		{"current", "func Current() {}"},
		{"configure", "func Configure() {}"},
		{"register_user", "func RegisterUser() {}"},
		{"i_user", "type IUser interface{}"},
		{"article", "var localArticle int"},
		{"article", "const RegisterArticle = 1"},
		{"article", "type IArticle interface{}"},
	} {
		t.Run(tc.name+tc.declaration, func(t *testing.T) {
			root := fixture(t)
			put(t, root, "internal/service/existing.go", "package service\n"+tc.declaration+"\n")
			if _, err := Generate(root, tc.name); err == nil {
				t.Fatal("existing package symbol accepted")
			}
			for _, path := range []string{"api", "internal/bootstrap/modules.go", "internal/router/modules.go"} {
				if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
					t.Errorf("left partial output %s", path)
				}
			}
			matches, _ := filepath.Glob(filepath.Join(root, "manifest/sql/003_*.sql"))
			if len(matches) != 0 {
				t.Fatal("left partial SQL")
			}
		})
	}
}

func TestGenerateRejectsExistingViewSymbolButAllowsMethodsAndTests(t *testing.T) {
	root := fixture(t)
	put(t, root, "internal/model/view/custom.go", "package view\ntype ArticleItem struct{}\n")
	if _, err := Generate(root, "article"); err == nil {
		t.Fatal("existing view type accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "api")); !os.IsNotExist(err) {
		t.Fatal("view conflict left partial module")
	}
	if err := os.Remove(filepath.Join(root, "internal/model/view/custom.go")); err != nil {
		t.Fatal(err)
	}
	put(t, root, "internal/service/custom.go", "package service\ntype Existing struct{}\nfunc (Existing) Article() {}\n")
	put(t, root, "internal/service/custom_test.go", "package service\nfunc Article() {}\n")
	if _, err := Generate(root, "article"); err != nil {
		t.Fatalf("unrelated methods/test symbols blocked module: %v", err)
	}
}
