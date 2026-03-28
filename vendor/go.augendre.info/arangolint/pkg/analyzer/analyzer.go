// Package analyzer contains tools for analyzing arangodb usage.
// It focuses on github.com/arangodb/go-driver/v2.
package analyzer

import (
	"errors"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// NewAnalyzer returns an arangolint analyzer.
func NewAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     "arangolint",
		Doc:      "opinionated best practices for arangodb client",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

var (
	errUnknown         = errors.New("unknown node type")
	errInvalidAnalysis = errors.New("invalid analysis")
)

const missingAllowImplicitOptionMsg = "missing AllowImplicit option"

func run(pass *analysis.Pass) (interface{}, error) {
	inspctr, typeValid := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !typeValid {
		return nil, errInvalidAnalysis
	}

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	inspctr.Preorder(nodeFilter, func(node ast.Node) {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return
		}

		if !isBeginTransaction(call, pass) {
			return
		}

		diag := analysis.Diagnostic{
			Pos:     call.Pos(),
			Message: missingAllowImplicitOptionMsg,
		}

		switch typedArg := call.Args[2].(type) {
		case *ast.Ident:
			if typedArg.Name == "nil" {
				pass.Report(diag)

				return
			}
		case *ast.UnaryExpr:
			elts, err := getElts(typedArg.X)
			if err != nil {
				return
			}

			if !eltsHasAllowImplicit(elts) {
				pass.Report(analysis.Diagnostic{
					Pos:     call.Pos(),
					Message: missingAllowImplicitOptionMsg,
				})
			}

			return
		}
	})

	return nil, nil //nolint:nilnil
}

func isBeginTransaction(call *ast.CallExpr, pass *analysis.Pass) bool {
	selExpr, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}

	xType := pass.TypesInfo.TypeOf(selExpr.X)
	if xType == nil {
		return false
	}

	const arangoStruct = "github.com/arangodb/go-driver/v2/arangodb.Database"

	if !strings.HasSuffix(xType.String(), arangoStruct) ||
		selExpr.Sel.Name != "BeginTransaction" {
		return false
	}

	const expectedArgsCount = 3

	return len(call.Args) == expectedArgsCount
}

func getElts(node ast.Node) ([]ast.Expr, error) {
	switch typedNode := node.(type) {
	case *ast.CompositeLit:
		return typedNode.Elts, nil
	default:
		return nil, errUnknown
	}
}

func eltsHasAllowImplicit(elts []ast.Expr) bool {
	for _, elt := range elts {
		if eltIsAllowImplicit(elt) {
			return true
		}
	}

	return false
}

func eltIsAllowImplicit(expr ast.Expr) bool {
	switch typedNode := expr.(type) {
	case *ast.KeyValueExpr:
		ident, ok := typedNode.Key.(*ast.Ident)
		if !ok {
			return false
		}

		return ident.Name == "AllowImplicit"
	default:
		return false
	}
}
