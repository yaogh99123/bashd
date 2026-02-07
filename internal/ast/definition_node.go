package ast

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type DefNode struct {
	Node      syntax.Node
	Name      string
	Scope     *syntax.FuncDecl // nil for global scope
	IsScoped  bool             // true for local/declare/typeset variables
	StartLine uint
	StartChar uint
	EndLine   uint
	EndChar   uint
}

func (d *DefNode) isBeforeCursor(cursor Cursor) bool {
	if d.StartLine <= cursor.Line {
		return true
	}
	if d.StartLine == cursor.Line && d.StartChar < cursor.Col {
		return true
	}
	return false
}

func (d *DefNode) isDefinitionAfter(otherDef *DefNode) bool {
	if d.StartLine > otherDef.StartLine {
		return true
	}
	if d.StartLine == otherDef.StartLine && d.StartChar > otherDef.StartChar {
		return true
	}
	return false
}

func (d *DefNode) isSameDefinition(def2 *DefNode) bool {
	return d.StartLine == def2.StartLine &&
		d.StartChar == def2.StartChar &&
		d.Name == def2.Name
}

func assignToDefNode(assignNode *syntax.Assign) *DefNode {
	if assignNode.Name == nil {
		return nil
	}

	name := assignNode.Name.Value
	startLine, startChar := assignNode.Name.Pos().Line(), assignNode.Name.Pos().Col()
	endLine, endChar := assignNode.Name.End().Line(), assignNode.Name.End().Col()

	return &DefNode{
		Node:      assignNode,
		Name:      name,
		Scope:     nil,
		IsScoped:  false,
		StartLine: startLine,
		StartChar: startChar,
		EndLine:   endLine,
		EndChar:   endChar,
	}

}

func funcDeclToDefNode(funcDecl *syntax.FuncDecl) *DefNode {
	if funcDecl.Name == nil {
		return nil
	}

	name := funcDecl.Name.Value
	startLine, startChar := funcDecl.Name.Pos().Line(), funcDecl.Name.Pos().Col()
	endLine, endChar := funcDecl.Name.End().Line(), funcDecl.Name.End().Col()

	return &DefNode{
		Node:      funcDecl,
		Name:      name,
		Scope:     nil,
		IsScoped:  false,
		StartLine: startLine,
		StartChar: startChar,
		EndLine:   endLine,
		EndChar:   endChar,
	}
}

func declClauseToDefNode(declClause *syntax.DeclClause, scope *syntax.FuncDecl) []DefNode {
	cmd := declClause.Variant.Value
	if cmd != "local" && cmd != "declare" && cmd != "typeset" {
		return nil
	}

	var defNodes []DefNode
	for _, arg := range declClause.Args {
		if arg.Name != nil {
			name := arg.Name.Value
			startLine, startChar := arg.Name.ValuePos.Line(), arg.Name.ValuePos.Col()
			endLine, endChar := arg.Name.ValueEnd.Line(), arg.Name.ValueEnd.Col()

			defNodes = append(defNodes, DefNode{
				Node:      declClause,
				Name:      name,
				Scope:     scope,
				IsScoped:  scope != nil,
				StartLine: startLine,
				StartChar: startChar,
				EndLine:   endLine,
				EndChar:   endChar,
			})
		}
	}
	return defNodes
}
func forClauseToDefNode(forClause *syntax.ForClause, scope *syntax.FuncDecl) *DefNode {
	var name string
	var startLine, startChar, endLine, endChar uint

	switch loop := forClause.Loop.(type) {
	case *syntax.WordIter:
		if loop.Name == nil {
			return nil
		}
		name = loop.Name.Value
		startLine, startChar = loop.Name.Pos().Line(), loop.Name.Pos().Col()
		endLine, endChar = loop.Name.End().Line(), loop.Name.End().Col()

	case *syntax.CStyleLoop:
		if loop.Init == nil {
			return nil
		}
		a, ok := loop.Init.(*syntax.BinaryArithm)
		if !ok {
			return nil
		}
		if a.Op == syntax.Assgn {
			word, ok := a.X.(*syntax.Word)
			if !ok {
				return nil
			}
			for _, wp := range word.Parts {
				switch p := wp.(type) {
				case *syntax.Lit:
					name = p.Value
					startLine, startChar = p.Pos().Line(), p.Pos().Col()
					endLine, endChar = p.End().Line(), p.End().Col()
				}
			}
		}
	}

	if name == "" {
		return nil
	}

	return &DefNode{
		Node:      forClause,
		Name:      name,
		Scope:     scope,
		IsScoped:  scope != nil,
		StartLine: startLine,
		StartChar: startChar,
		EndLine:   endLine,
		EndChar:   endChar,
	}
}

