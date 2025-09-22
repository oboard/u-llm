package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 常量定义
const (
	APIURL        = "https://cloudsearchapi.ulearning.cn/kbChat/chat"
	HistoryAPIURL = "https://cloudsearchapi.ulearning.cn/kbChat/historyList?assistantId=6"
	AssistantID   = "6"
	SessionSign   = "2"
	AskType       = "1"
	FallbackMsg   = "抱歉，我无法处理您的请求。请稍后再试。"
)

// 辅助函数：返回空历史记录
func returnEmptyHistory(w http.ResponseWriter) {
	emptyHistory := OpenAIHistoryResponse{
		Object: "list",
		Data:   []OpenAIConversation{},
	}
	json.NewEncoder(w).Encode(emptyHistory)
}

// 辅助函数：解析SSE数据
func parseSSEData(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	jsonData := strings.TrimPrefix(line, "data:")
	var streamResp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonData), &streamResp); err != nil {
		return "", false
	}
	return streamResp.Data, true
}

// 辅助函数：创建响应元数据
func createResponseMetadata() (string, int64) {
	now := time.Now().Unix()
	return fmt.Sprintf("chatcmpl-%d", now), now
}

// 通用聊天处理参数
type ChatProcessParams struct {
	Model       string
	Prompt      string
	UserAPIKey  string
	IsStream    bool
	RequestID   string
	MaxTokens   *int     // Completions API 支持
	Temperature *float64 // Completions API 支持
	Stop        []string // Completions API 支持
}

