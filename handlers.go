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
	
	log.Printf("Final prompt: %s", finalPrompt)

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
		http.Error(w, fmt.Sprintf("Failed to send request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
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
						Delta: Delta{Content: data},
						Index: 0,
					}},
				}

				chunkData, _ := json.Marshal(chunkResp)
				fmt.Fprintf(w, "data: %s\n\n", string(chunkData))
				flusher.Flush()
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Error reading stream: %v", err)
		}

		// 发送结束标记
		finishResp := ChatCompletionChunk{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: createdTime,
			Model:   req.Model,
			Choices: []StreamChoice{
				{
					Delta:        Delta{},
					Index:        0,
					FinishReason: "stop",
				},
			},
		}

		finishData, _ := json.Marshal(finishResp)
		fmt.Fprintf(w, "data: %s\n\n", string(finishData))
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
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
			http.Error(w, "Failed to read response stream", http.StatusInternalServerError)
			return
		}

		// 检查并处理空内容
		if strings.TrimSpace(fullContent) == "" {
			fullContent = FallbackMsg
			log.Printf("Warning: Empty response content, using fallback message")
		}

		log.Printf("Final response content length: %d", len(fullContent))

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
					FinishReason: "stop",
					Index:        0,
				},
			},
		}

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
		Data:   GetAvailableModels(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modelsResp)
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
