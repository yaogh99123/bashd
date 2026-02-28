package ast

import (
	"slices"
	"testing"
)

func Test_RefNodes(t *testing.T) {
	input := `
a="global"

foo() {
  local b="scoped"
  c="global_in_func"
  declare d=123
  typeset e
  echo "$a $b $c"
}

read f

for g in 1 2 3; do
  echo $e
done
`
	fileAst, _ := ParseDocument(input, "", false)
	refNodes := fileAst.RefNodes(true)
	for _, r := range refNodes {
		t.Log(r.Name)
	}

	expected := []struct {
		name      string
		startLine uint
	}{
		{"a", 2},
		{"foo", 4},
		{"b", 5},
		{"c", 6},
		{"d", 7},
		{"e", 8},
		{"echo", 9},
		{"a", 9},
		{"b", 9},
		{"c", 9},
		{"f", 12},
		{"g", 14},
		{"echo", 15},
		{"e", 15},
	}

	if len(refNodes) != len(expected) {
		t.Fatalf("expected %d definition nodes, got %d", len(expected), len(refNodes))
	}

	for i := range refNodes {
		if refNodes[i].Name != expected[i].name {
			t.Errorf("expected '%s', got '%s'", expected[i].name, refNodes[i].Name)
		}
		if refNodes[i].StartLine != expected[i].startLine {
			t.Errorf("expected '%d', got '%d'", expected[i].startLine, refNodes[i].StartLine)
		}
	}
}

func Test_FindRefInFile(t *testing.T) {
	input := `
a="global"
b="global2"

foo() {
  local b="scoped"
  c="global_in_func"
  echo "$a $b $c"
  local a="shadows"
  echo "$a"
}

echo "$a $b $c"
`
	fileAst, _ := ParseDocument(input, "", false)

	tests := []struct {
		cursor     Cursor
		name       string
		startLines []uint
	}{
		{NewCursor(1, 1), "a", []uint{2, 8, 13}},
		{NewCursor(2, 12), "b", []uint{3, 13}},
		{NewCursor(5, 9), "b", []uint{6, 8}},
		{NewCursor(6, 3), "c", []uint{7, 8, 13}},
		{NewCursor(9, 10), "a", []uint{9, 10}},
	}

	for _, e := range tests {
		refNodes := fileAst.FindRefsInFile(e.cursor, true)
		for _, refNode := range refNodes {
			t.Logf("%s - %d", refNode.Name, refNode.StartLine)
			if refNode.Name != e.name {
				t.Errorf("expected '%s', got '%s'", e.name, refNode.Name)
			}
			if !slices.Contains(e.startLines, refNode.StartLine) {
				t.Errorf("expected '%v', got '%d'", e.startLines, refNode.StartLine)
			}
		}
	}
}

func Test_RefNodes_Arithmetic(t *testing.T) {
	input := `
a=1
b=2

echo $((a + b))
((a++))
((c = a + b))

for ((i=0; i<10; i++)); do
  echo $i
done
`
	fileAst, _ := ParseDocument(input, "", false)
	refNodes := fileAst.RefNodes(true)

	expected := []struct {
		name string
		line uint
	}{
		{"a", 2},
		{"b", 3},

		// echo
		{"echo", 5},
		{"a", 5},
		{"b", 5},

		// ((a++))
		{"a", 6},

		// ((c = a + b))
		{"c", 7},
		{"a", 7},
		{"b", 7},

		// for ((i=0; i<10; i++))
		{"i", 9},
		{"i", 9},
		{"i", 9},

		{"echo", 10},
		{"i", 10},
	}

	if len(refNodes) != len(expected) {
		t.Fatalf("expected %d refs, got %d", len(expected), len(refNodes))
	}

	for i := range expected {
		if refNodes[i].Name != expected[i].name {
			t.Errorf("expected name %q, got %q", expected[i].name, refNodes[i].Name)
		}
		if refNodes[i].StartLine != expected[i].line {
			t.Errorf("expected line %d, got %d", expected[i].line, refNodes[i].StartLine)
		}
	}
}

func Test_FindRefsInFile_Arithmetic(t *testing.T) {
	input := `
a=1

foo() {
  local a=2
  echo $((a + 1))
}

echo $((a + 1))
`
	fileAst, _ := ParseDocument(input, "", false)

	tests := []struct {
		cursor     Cursor
		name       string
		startLines []uint
	}{
		{NewCursor(1, 1), "a", []uint{2, 9}},  // global `a`
		{NewCursor(4, 9), "a", []uint{5, 6}}, // local `a`
	}

	for _, tt := range tests {
		refNodes := fileAst.FindRefsInFile(tt.cursor, true)

		if len(refNodes) == 0 {
			t.Fatalf("no refs found for %s", tt.name)
		}

		for _, r := range refNodes {
			if r.Name != tt.name {
				t.Errorf("expected name %q, got %q", tt.name, r.Name)
			}
			if !slices.Contains(tt.startLines, r.StartLine) {
				t.Errorf("unexpected line %d for %s", r.StartLine, tt.name)
			}
		}
	}
}

func Test_RefNodes_Arithmetic_WithParamExp(t *testing.T) {
	input := `
a=1
echo $(( $a + 1 ))
`
	fileAst, _ := ParseDocument(input, "", false)
	refNodes := fileAst.RefNodes(true)

	expectedNames := []string{
		"a", // definition
		"echo",
		"a", // arithmetic reference
	}

	if len(refNodes) != len(expectedNames) {
		t.Fatalf("expected %d refs, got %d", len(expectedNames), len(refNodes))
	}

	for i := range expectedNames {
		if refNodes[i].Name != expectedNames[i] {
			t.Errorf("expected %q, got %q", expectedNames[i], refNodes[i].Name)
		}
	}
}
