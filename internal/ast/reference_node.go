package ast

import (
	"log/slog"
	"strconv"

	"github.com/matkrin/bashd/internal/lsp"
	"mvdan.cc/sh/v3/syntax"
)

type RefNode struct {
	Node      syntax.Node
	Name      string
	Scope     *syntax.FuncDecl
	StartLine uint
	StartChar uint
	EndLine   uint
	EndChar   uint
}

func (r *RefNode) ToLspLocation(uri string) lsp.Location {
	return lsp.Location{
		URI: uri,
		Range: lsp.NewRange(
			r.StartLine-1,
			r.StartChar-1,
			r.EndLine-1,
			r.EndChar-1,
		),
	}
}

func (r *RefNode) ToLspTextEdit(newText string) lsp.TextEdit {
	return lsp.TextEdit{
		Range: lsp.NewRange(
			r.StartLine-1,
			r.StartChar-1,
			r.EndLine-1,
			r.EndChar-1,
		),
		NewText: newText,
	}
}

func paramExpToRefNode(paramExp *syntax.ParamExp) *RefNode {
	if paramExp.Param == nil {
		return nil
	}

	name := paramExp.Param.Value
	startLine, startChar := paramExp.Param.Pos().Line(), paramExp.Param.Pos().Col()
	endLine, endChar := paramExp.Param.End().Line(), paramExp.Param.End().Col()

	return &RefNode{
		Node:      paramExp,
		Name:      name,
		Scope:     nil,
		StartLine: startLine,
		StartChar: startChar,
		EndLine:   endLine,
		EndChar:   endChar,
	}
}

func (a *Ast) RefNodes(includeDeclaration bool) []RefNode {
	refNodes := []RefNode{}

	syntax.Walk(a.File, func(node syntax.Node) bool {
		nodes, descent := syntaxNodeToRefNode(node, nil, includeDeclaration)
		if len(nodes) > 0 {
			refNodes = append(refNodes, nodes...)
		}

		return descent
	})

	return refNodes
}

func (a *Ast) FindRefsInFile(cursor Cursor, includeDeclaration bool) []RefNode {
	cursorNode := a.FindNodeUnderCursor(cursor)
	targetIdentifier := ExtractIdentifier(cursorNode)
	if targetIdentifier == "" {
		return nil
	}
	slog.Info("REFS", "includeDeclaration", includeDeclaration)

	references := []RefNode{}

	defNode := a.FindDefInFile(cursor)

	slog.Info("FINDREFS", "DEFNODE", defNode)

	if defNode == nil {
		// No definition found - return all references with same name (fallback behavior)
		for _, refNode := range a.RefNodes(includeDeclaration) {
			if refNode.Name == targetIdentifier {
				references = append(references, refNode)
			}
		}
		return references
	}

	// Definition found - find all references that would resolve to this same definition
	for _, refNode := range a.RefNodes(includeDeclaration) {
		if refNode.Name != targetIdentifier {
			continue
		}

		if a.wouldResolveToSameDefinition(refNode.Node, defNode) {
			references = append(references, refNode)
		}
	}

	return references
}

