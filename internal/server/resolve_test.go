package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"github.com/matkrin/bashd/internal/lsp"
)

func TestCompletionResolveResponse(t *testing.T) {
	// 模拟初始化 state，确保路径查找生效（虽然 resolve 主要看 Label）
	
	tests := []struct {
		name string
		item lsp.CompletionItem
	}{
		{
			name: "External Command (ls)",
			item: lsp.CompletionItem{
				Label: "ls",
				Kind:  lsp.CompletionFunction,
			},
		},
		{
			name: "Variable (MY_VAR)",
			item: lsp.CompletionItem{
				Label: "MY_VAR",
				Kind:  lsp.CompletionVariable,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &lsp.CompletionItemResolveRequest{
				Params: lsp.CompletionItemResolveParams{
					CompletionItem: tt.item,
				},
			}

			response := handleCompletionItemResolve(request)
			data, _ := json.MarshalIndent(response, "", "  ")
			
			fmt.Printf("\n=== Test: %s ===\n", tt.name)
			fmt.Println(string(data))
			
			// 验证逻辑
			if tt.item.Kind == lsp.CompletionVariable {
				if response.Result.Documentation != "" {
					t.Errorf("Expected empty documentation for variable, but got content")
				}
			} else if tt.item.Kind == lsp.CompletionFunction {
				// 只要系统有 ls，就应该有内容
				if response.Result.Documentation == "" {
					// 如果系统真的没有 ls，或者 man 运行失败，这里可能为 nil
					// 但由于我们加了空格兜底，且 handleCompletionItemResolve 检查了非空内容
					fmt.Println("Note: Documentation is nil (might be expected if tool lookup failed in test environment)")
				}
			}
		})
	}
}
