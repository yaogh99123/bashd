package ast

import (
	"os"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type Cursor struct {
	Line uint
	Col  uint
}

func NewCursor(lspLine, lspCol uint) Cursor {
	return Cursor{Line: lspLine + 1, Col: lspCol + 1}
}

func (c *Cursor) isCursorInNode(node syntax.Node) bool {
	startLine := node.Pos().Line()
	startCol := node.Pos().Col()
	endLine := node.End().Line()
	endCol := node.End().Col()

	if c.Line < startLine || c.Line > endLine {
		return false
	}

	if startLine == endLine {
		return c.Line == startLine &&
			c.Col >= startCol && c.Col <= endCol
	}

	switch c.Line {
	case startLine:
		return c.Col >= startCol
	case endLine:
		return c.Col <= endCol
	default:
		return true
	}
}

type Ast struct {
	File *syntax.File
}

func ParseDocument(documentText, documentName string, fallible bool) (*Ast, error) {
	reader := strings.NewReader(documentText)
	var parser *syntax.Parser
	if fallible {
		parser = syntax.NewParser(syntax.KeepComments(true), syntax.RecoverErrors(9999))
	} else {
		parser = syntax.NewParser(syntax.KeepComments(true))
	}
	file, err := parser.Parse(reader, documentName)
	if err != nil {
		return nil, err
	}
	return &Ast{File: file}, nil
}

func (a *Ast) FindNodeUnderCursor(cursor Cursor) syntax.Node {
	var found syntax.Node

	syntax.Walk(a.File, func(node syntax.Node) bool {
		if node == nil {
			return true
		}
		if cursor.isCursorInNode(node) {
			found = node
			return true
		}
		return true
	})

	return found
}

func ExtractIdentifier(node syntax.Node) string {
	switch n := node.(type) {
	case *syntax.Lit:
		return n.Value
	case *syntax.ParamExp:
		if n.Param != nil {
			return n.Param.Value
		}
	case *syntax.Word:
		if len(n.Parts) == 1 {
			switch p := n.Parts[0].(type) {
			case *syntax.Lit:
				return p.Value
			}
		}
	case *syntax.Assign:
		if n.Name != nil {
			return n.Name.Value
		}
	case *syntax.FuncDecl:
		if n.Name != nil {
			return n.Name.Value
		}
	case *syntax.DeclClause:
		if n.Variant != nil {
			return n.Variant.Value
		}
	case *syntax.CoprocClause:
		return "coproc"
	}
	return ""
}

func extractAndExpandWord(word *syntax.Word, env map[string]string) string {
	var b strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)

		case *syntax.ParamExp:
			val := env[p.Param.Value]
			b.WriteString(val)

		case *syntax.SglQuoted:
			b.WriteString(p.Value)

		case *syntax.DblQuoted:
			for _, qpart := range p.Parts {
				switch qp := qpart.(type) {
				case *syntax.Lit:
					b.WriteString(qp.Value)
				case *syntax.ParamExp:
					val := env[qp.Param.Value]
					b.WriteString(val)
				}
			}
		}
	}

	return os.Expand(b.String(), func(key string) string {
		return env[key]
	})
}

func (a *Ast) findEnclosingFunction(cursor Cursor) *syntax.FuncDecl {
	var enclosingFunc *syntax.FuncDecl

	syntax.Walk(a.File, func(node syntax.Node) bool {
		fn, ok := node.(*syntax.FuncDecl)
		if !ok {
			return true
		}
		if cursor.isCursorInNode(fn) {
			enclosingFunc = fn
		}
		return true
	})

	return enclosingFunc
}
