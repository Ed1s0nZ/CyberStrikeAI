package security

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
)

func TestEveryProtectedRouteHasCatalogPermission(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "app", "app.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]string{
		"GET": http.MethodGet, "POST": http.MethodPost, "PUT": http.MethodPut,
		"PATCH": http.MethodPatch, "DELETE": http.MethodDelete,
	}
	prefixes := map[string]string{"protected": "", "c2Routes": "/c2", "knowledgeRoutes": "/knowledge"}
	found := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		prefix, protected := prefixes[ident.Name]
		method, routeMethod := methods[sel.Sel.Name]
		literal, literalPath := call.Args[0].(*ast.BasicLit)
		if !protected || !routeMethod || !literalPath || literal.Kind != token.STRING {
			return true
		}
		path, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Errorf("invalid route literal %s", literal.Value)
			return true
		}
		found++
		permission := permissionForRequest(method, "/api"+prefix+path)
		if permission == "" {
			t.Errorf("unmapped protected route: %s %s%s", method, prefix, path)
		} else if _, ok := PermissionCatalog[permission]; !ok {
			t.Errorf("route %s %s%s maps to unknown permission %q", method, prefix, path, permission)
		}
		return true
	})
	if found < 100 {
		t.Fatalf("route inventory unexpectedly small: %d", found)
	}
}
