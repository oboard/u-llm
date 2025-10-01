package main

import "fmt"

// extractTextContent extracts text content from either string or array format
func extractTextContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		// Handle array of content parts (like from OpenAI API)
		for _, part := range v {
			if partMap, ok := part.(map[string]any); ok {
				if partType, exists := partMap["type"]; exists && partType == "text" {
					if text, exists := partMap["text"]; exists {
						if textStr, ok := text.(string); ok {
							return textStr
						}
					}
				}
			}
		}
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func help() {
	fmt.Printf("使用方法: ullm <command> [arguments]\n")
	fmt.Printf("可用命令:\n")
	fmt.Printf("  serve --port PORT [--debug]    启动HTTP服务器\n")
	fmt.Printf("    --port PORT     指定服务器端口号 (默认: 8080)\n")
	fmt.Printf("    --debug         启用调试模式，打印详细的客户端请求日志，包括404错误\n")
}
