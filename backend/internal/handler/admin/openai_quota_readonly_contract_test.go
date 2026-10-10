package admin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// Quota read endpoints must not signal the independent automatic reset worker.
// This boundary is structural: the worker runs asynchronously, outside the
// handler's HTTP response and the fake quota client's observable calls.
func TestOpenAIQuotaReadHandlersDoNotTriggerCreditConsumption(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "openai_oauth_handler.go", nil, 0)
	require.NoError(t, err)
	checked := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "QueryQuota" && fn.Name.Name != "RefreshQuota") {
			continue
		}
		checked++
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch selector.Sel.Name {
			case "NotifyOpenAIAutoResetCredit", "ResetCredit", "ResetCreditTargeted":
				t.Errorf("%s must not call %s", fn.Name.Name, selector.Sel.Name)
			}
			return true
		})
	}
	require.Equal(t, 2, checked)
}
