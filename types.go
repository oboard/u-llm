package main

import "time"

// 认证相关结构体
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

// 聊天完成相关结构体
type ChatCompletionsRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	} `json:"messages"`
	Temperature   *float64 `json:"temperature,omitempty"`
	Stream        *bool    `json:"stream,omitempty"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage,omitempty"`
	} `json:"stream_options,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason *string `json:"finish_reason"`
	Index        int     `json:"index"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatCompletionsResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Usage   Usage    `json:"usage"`
	Choices []Choice `json:"choices"`
}

// 流式响应相关结构体
type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type StreamChoice struct {
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
	Index        int     `json:"index"`
}

type ChatCompletionChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// 模型相关结构体
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelConfig 模型配置信息
type ModelConfig struct {
	ID      string
	APIID   string // API中使用的modelId
	Object  string
	Created int64
	OwnedBy string
}

// 统一的模型配置
var ModelConfigs = []ModelConfig{
	{
		ID:      "qwen",
		APIID:   "1",
		Object:  "model",
		Created: 1677610602,
		OwnedBy: "ulearning",
	},
	{
		ID:      "doubao",
		APIID:   "2",
		Object:  "model",
		Created: 1687882411,
		OwnedBy: "ulearning",
	},
	{
		ID:      "deepseek-r1",
		APIID:   "3",
		Object:  "model",
		Created: 1712361441,
		OwnedBy: "ulearning",
	},
	{
		ID:      "qwen2.5-vl-7b",
		APIID:   "4",
		Object:  "model",
		Created: 1712361441,
		OwnedBy: "ulearning",
	},
	{
		ID:      "deepseek-r1-local",
		APIID:   "6",
		Object:  "model",
		Created: 1712361441,
		OwnedBy: "ulearning",
	},
	{
		ID:      "deepseek-v3.1",
		APIID:   "7",
		Object:  "model",
		Created: 1712361441,
		OwnedBy: "ulearning",
	},
}

// GetModelAPIID 根据模型ID获取API中使用的modelId
func GetModelAPIID(modelID string) string {
	for _, config := range ModelConfigs {
		if config.ID == modelID {
			return config.APIID
		}
	}
	return "2" // 默认使用豆包
}

// GetAvailableModels 获取可用模型列表
func GetAvailableModels() []Model {
	models := make([]Model, len(ModelConfigs))
	for i, config := range ModelConfigs {
		models[i] = Model{
			ID:      config.ID,
			Object:  config.Object,
			Created: config.Created,
			OwnedBy: config.OwnedBy,
		}
	}
	return models
}

type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// 历史记录相关结构体
type HistoryItem struct {
	Query       string `json:"query"`
	CustomTitle string `json:"customTitle"`
	Answer      string `json:"answer"`
	Type        int    `json:"type"`
	Infos       []any  `json:"infos"`
	UserID      int64  `json:"userId"`
	SessionID   string `json:"sessionId"`
	RequestID   string `json:"requestId"`
	CreateTime  int64  `json:"createTime"`
	AssistantID int    `json:"assistantId"`
	RoleID      int    `json:"roleId"`
	SessionSign int    `json:"sessionSign"`
	References  []any  `json:"references"`
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

// OpenAI Responses API 数据结构
type ResponsesInputContent struct {
	Type     string `json:"type"`                // "input_text" 或 "input_image"
	Text     string `json:"text,omitempty"`      // 文本内容
	ImageURL string `json:"image_url,omitempty"` // 图片URL
}

type ResponsesInputMessage struct {
	Role    string                  `json:"role"`
	Content []ResponsesInputContent `json:"content"`
}

type ResponsesRequest struct {
	Model  string      `json:"model"`
	Input  interface{} `json:"input"` // 可以是字符串或 []ResponsesInputMessage
	Stream *bool       `json:"stream,omitempty"`
}

type ResponsesOutputContent struct {
	Type string `json:"type"` // "output_text"
	Text string `json:"text"`
}

type ResponsesOutputMessage struct {
	ID      string                   `json:"id"`
	Type    string                   `json:"type"` // "message"
	Role    string                   `json:"role"` // "assistant"
	Content []ResponsesOutputContent `json:"content"`
}

type ResponsesResponse struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"` // "response"
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Output  []ResponsesOutputMessage `json:"output"`
	Usage   Usage                    `json:"usage"`
}

// 流式响应结构
type ResponsesStreamDelta struct {
	Content []ResponsesOutputContent `json:"content,omitempty"`
}

type ResponsesStreamChunk struct {
	ID    string               `json:"id"`
	Type  string               `json:"type"` // "message_delta"
	Delta ResponsesStreamDelta `json:"delta"`
}

// Completions API 相关结构体
type CompletionsRequest struct {
	Model         string   `json:"model"`
	Prompt        string   `json:"prompt"`
	MaxTokens     *int     `json:"max_tokens,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	Stop          []string `json:"stop,omitempty"`
	Stream        *bool    `json:"stream,omitempty"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage,omitempty"`
	} `json:"stream_options,omitempty"`
}

type CompletionsChoice struct {
	Text         string  `json:"text"`
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason"`
}

type CompletionsResponse struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []CompletionsChoice `json:"choices"`
	Usage   Usage               `json:"usage"`
}

// Completions 流式响应结构体
type CompletionsStreamChoice struct {
	Text         string  `json:"text"`
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason"`
}

type CompletionsStreamChunk struct {
	ID      string                    `json:"id"`
	Object  string                    `json:"object"`
	Created int64                     `json:"created"`
	Model   string                    `json:"model"`
	Choices []CompletionsStreamChoice `json:"choices"`
}

// 统一的流式响应格式
type UnifiedStreamChunk struct {
	ID    string `json:"id"`
	Type  string `json:"type"`            // "response.output_text.delta" 或 "response.completed"
	Delta string `json:"delta,omitempty"` // 增量文本内容，仅在 delta 类型时存在
}
