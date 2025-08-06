# U-LLM

U-LLM 是一个大语言模型代理服务，提供了一个OpenAI格式的API接口，用于调用大语言模型。

## 功能

- 调用大语言模型
- 支持自定义模型参数
- 支持批量调用
- 支持流式调用

## API

服务器启动在 http://localhost:8080\n
可用接口:
  - POST http://localhost:8080/v1/chat/completions - 聊天完成
  - GET  http://localhost:8080/v1/models - 模型列表
  - GET  http://localhost:8080/v1/chat/history - OpenAI格式历史记录

Cline OpenAI Compatible API
URL: http://localhost:8080/v1
apikey: [RANDOM]
model_id: [MODEL_ID]


## 支持模型
```json
[
   {
        "modelId": "qianwen",
        "modelName": "通义千问",
    },
    {
        "modelId": "doubao", 
        "modelName": "豆包",
    },
    {
        "modelId": "deepseek-r1",
        "modelName": "DeepSeekR1",
    },
    {
        "modelId": "qianwen-vl",
        "modelName": "通义千问VL",
    },
    {
        "modelId": "deepseek-r1-online",
        "modelName": "DeepSeekR1-Online",
    },
    {
        "modelId": "deepseek-r1-local",
        "modelName": "DeepSeekR1-本地版",
    }
]
```

## 贡献

欢迎提交 Issue 和 Pull Request 来帮助改进这个项目。

## 许可证

Apache 2.0
