package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

type LoginRequest struct {
	LoginName  string `json:"loginName"`
	Password   string `json:"password"`
	Device     string `json:"device"`
	AppVersion string `json:"appVersion"`
	WebEnv     string `json:"webEnv"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type TokenCache struct {
	Token      string    `json:"token"`
	ExpireTime time.Time `json:"expireTime"`
}

type ChatCompletionsRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string      `json:"role"`
		Content interface{} `json:"content"`
	} `json:"messages"`
	Temperature   *float64 `json:"temperature,omitempty"`
	Stream        *bool    `json:"stream,omitempty"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage,omitempty"`
	} `json:"stream_options,omitempty"`
}

type ChatCompletionsResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	} `json:"choices"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// 历史记录相关结构体
type HistoryItem struct {
	Query       string        `json:"query"`
	CustomTitle string        `json:"customTitle"`
	Answer      string        `json:"answer"`
	Type        int           `json:"type"`
	Infos       []interface{} `json:"infos"`
	UserID      int64         `json:"userId"`
	SessionID   string        `json:"sessionId"`
	RequestID   string        `json:"requestId"`
	CreateTime  int64         `json:"createTime"`
	AssistantID int           `json:"assistantId"`
	RoleID      int           `json:"roleId"`
	SessionSign int           `json:"sessionSign"`
	References  []interface{} `json:"references"`
}

type HistoryResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Result  [][]HistoryItem `json:"result"`
}

// OpenAI格式的历史记录结构
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIConversation struct {
	ID       string          `json:"id"`
	Object   string          `json:"object"`
	Created  int64           `json:"created"`
	Messages []OpenAIMessage `json:"messages"`
}

type OpenAIHistoryResponse struct {
	Object string               `json:"object"`
	Data   []OpenAIConversation `json:"data"`
}

// extractTextContent extracts text content from either string or array format
func extractTextContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		// Handle array of content parts (like from OpenAI API)
		for _, part := range v {
			if partMap, ok := part.(map[string]interface{}); ok {
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

func getToken() (string, error) {
	// 检查缓存文件是否存在
	if _, err := os.Stat("cache.json"); err == nil {
		// 读取缓存文件
		data, err := os.ReadFile("cache.json")
		if err != nil {
			return "", err
		}

		var cache TokenCache
		if err := json.Unmarshal(data, &cache); err != nil {
			return "", err
		}

		// 检查token是否过期
		if time.Now().Before(cache.ExpireTime) {
			return cache.Token, nil
		}
	}

	// 如果缓存不存在或已过期，重新登录获取token
	loginData := url.Values{}
	loginData.Set("loginName", "hfdhdfhfd")
	loginData.Set("password", "Aa123456")

	// 创建一个 cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", fmt.Errorf("创建 cookie jar 失败: %v", err)
	}

	// 创建一个允许重定向的客户端，并设置 cookie jar
	client := &http.Client{
		Timeout: 10 * time.Second,
		Jar:     jar,
	}

	// 创建请求
	req, err := http.NewRequest("POST", "https://courseapi.ulearning.cn/users/login/v2", strings.NewReader(loginData.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 获取所有 cookies
	cookies := jar.Cookies(req.URL)

	// 从 cookie jar 中获取 token
	var token string
	for _, cookie := range cookies {
		if cookie.Name == "token" {
			token = cookie.Value
			break
		}
	}

	if token == "" {
		// 如果没有找到 token，尝试从 AUTHORIZATION cookie 获取
		for _, cookie := range cookies {
			if cookie.Name == "AUTHORIZATION" {
				token = cookie.Value
				break
			}
		}
	}

	if token == "" {
		// 构建详细的错误信息
		return "", fmt.Errorf("未找到 token cookie (状态码: %d, 响应头: %v, Cookies: %v)",
			resp.StatusCode,
			resp.Header,
			cookies)
	}

	// 缓存token，设置1小时过期
	cache := TokenCache{
		Token:      token,
		ExpireTime: time.Now().Add(time.Hour),
	}

	cacheData, err := json.Marshal(cache)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile("cache.json", cacheData, 0644); err != nil {
		return "", err
	}

	return token, nil
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received chat completions request: %s %s", r.Method, r.URL.Path)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 检查并提取用户的API key
	auth := r.Header.Get("Authorization")
	if auth == "" {
		http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
		return
	}
	userApiKey := strings.TrimPrefix(auth, "Bearer ")
	if userApiKey == auth { // 如果没有Bearer前缀
		userApiKey = auth
	}
	log.Printf("User API key: %s", userApiKey)

	// 读取请求体用于调试
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	log.Printf("Request body: %s", string(body))

	// 解析请求体
	var req ChatCompletionsRequest
	if json.Unmarshal(body, &req) != nil {
		log.Printf("Failed to parse JSON: %v", err)
		http.Error(w, fmt.Sprintf("Invalid request payload: %v", err), http.StatusBadRequest)
		return
	}
	log.Printf("Parsed request - Model: %s, Messages count: %d, Stream: %v", req.Model, len(req.Messages), req.Stream != nil && *req.Stream)

	// 调用 Ulearning API
	// Get token for authentication
	token, err := getToken()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get token: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract text content from the last message
	lastMessage := req.Messages[len(req.Messages)-1]
	textContent := extractTextContent(lastMessage.Content)
	log.Printf("Extracted text content: %s", textContent)

	// Prepare request body
	requestBody := map[string]interface{}{
		"query":  textContent,
		"images": []string{},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, "Failed to marshal request body", http.StatusInternalServerError)
		return
	}

	// Create request to Ulearning API
	apiURL := "https://cloudsearchapi.ulearning.cn/kbChat/chat"
	apiReq, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Set required headers
	apiReq.Header.Set("Authorization", token)
	apiReq.Header.Set("Content-Type", "application/json;charset=UTF-8")

	// Add query parameters
	q := apiReq.URL.Query()
	// 使用用户的API key作为sessionId，确保每个用户有独立的会话
	q.Add("sessionId", userApiKey)
	q.Add("assistantId", "6")

	// 根据模型名称设置对应的modelId
	var modelId string
	switch req.Model {
	case "qianwen":
		modelId = "1"
	case "doubao":
		modelId = "2"
	case "deepseek-r1":
		modelId = "3"
	case "qianwen-vl":
		modelId = "4"
	case "deepseek-r1-online":
		modelId = "5"
	case "deepseek-r1-local":
		modelId = "6"
	default:
		modelId = "2" // 默认使用豆包
	}
	q.Add("modelId", modelId)

	q.Add("sessionSign", "2")
	q.Add("askType", "1")
	q.Add("requestId", "23213")
	apiReq.URL.RawQuery = q.Encode()

	// Send request
	client := &http.Client{}
	resp, err := client.Do(apiReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("API request failed with status: %d", resp.StatusCode), resp.StatusCode)
		return
	}

	// Read and parse streaming response (SSE format)
	scanner := bufio.NewScanner(resp.Body)
	var fullContent string
	for scanner.Scan() {
		line := scanner.Text()

		// Handle Server-Sent Events format
		if strings.HasPrefix(line, "data:") {
			// Extract JSON data after "data:" prefix
			jsonData := strings.TrimPrefix(line, "data:")
			var streamResp struct {
				Data string `json:"data"`
			}
			if err := json.Unmarshal([]byte(jsonData), &streamResp); err == nil {
				fullContent += streamResp.Data
			}
		}
	}

	if err := scanner.Err(); err != nil {
		http.Error(w, "Failed to read response stream", http.StatusInternalServerError)
		return
	}

	// 检查并处理空内容
	if strings.TrimSpace(fullContent) == "" {
		fullContent = "抱歉，我无法处理您的请求。请稍后再试。"
		log.Printf("Warning: Empty response content, using fallback message")
	}

	log.Printf("Final response content length: %d", len(fullContent))

	// Convert to OpenAI API format
	openAIResp := ChatCompletionsResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     0, // Token counts not available
			CompletionTokens: 0, // Token counts not available
			TotalTokens:      0, // Token counts not available
		},
		Choices: []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
			Index        int    `json:"index"`
		}{
			{
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: fullContent,
				},
				FinishReason: "stop",
				Index:        0,
			},
		},
	}

	// 检查是否需要流式响应
	if req.Stream != nil && *req.Stream {
		// 返回流式响应
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// 发送流式数据块
		chunkResp := struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
				Index        int     `json:"index"`
			} `json:"choices"`
		}{
			ID:      openAIResp.ID,
			Object:  "chat.completion.chunk",
			Created: openAIResp.Created,
			Model:   openAIResp.Model,
			Choices: []struct {
				Delta struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
				Index        int     `json:"index"`
			}{
				{
					Delta: struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					}{
						Role:    "assistant",
						Content: fullContent,
					},
					FinishReason: nil,
					Index:        0,
				},
			},
		}

		// 发送内容块
		chunkData, _ := json.Marshal(chunkResp)
		fmt.Fprintf(w, "data: %s\n\n", string(chunkData))

		// 发送结束块
		finishReason := "stop"
		chunkResp.Choices[0].Delta.Content = ""
		chunkResp.Choices[0].Delta.Role = ""
		chunkResp.Choices[0].FinishReason = &finishReason
		finishData, _ := json.Marshal(chunkResp)
		fmt.Fprintf(w, "data: %s\n\n", string(finishData))

		// 发送usage信息（如果请求了）
		if req.StreamOptions != nil && req.StreamOptions.IncludeUsage {
			usageResp := struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Created int64  `json:"created"`
				Model   string `json:"model"`
				Usage   struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
				Choices []struct {
					Delta struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
					Index        int     `json:"index"`
				} `json:"choices"`
			}{
				ID:      openAIResp.ID,
				Object:  "chat.completion.chunk",
				Created: openAIResp.Created,
				Model:   openAIResp.Model,
				Usage:   openAIResp.Usage,
				Choices: []struct {
					Delta struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
					Index        int     `json:"index"`
				}{},
			}
			usageData, _ := json.Marshal(usageResp)
			fmt.Fprintf(w, "data: %s\n\n", string(usageData))
		}

		// 发送结束标记
		fmt.Fprintf(w, "data: [DONE]\n\n")
	} else {
		// 返回非流式响应
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResp)
	}
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 返回可用模型列表
	modelsResp := ModelsResponse{
		Object: "list",
		Data: []Model{
			{
				ID:      "qianwen",
				Object:  "model",
				Created: 1677610602,
				OwnedBy: "ulearning",
			},
			{
				ID:      "doubao",
				Object:  "model",
				Created: 1687882411,
				OwnedBy: "ulearning",
			},
			{
				ID:      "deepseek-r1",
				Object:  "model",
				Created: 1712361441,
				OwnedBy: "ulearning",
			},
			{
				ID:      "qianwen-vl",
				Object:  "model",
				Created: 1712361441,
				OwnedBy: "ulearning",
			},
			{
				ID:      "deepseek-r1-online",
				Object:  "model",
				Created: 1712361441,
				OwnedBy: "ulearning",
			},
			{
				ID:      "deepseek-r1-local",
				Object:  "model",
				Created: 1712361441,
				OwnedBy: "ulearning",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsResp)
}

// 处理历史记录请求（原格式）

// 处理OpenAI格式的历史记录请求
func handleOpenAIHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 检查Authorization header
	auth, err := getToken()
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 从Authorization header中提取token
	token := strings.TrimPrefix(auth, "Bearer ")

	// 调用kbChat API获取历史记录
	kbChatURL := "https://cloudsearchapi.ulearning.cn/kbChat/historyList?assistantId=6"
	req, err := http.NewRequest("GET", kbChatURL, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		// 返回空的历史记录
		emptyHistory := OpenAIHistoryResponse{
			Object: "list",
			Data:   []OpenAIConversation{},
		}
		json.NewEncoder(w).Encode(emptyHistory)
		return
	}

	// 设置请求头
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
	log.Printf("Calling kbChat API: %s with token: %s", kbChatURL, token)

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch history from kbChat API: %v", err)
		// 返回空的历史记录
		emptyHistory := OpenAIHistoryResponse{
			Object: "list",
			Data:   []OpenAIConversation{},
		}
		json.NewEncoder(w).Encode(emptyHistory)
		return
	}
	defer resp.Body.Close()

	log.Printf("kbChat API response status: %d", resp.StatusCode)

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response: %v", err)
		// 返回空的历史记录
		emptyHistory := OpenAIHistoryResponse{
			Object: "list",
			Data:   []OpenAIConversation{},
		}
		json.NewEncoder(w).Encode(emptyHistory)
		return
	}

	log.Printf("kbChat API response body: %s", string(body))

	// 检查API响应状态
	if resp.StatusCode != 200 {
		log.Printf("kbChat API returned non-200 status: %d, returning empty history", resp.StatusCode)
		// 返回空的历史记录
		emptyHistory := OpenAIHistoryResponse{
			Object: "list",
			Data:   []OpenAIConversation{},
		}
		json.NewEncoder(w).Encode(emptyHistory)
		return
	}

	// 解析kbChat API响应
	var historyResp HistoryResponse
	if err := json.Unmarshal(body, &historyResp); err != nil {
		log.Printf("Failed to parse history response: %v, body: %s", err, string(body))
		// 返回空的历史记录而不是错误
		emptyHistory := OpenAIHistoryResponse{
			Object: "list",
			Data:   []OpenAIConversation{},
		}
		json.NewEncoder(w).Encode(emptyHistory)
		return
	}

	// 转换为OpenAI格式
	var conversations []OpenAIConversation

	// 遍历历史记录会话
	for i, session := range historyResp.Result {
		if len(session) == 0 {
			continue
		}

		// 创建对话
		conversation := OpenAIConversation{
			ID:       fmt.Sprintf("conv_%d", i),
			Object:   "conversation",
			Created:  session[0].CreateTime / 1000, // 转换为秒
			Messages: []OpenAIMessage{},
		}

		// 转换消息格式
		for _, item := range session {
			// 添加用户消息
			conversation.Messages = append(conversation.Messages, OpenAIMessage{
				Role:    "user",
				Content: item.Query,
			})
			// 添加助手回复
			conversation.Messages = append(conversation.Messages, OpenAIMessage{
				Role:    "assistant",
				Content: item.Answer,
			})
		}

		conversations = append(conversations, conversation)
	}

	response := OpenAIHistoryResponse{
		Object: "list",
		Data:   conversations,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func startServer(port int) error {
	http.HandleFunc("/v1/chat/completions", handleChatCompletions)
	http.HandleFunc("/v1/models", handleModels)
	http.HandleFunc("/v1/chat/history", handleOpenAIHistory)
	// log所有请求
	http.Handle("/", http.StripPrefix("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		http.DefaultServeMux.ServeHTTP(w, r)
	})))

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("服务器启动在 http://localhost%s\n", addr)
	fmt.Printf("可用接口:\n")
	fmt.Printf("  POST http://localhost%s/v1/chat/completions - 聊天完成\n", addr)
	fmt.Printf("  GET  http://localhost%s/v1/models - 模型列表\n", addr)
	fmt.Printf("  GET  http://localhost%s/v1/chat/history - OpenAI格式历史记录\n", addr)
	return http.ListenAndServe(addr, nil)
}

func help() {
	fmt.Printf("使用方法: ullm <command> [arguments]\n")
	fmt.Printf("可用命令:\n")
	fmt.Printf("  serve --port PORT    启动HTTP服务器\n")
}

func main() {
	// 定义命令行参数
	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)

	// 定义serve命令的端口参数
	port := serveCmd.Int("port", 8080, "服务器端口号")

	if len(os.Args) < 2 {
		help()
		os.Exit(1)
	}
	// 解析命令
	switch os.Args[1] {
	case "serve":
		serveCmd.Parse(os.Args[2:])
		if err := startServer(*port); err != nil {
			log.Fatalf("服务器启动失败: %v", err)
		}
	default:
		help()
		os.Exit(1)
	}
}