// 通用聊天处理函数 - 消除重复代码
func processChatRequest(params ChatProcessParams) (*http.Response, error) {
	// Get token for authentication
	token, err := getToken()
	if err != nil {
		return nil, fmt.Errorf("auth failed: %v", err)
	}

	// Prepare request body
	requestBody := map[string]any{
		"query":  params.Prompt,
		"images": []string{},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	// Create request to Ulearning API
	apiReq, err := http.NewRequest("POST", APIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set required headers
	apiReq.Header.Set("Authorization", token)
	apiReq.Header.Set("Content-Type", "application/json;charset=UTF-8")

	// Add query parameters
	q := apiReq.URL.Query()
	q.Add("sessionId", params.UserAPIKey)
	q.Add("assistantId", AssistantID)
	q.Add("modelId", GetModelAPIID(params.Model))
	q.Add("sessionSign", SessionSign)
	q.Add("askType", AskType)
	q.Add("requestId", strconv.FormatInt(time.Now().Unix(), 10))
	apiReq.URL.RawQuery = q.Encode()

	// Send request
	client := &http.Client{}
	resp, err := client.Do(apiReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	return resp, nil
}

var stopSignal = func() *string {
	finishReason := "stop"
	return &finishReason
}()

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	// 生成请求ID用于追踪
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())

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

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[%s] ERROR: Failed to read request body: %v", requestID, err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 解析请求体
	var req ChatCompletionsRequest
	if json.Unmarshal(body, &req) != nil {
		log.Printf("[%s] ERROR: Invalid JSON payload: %v", requestID, err)
		http.Error(w, fmt.Sprintf("Invalid request payload: %v", err), http.StatusBadRequest)
		return
	}

	// 关键信息日志 - 一行搞定
	log.Printf("[%s] model=%s msgs=%d stream=%v user=%.8s",
		requestID, req.Model, len(req.Messages),
		req.Stream != nil && *req.Stream, userApiKey)

	// 调用 Ulearning API
	// Get token for authentication
	token, err := getToken()
	if err != nil {
		log.Printf("[%s] ERROR: Auth failed: %v", requestID, err)
		http.Error(w, fmt.Sprintf("Failed to get token: %v", err), http.StatusInternalServerError)
		return
	}

	// 构建完整的prompt，包括系统提示词和用户消息
	var prompt strings.Builder
	var hasSystemMessage bool

	// 处理系统消息
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemContent := extractTextContent(msg.Content)
			if systemContent != "" {
				prompt.WriteString("System: ")
				prompt.WriteString(systemContent)
				prompt.WriteString("\n\n")
				hasSystemMessage = true
			}
		}
	}

	// 处理用户消息（取最后一条用户消息）
	var userContent string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userContent = extractTextContent(req.Messages[i].Content)
			break
		}
	}

	var finalPrompt string
	if hasSystemMessage && userContent != "" {
		// 有系统消息时，添加前缀
		prompt.WriteString("User: ")
		prompt.WriteString(userContent)
		finalPrompt = prompt.String()
	} else if userContent != "" {
		// 没有系统消息时，直接使用用户内容
		finalPrompt = userContent
	} else {
		// 如果没有找到有效内容，使用最后一条消息作为fallback
		lastMessage := req.Messages[len(req.Messages)-1]
		finalPrompt = extractTextContent(lastMessage.Content)
	}

	// Prepare request body
	requestBody := map[string]any{
		"query":  finalPrompt,
		"images": []string{},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, "Failed to marshal request body", http.StatusInternalServerError)
		return
	}

	// Create request to Ulearning API
	apiReq, err := http.NewRequest("POST", APIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Set required headers
	apiReq.Header.Set("Authorization", token)
	apiReq.Header.Set("Content-Type", "application/json;charset=UTF-8")

	// Add query parameters
	q := apiReq.URL.Query()
	q.Add("sessionId", userApiKey) // 使用用户的API key作为sessionId
	q.Add("assistantId", AssistantID)
	q.Add("modelId", GetModelAPIID(req.Model))
	q.Add("sessionSign", SessionSign)
	q.Add("askType", AskType)
	q.Add("requestId", strconv.FormatInt(time.Now().Unix(), 10))
	apiReq.URL.RawQuery = q.Encode()

	// Send request
	client := &http.Client{}
	resp, err := client.Do(apiReq)
	if err != nil {
		log.Printf("[%s] ERROR: API request failed: %v", requestID, err)
		http.Error(w, fmt.Sprintf("Failed to send request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("[%s] ERROR: API status %d", requestID, resp.StatusCode)
		http.Error(w, fmt.Sprintf("API request failed with status: %d", resp.StatusCode), resp.StatusCode)
		return
	}

	// 检查是否为流式请求
	if req.Stream != nil && *req.Stream {
		// 流式响应：实时转发数据
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// 生成响应ID和时间戳
		responseID, createdTime := createResponseMetadata()

		// 实时转发流式数据
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if data, ok := parseSSEData(scanner.Text()); ok && data != "" {
				// 转换为OpenAI格式并立即发送
				chunkResp := ChatCompletionChunk{
					ID:      responseID,
					Object:  "chat.completion.chunk",
					Created: createdTime,
					Model:   req.Model,
					Choices: []StreamChoice{{
						Delta:        Delta{Content: data},
						FinishReason: nil, // 中间消息的finish_reason为null
						Index:        0,
					}},
				}

				chunkData, _ := json.Marshal(chunkResp)
				fmt.Fprintf(w, "data: %s\n\n", string(chunkData))
				flusher.Flush()
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[%s] ERROR: Stream read failed: %v", requestID, err)
		}

		// 发送结束标记
		finishReason := "stop"
		finishResp := ChatCompletionChunk{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: createdTime,
			Model:   req.Model,
			Choices: []StreamChoice{
				{
					Delta:        Delta{},
					Index:        0,
					FinishReason: &finishReason,
				},
			},
		}

		finishData, _ := json.Marshal(finishResp)
		fmt.Fprintf(w, "data: %s\n\n", string(finishData))
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()

		log.Printf("[%s] SUCCESS: stream completed", requestID)
	} else {
		// 非流式响应：收集完整内容
		scanner := bufio.NewScanner(resp.Body)
		var fullContent string
		for scanner.Scan() {
			if data, ok := parseSSEData(scanner.Text()); ok {
				fullContent += data
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[%s] ERROR: Response stream failed: %v", requestID, err)
			http.Error(w, "Failed to read response stream", http.StatusInternalServerError)
			return
		}

		// 检查并处理空内容
		if strings.TrimSpace(fullContent) == "" {
			fullContent = FallbackMsg
			log.Printf("[%s] WARN: Empty response, using fallback", requestID)
		}

		// Convert to OpenAI API format
		openAIResp := ChatCompletionsResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Usage: Usage{
				PromptTokens:     0, // Token counts not available
				CompletionTokens: 0, // Token counts not available
				TotalTokens:      0, // Token counts not available
			},
			Choices: []Choice{
				{
					Message: Message{
						Role:    "assistant",
						Content: fullContent,
					},
					FinishReason: stopSignal,
					Index:        0,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openAIResp)

		log.Printf("[%s] SUCCESS: response_len=%d", requestID, len(fullContent))
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
		Data:   GetAvailableModels(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsResp)
}

func handleResponses(w http.ResponseWriter, r *http.Request) {
	// 生成请求ID用于追踪
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())

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
	if userApiKey == auth {
		userApiKey = auth
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[%s] ERROR: Failed to read request body: %v", requestID, err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 解析请求体
	var req ResponsesRequest
	if json.Unmarshal(body, &req) != nil {
		log.Printf("[%s] ERROR: Invalid JSON payload: %v", requestID, err)
		http.Error(w, fmt.Sprintf("Invalid request payload: %v", err), http.StatusBadRequest)
		return
	}

	// 提取prompt内容
	var prompt string
	switch input := req.Input.(type) {
	case string:
		// 简单文本输入
		prompt = input
	case []interface{}:
		// 多模态输入 - 目前只处理文本部分
		for _, item := range input {
			if msgMap, ok := item.(map[string]interface{}); ok {
				if role, exists := msgMap["role"]; exists && role == "user" {
					if content, exists := msgMap["content"]; exists {
						if contentArray, ok := content.([]interface{}); ok {
							for _, contentItem := range contentArray {
								if contentMap, ok := contentItem.(map[string]interface{}); ok {
									if contentType, exists := contentMap["type"]; exists && contentType == "input_text" {
										if text, exists := contentMap["text"]; exists {
											if textStr, ok := text.(string); ok {
												prompt = textStr
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
	default:
		prompt = fmt.Sprintf("%v", input)
	}

	if prompt == "" {
		http.Error(w, "No valid input found", http.StatusBadRequest)
		return
	}

	// 关键信息日志
	log.Printf("[%s] model=%s prompt_len=%d stream=%v user=%.8s",
		requestID, req.Model, len(prompt),
		req.Stream != nil && *req.Stream, userApiKey)

	// 使用通用处理函数
	params := ChatProcessParams{
		Model:      req.Model,
		Prompt:     prompt,
		UserAPIKey: userApiKey,
		IsStream:   req.Stream != nil && *req.Stream,
		RequestID:  requestID,
	}

	resp, err := processChatRequest(params)
	if err != nil {
		log.Printf("[%s] ERROR: %v", requestID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 生成响应ID和时间戳
	responseID, createdTime := createResponseMetadata()

	// 检查是否为流式请求
	if req.Stream != nil && *req.Stream {
		// 流式响应
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// 实时转发流式数据
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			if data, ok := parseSSEData(scanner.Text()); ok && data != "" {
				// 转换为统一的 delta 格式
				chunkResp := UnifiedStreamChunk{
					ID:    responseID,
					Type:  "response.output_text.delta",
					Delta: data,
				}

				chunkData, _ := json.Marshal(chunkResp)
				fmt.Fprintf(w, "data: %s\n\n", string(chunkData))
				flusher.Flush()
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[%s] ERROR: Stream read failed: %v", requestID, err)
		}

		// 发送完成标记
		completedResp := UnifiedStreamChunk{
			ID:   responseID,
			Type: "response.completed",
		}
		completedData, _ := json.Marshal(completedResp)
		fmt.Fprintf(w, "data: %s\n\n", string(completedData))

		// 发送结束标记
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()

		log.Printf("[%s] SUCCESS: stream completed", requestID)
	} else {
		// 非流式响应
		scanner := bufio.NewScanner(resp.Body)
		var fullContent string
		for scanner.Scan() {
			if data, ok := parseSSEData(scanner.Text()); ok {
				fullContent += data
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("[%s] ERROR: Response stream failed: %v", requestID, err)
			http.Error(w, "Failed to read response stream", http.StatusInternalServerError)
			return
		}

		// 检查并处理空内容
		if strings.TrimSpace(fullContent) == "" {
			fullContent = FallbackMsg
			log.Printf("[%s] WARN: Empty response, using fallback", requestID)
		}

		// 转换为 Responses API 格式
		responsesResp := ResponsesResponse{
			ID:      responseID,
			Object:  "response",
			Created: createdTime,
			Model:   req.Model,
			Output: []ResponsesOutputMessage{{
				ID:   "msg_1",
				Type: "message",
				Role: "assistant",
				Content: []ResponsesOutputContent{{
					Type: "output_text",
					Text: fullContent,
				}},
			}},
			Usage: Usage{
				PromptTokens:     0, // Token counts not available
				CompletionTokens: 0, // Token counts not available
				TotalTokens:      0, // Token counts not available
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responsesResp)

		log.Printf("[%s] SUCCESS: response_len=%d", requestID, len(fullContent))
	}
}

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
	req, err := http.NewRequest("GET", HistoryAPIURL, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		returnEmptyHistory(w)
		return
	}

	// 设置请求头
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
	log.Printf("Calling kbChat API: %s with token: %s", HistoryAPIURL, token)

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch history from kbChat API: %v", err)
		returnEmptyHistory(w)
		return
	}
	defer resp.Body.Close()

	log.Printf("kbChat API response status: %d", resp.StatusCode)

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response: %v", err)
		returnEmptyHistory(w)
		return
	}

	log.Printf("kbChat API response body: %s", string(body))

	// 检查API响应状态
	if resp.StatusCode != 200 {
		log.Printf("kbChat API returned non-200 status: %d, returning empty history", resp.StatusCode)
		returnEmptyHistory(w)
		return
	}

	// 解析kbChat API响应
	var historyResp HistoryResponse
	if err := json.Unmarshal(body, &historyResp); err != nil {
		log.Printf("Failed to parse history response: %v, body: %s", err, string(body))
		returnEmptyHistory(w)
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
	http.HandleFunc("/v1/responses", handleResponses)
	// log所有请求
	http.Handle("/", http.StripPrefix("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		http.DefaultServeMux.ServeHTTP(w, r)
	})))

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("服务器启动在 http://0.0.0.0%s\n", addr)
	fmt.Printf("可用接口:\n")
	fmt.Printf("  POST http://0.0.0.0%s/v1/chat/completions - 聊天完成\n", addr)
	fmt.Printf("  POST http://0.0.0.0%s/v1/responses - OpenAI统一响应接口\n", addr)
	fmt.Printf("  GET  http://0.0.0.0%s/v1/models - 模型列表\n", addr)
	fmt.Printf("  GET  http://0.0.0.0%s/v1/chat/history - OpenAI格式历史记录\n", addr)
	return http.ListenAndServe(addr, nil)
}