func callExprToDefNode(callExpr *syntax.CallExpr, scope *syntax.FuncDecl) []DefNode {
	if len(callExpr.Args) < 2 {
		return nil
	}
	cmdName := ExtractIdentifier(callExpr.Args[0])
	if cmdName != "read" {
		return nil
	}

	var defNodes []DefNode
	for _, arg := range callExpr.Args[1:] {
		name := ExtractIdentifier(arg)
		if name == "" || strings.HasPrefix(name, "-") {
			continue
		}

		startLine, startChar := arg.Pos().Line(), arg.Pos().Col()
		endLine, endChar := arg.End().Line(), arg.End().Col()
		defNodes = append(defNodes, DefNode{
			Node:      callExpr,
			Name:      name,
			Scope:     scope,
			IsScoped:  scope != nil,
			StartLine: startLine,
			StartChar: startChar,
			EndLine:   endLine,
			EndChar:   endChar,
		})
	}
	return defNodes
}

func handleNode(node syntax.Node, scope *syntax.FuncDecl) ([]DefNode, bool) {
	descent := true
	var defNodes []DefNode
	switch n := node.(type) {
	case *syntax.Assign:
		if defNode := assignToDefNode(n); defNode != nil {
			defNodes = append(defNodes, *defNode)
		}

	case *syntax.DeclClause:
		if nodes := declClauseToDefNode(n, scope); len(nodes) > 0 {
			defNodes = append(defNodes, nodes...)
		}
		descent = false

	case *syntax.ForClause:
		if defNode := forClauseToDefNode(n, scope); defNode != nil {
			defNodes = append(defNodes, *defNode)
		}

	case *syntax.CallExpr:
		if nodes := callExprToDefNode(n, scope); len(nodes) > 0 {
			defNodes = append(defNodes, nodes...)
		}

	}

	return defNodes, descent
}

func (a *Ast) DefNodes() []DefNode {
	defNodes := []DefNode{}

	syntax.Walk(a.File, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.FuncDecl:
			if defNode := funcDeclToDefNode(n); defNode != nil {
				defNodes = append(defNodes, *defNode)
			}
			syntax.Walk(n.Body, func(innerNode syntax.Node) bool {
				nodes, descent := handleNode(innerNode, n)
				if len(nodes) > 0 {
					defNodes = append(defNodes, nodes...)
				}
				return descent
			})
			return false
		}

		nodes, descent := handleNode(node, nil);
		if len(nodes) > 0 {
			defNodes = append(defNodes, nodes...)
		}

		return descent
	})

	return defNodes
}

func (a *Ast) FindDefInFile(cursor Cursor) *DefNode {
	cursorNode := a.FindNodeUnderCursor(cursor)
	targetIdentifier := ExtractIdentifier(cursorNode)
	if targetIdentifier == "" {
		return nil
	}

	cursorScope := a.findEnclosingFunction(cursor)

	// Find scoped variables in the same function scope as cursor
	if cursorScope != nil {
		for _, defNode := range a.DefNodes() {
			if defNode.Name == targetIdentifier && defNode.IsScoped && defNode.Scope == cursorScope {
				// Check if the scoped variable is declared before the cursor position (shadowing)
				if defNode.isBeforeCursor(cursor) {
					return &defNode
				}
			}
		}
	}

	// Find global definitions i.e., functions and non-scoped variables
	for _, defNode := range a.DefNodes() {
		if defNode.Name == targetIdentifier {
			if defNode.IsScoped {
				continue
			}
			return &defNode
		}
	}

	return nil
}