func syntaxNodeToRefNode(node syntax.Node, scope *syntax.FuncDecl, includeDeclaration bool) ([]RefNode, bool) {
	descent := true
	var refNodes []RefNode
	switch n := node.(type) {
	case *syntax.Assign:
		if refNode := assignToRefNode(n, includeDeclaration); refNode != nil {
			refNodes = append(refNodes, *refNode)
		}

	case *syntax.DeclClause:
		if nodes := declClauseToRefNode(n, scope, includeDeclaration); len(nodes) > 0 {
			refNodes = append(refNodes, nodes...)
		}
		descent = false

	case *syntax.ForClause:
		if refNode := forClauseToRefNode(n, includeDeclaration); refNode != nil {
			refNodes = append(refNodes, *refNode)
		}

	case *syntax.CallExpr:
		if refNode := callExprToRefNode(n, scope, includeDeclaration); refNode != nil {
			refNodes = append(refNodes, *refNode)
		}

	case *syntax.FuncDecl:
		if refNode := funcDeclToRefNode(n, includeDeclaration); refNode != nil {
			refNodes = append(refNodes, *refNode)
		}
		syntax.Walk(n.Body, func(innerNode syntax.Node) bool {
			nodes, descent := syntaxNodeToRefNode(innerNode, n, includeDeclaration)
			if len(nodes) > 0 {
				refNodes = append(refNodes, nodes...)
			}
			return descent
		})
		descent = false

	case *syntax.ParamExp:
		if refNode := paramExpToRefNode(n); refNode != nil {
			refNodes = append(refNodes, *refNode)
		}

	case *syntax.ArithmExp:
		if nodes := arithmExpToRefNode(n, scope); len(nodes) > 0 {
			refNodes = append(refNodes, nodes...)
		}

	}

	return refNodes, descent
}

