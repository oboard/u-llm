package main

import (
	"fmt"
	"strings"
)

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

// extractFinalPrompt 统一处理system prompt拼接逻辑
// 支持两种消息格式：
// 1. ChatCompletions格式：[]struct{Role string, Content any}
// 2. Responses格式：[]interface{}
func extractFinalPrompt(messages any) string {
	var promptBuilder strings.Builder
	var hasSystemMessage bool
	var userContent string

	switch msgs := messages.(type) {
	case []interface{}:
		// handleResponses 格式：[]interface{}
		// 第一遍：处理系统消息
		for _, item := range msgs {
			if msgMap, ok := item.(map[string]interface{}); ok {
				if role, exists := msgMap["role"]; exists && role == "system" {
					if content, exists := msgMap["content"]; exists {
						var systemContent string
						// 处理系统消息内容（可能是字符串或数组）
						switch sysContent := content.(type) {
						case string:
							systemContent = sysContent
						case []interface{}:
							// 如果系统消息也是数组格式，提取文本
							for _, sysItem := range sysContent {
								if sysMap, ok := sysItem.(map[string]interface{}); ok {
									if sysType, exists := sysMap["type"]; exists && sysType == "input_text" {
										if text, exists := sysMap["text"]; exists {
											if textStr, ok := text.(string); ok {
												systemContent = textStr
												break
											}
										}
									}
								}
							}
						default:
							systemContent = fmt.Sprintf("%v", content)
						}

						if systemContent != "" {
							promptBuilder.WriteString("System: ")
							promptBuilder.WriteString(systemContent)
							promptBuilder.WriteString("\n\n")
							hasSystemMessage = true
						}
					}
				}
			}
		}

		// 第二遍：处理用户消息
		for _, item := range msgs {
			if msgMap, ok := item.(map[string]interface{}); ok {
				if role, exists := msgMap["role"]; exists && role == "user" {
					if content, exists := msgMap["content"]; exists {
						if contentArray, ok := content.([]interface{}); ok {
							for _, contentItem := range contentArray {
								if contentMap, ok := contentItem.(map[string]interface{}); ok {
									if contentType, exists := contentMap["type"]; exists && contentType == "input_text" {
										if text, exists := contentMap["text"]; exists {
											if textStr, ok := text.(string); ok {
												userContent = textStr
												break
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

	case []struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}:
		// handleChatCompletions 格式：[]struct{Role string, Content any}
		// 第一遍：处理系统消息
		for _, msg := range msgs {
			if msg.Role == "system" {
				systemContent := extractTextContent(msg.Content)
				if systemContent != "" {
					promptBuilder.WriteString("System: ")
					promptBuilder.WriteString(systemContent)
					promptBuilder.WriteString("\n\n")
					hasSystemMessage = true
				}
			}
		}

		// 第二遍：处理用户消息，取最后一个用户消息
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == "user" {
				userContent = extractTextContent(msgs[i].Content)
				if userContent != "" {
					break
				}
			}
		}

	default:
		// 未知格式，返回空字符串让调用方使用原有逻辑
		return ""
	}

	// 构建最终prompt
	if hasSystemMessage && userContent != "" {
		// 有系统消息时，添加用户前缀
		promptBuilder.WriteString("User: ")
		promptBuilder.WriteString(userContent)
		return promptBuilder.String()
	} else if userContent != "" {
		// 没有系统消息时，直接使用用户内容
		return userContent
	}

	return ""
}

func help() {
	fmt.Printf("使用方法: ullm <command> [arguments]\n")
	fmt.Printf("可用命令:\n")
	fmt.Printf("  serve --port PORT [--debug]    启动HTTP服务器\n")
	fmt.Printf("    --port PORT     指定服务器端口号 (默认: 8080)\n")
	fmt.Printf("    --debug         启用调试模式，打印详细的客户端请求日志，包括404错误\n")
}
