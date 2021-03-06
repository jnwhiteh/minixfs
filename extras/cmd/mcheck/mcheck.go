package main

import (
	"flag"
	"fmt"
	"go/ast"
	"rog-go.googlecode.com/hg/exp/go/parser"
	"rog-go.googlecode.com/hg/exp/go/types"
)

// Really, everything gets narrowed down to an ast.Ident
func checkTypes(e ast.Expr, p ast.Expr) {
	// Fetch the type of this expression
	_, objType := types.ExprType(e, types.DefaultImporter)
	pos := types.FileSet.Position(e.Pos())
	if objType.Kind != ast.Bad {
		switch n := objType.Node.(type) {
		case *ast.Ident:
			// If we have a parent expression, use that position
			if p != nil {
				pos = types.FileSet.Position(p.Pos())
			}
			// Actually do the check against typeMatch here.
			if n.Name == *typeMatch {
				fmt.Printf("%s: Match for '%s'\n", pos, *typeMatch)
			}
		case *ast.StarExpr:
			// Unwrap any star expressions
			checkTypes(n.X, e)
		}
	}
	/*
		if e != nil {
			pos := types.FileSet.Position(e.Pos())

			if obj != nil {
				debugf("%s: %s %q <%T>", pos, objtype.Kind, objtype.Node, objtype.Node)
				arr, ok := objtype.Node.(*ast.ArrayType)
				if ok {
					debugf("IT IS AN ARRAY TYPE")
					// Check to see if the type of Elt is a star expression
					star, ok := arr.Elt.(*ast.StarExpr)
					if ok {
						checkTypes(star.X)
					}
				}
			} else {
				debugf("%s: Could not determine type %T", pos, e)
			}
		}
	*/
}

func checkExprs(pkg *ast.File) {
	var visit astVisitor
	visit = func(n ast.Node) bool {
		if n != nil {
			debugf("Visiting node at %s of type %T", types.FileSet.Position(n.Pos()), n)
		}

		switch n := n.(type) {
		case *ast.KeyValueExpr:
		case ast.Expr:
			checkTypes(n, nil)
		}

		switch n := n.(type) {
		case *ast.ImportSpec:
			if n.Name != nil && n.Name.Name == "." {
				// we don't support this
				return false
			}
			return true
		case *ast.FuncDecl:
			// add object for init functions
			if n.Recv == nil && n.Name.Name == "init" {
				n.Name.Obj = ast.NewObj(ast.Fun, "init")
			}
			return true
		case *ast.Ident:
			if n.Name == "_" {
				return false
			}
		case *ast.KeyValueExpr:
			// don't try to resolve the key part of a key-value
			// because it might be a map key which doesn't
			// need resolving, and we can't tell without being
			// complicated with types.
			ast.Walk(visit, n.Value)
			return false
		case *ast.SelectorExpr:
			ast.Walk(visit, n.X)
		case *ast.File:
			for _, d := range n.Decls {
				ast.Walk(visit, d)
			}
			return false
		default:
			return true
		}
		return false
	}

	ast.Walk(visit, pkg)
}

var typeMatch *string = flag.String("type", "Superblock", "the type for which you want to scan")
var showDebug *bool = flag.Bool("debug", false, "display debug information")

func main() {
	flag.Parse()
	pkgs, _ := parser.ParseFiles(types.FileSet, flag.Args(), 0)
	for _, pkg := range pkgs {
		fmt.Printf("Checking package '%s'\n", pkg.Name)
		for _, f := range pkg.Files {
			fmt.Printf("Scanning %s\n", types.FileSet.Position(f.Package).Filename)
			checkExprs(f)
		}
	}
}