func callExprToRefNode(callExpr *syntax.CallExpr, scope *syntax.FuncDecl, includeDeclaration bool) *RefNode {
	var name string
	var startLine, startChar, endLine, endChar uint

	if len(callExpr.Args) > 0 {
		cmdName := ExtractIdentifier(callExpr.Args[0])

		// Variable assignments as part of read statements
		if cmdName == "read" && includeDeclaration {
			for _, arg := range callExpr.Args[1:] {
				for _, wp := range arg.Parts {
					switch p := wp.(type) {
					case *syntax.Lit:
						name = p.Value
						startLine, startChar = p.Pos().Line(), p.Pos().Col()
						endLine, endChar = p.End().Line(), p.End().Col()
					}
				}
			}
		} else if cmdName != "" && cmdName != "local" && cmdName != "declare" && cmdName != "typeset" && cmdName != "read" {
			arg := callExpr.Args[0]
			for _, wp := range arg.Parts {
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

	return &RefNode{
		Node:      callExpr,
		Name:      name,
		Scope:     scope,
		StartLine: startLine,
		StartChar: startChar,
		EndLine:   endLine,
		EndChar:   endChar,
	}
}

func funcDeclToRefNode(funcDecl *syntax.FuncDecl, includeDeclaration bool) *RefNode {
	if funcDecl.Name == nil || !includeDeclaration {
		return nil
	}

	name := funcDecl.Name.Value
	startLine, startChar := funcDecl.Name.Pos().Line(), funcDecl.Name.Pos().Col()
	endLine, endChar := funcDecl.Name.End().Line(), funcDecl.Name.End().Col()

	return &RefNode{
		Node:      funcDecl,
		Name:      name,
		Scope:     nil,
		StartLine: startLine,
		StartChar: startChar,
		EndLine:   endLine,
		EndChar:   endChar,
	}
}

func assignToRefNode(assignNode *syntax.Assign, includeDeclaration bool) *RefNode {
	if assignNode.Name == nil || !includeDeclaration {
		return nil
	}

	name := assignNode.Name.Value
	startLine, startChar := assignNode.Name.Pos().Line(), assignNode.Name.Pos().Col()
	endLine, endChar := assignNode.Name.End().Line(), assignNode.Name.End().Col()

	return &RefNode{
		Node:      assignNode,
		Name:      name,
		Scope:     nil,
		StartLine: startLine,
		StartChar: startChar,
		EndLine:   endLine,
		EndChar:   endChar,
	}
}

func declClauseToRefNode(declClause *syntax.DeclClause, scope *syntax.FuncDecl, includeDeclaration bool) []RefNode {
	if !includeDeclaration {
		return nil
	}

	cmd := declClause.Variant.Value
	if cmd != "local" && cmd != "declare" && cmd != "typeset" {
		return nil
	}

	var refNodes []RefNode
	for _, arg := range declClause.Args {
		if arg.Name != nil {
			name := arg.Name.Value
			startLine, startChar := arg.Name.ValuePos.Line(), arg.Name.ValuePos.Col()
			endLine, endChar := arg.Name.ValueEnd.Line(), arg.Name.ValueEnd.Col()

			refNodes = append(refNodes, RefNode{
				Node:      declClause,
				Name:      name,
				Scope:     scope,
				StartLine: startLine,
				StartChar: startChar,
				EndLine:   endLine,
				EndChar:   endChar,
			})
		}
	}

	return refNodes
}

func forClauseToRefNode(forClause *syntax.ForClause, includeDeclaration bool) *RefNode {
	if !includeDeclaration {
		return nil
	}

	var name string
	var startLine, startChar, endLine, endChar uint

	switch loop := forClause.Loop.(type) {
	case *syntax.WordIter:
		if loop.Name != nil {
			name = loop.Name.Value
			startLine, startChar = loop.Name.Pos().Line(), loop.Name.Pos().Col()
			endLine, endChar = loop.Name.End().Line(), loop.Name.End().Col()
		}

	case *syntax.CStyleLoop:
		if loop.Init != nil {
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
	}

	return &RefNode{
		Node:      forClause,
		Name:      name,
		Scope:     nil,
		StartLine: startLine,
		StartChar: startChar,
		EndLine:   endLine,
		EndChar:   endChar,
	}

}

func arithmExpToRefNode(arithmExp *syntax.ArithmExp, scope *syntax.FuncDecl) []RefNode {
    var refNodes []RefNode

    var walkArithm func(syntax.ArithmExpr)
    walkArithm = func(expr syntax.ArithmExpr) {
        switch e := expr.(type) {

        case *syntax.Word:
            for _, wp := range e.Parts {
                if lit, ok := wp.(*syntax.Lit); ok {
                    name := lit.Value

                    // Ignore numbers
                    if _, err := strconv.Atoi(name); err == nil {
                        continue
                    }

                    refNodes = append(refNodes, RefNode{
                        Node:      arithmExp,
                        Name:      name,
                        Scope:     scope,
                        StartLine: lit.Pos().Line(),
                        StartChar: lit.Pos().Col(),
                        EndLine:   lit.End().Line(),
                        EndChar:   lit.End().Col(),
                    })
                }
            }

        case *syntax.BinaryArithm:
            walkArithm(e.X)
            walkArithm(e.Y)

        case *syntax.UnaryArithm:
            walkArithm(e.X)

        case *syntax.ParenArithm:
            walkArithm(e.X)
        }
    }

    walkArithm(arithmExp.X)

    return refNodes
}

func (a *Ast) wouldResolveToSameDefinition(refCursorNode syntax.Node, targetDefNode *DefNode) bool {
	pos := refCursorNode.Pos()
	cursor := Cursor{Line: pos.Line(), Col: pos.Col()}
	refScope := a.findEnclosingFunction(cursor)

	targetIdentifier := targetDefNode.Name

	// Look for scoped variables in the same function scope
	if refScope != nil {
		var closestLocalDef *DefNode

		for _, defNode := range a.DefNodes() {
			if defNode.Name == targetIdentifier && defNode.IsScoped && defNode.Scope == refScope {
				// Check if the scoped variable is declared BEFORE the reference position
				if defNode.isBeforeCursor(cursor) {
					// Among all local variables declared before this reference,
					// find the one that's closest (latest declaration)
					if closestLocalDef == nil || defNode.isDefinitionAfter(closestLocalDef) {
						closestLocalDef = &defNode
					}
				}
			}
		}

		// If we found a local variable declared before the reference, use it
		if closestLocalDef != nil {
			return closestLocalDef.isSameDefinition(targetDefNode)
		}
	}

	// No local variable found that's declared before the reference
	// Look for global definitions (functions and non-scoped variables)
	for _, defNode := range a.DefNodes() {
		if defNode.Name == targetIdentifier {
			if defNode.IsScoped {
				continue
			}
			return defNode.isSameDefinition(targetDefNode)
		}
	}

	return false
}
