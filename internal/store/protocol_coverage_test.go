package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Every public RPC operation must have an explicit controller dispatch.
// This prevents the published catalog growing ahead of its implementation.
func TestPublicProtocolCatalogHasCompleteControllerDispatch(t *testing.T) {
	operations, err := parser.ParseFile(token.NewFileSet(), "../../protocol/operations.go", nil, 0)
	if err != nil { t.Fatal(err) }
	dispatch, err := parser.ParseFile(token.NewFileSet(), "protocol.go", nil, 0)
	if err != nil { t.Fatal(err) }
	declared := map[string]bool{}
	ast.Inspect(operations, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok { return true }
		for _, name := range spec.Names {
			if strings.HasPrefix(name.Name, "Operation") {
				declared[name.Name] = true
			}
		}
		return true
	})
	actual := map[string]bool{}
	ast.Inspect(dispatch, func(n ast.Node) bool {
		clause, ok := n.(*ast.CaseClause)
		if !ok { return true }
		for _, expr := range clause.List {
			selector, ok := expr.(*ast.SelectorExpr)
			if !ok { continue }
			pkg, ok := selector.X.(*ast.Ident)
			if ok && pkg.Name == "protocol" { actual[selector.Sel.Name] = true }
		}
		return true
	})
	for name := range declared {
		if name == "OperationEventPublish" { continue } // typed durable event stream
		if !actual[name] { t.Errorf("%s is declared but no controller handler exists", name) }
	}
	if len(declared) != 37 { t.Fatalf("unexpected public catalog count: %d", len(declared)) }
}
