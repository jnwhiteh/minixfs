package main

import (
	"go/ast"
	"log"
)

func debug(fmt string) {
        if *showDebug {
                log.Print(fmt)
        }
}

func debugf(fmt string, args ... interface{}) {
        if *showDebug {
                log.Printf(fmt, args...)
        }
}

type astVisitor func(n ast.Node) bool

func (f astVisitor) Visit(n ast.Node) ast.Visitor {
	if f(n) {
		return f
	}
	return nil
}
