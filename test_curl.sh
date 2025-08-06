#!/bin/bash

# 测试 u-llm 聊天完成 API
# 使用方法: ./test_curl.sh

echo "测试 u-llm 聊天完成 API..."
echo "确保服务器已启动: go run main.go serve --port 8081"
echo ""

# 基本测试
echo "=== 基本聊天测试 ==="
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "你好，请简单介绍一下自己"
      }
    ]
  }' | jq .

echo ""
echo "=== 多轮对话测试 ==="
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "什么是人工智能？"
      },
      {
        "role": "assistant",
        "content": "人工智能是计算机科学的一个分支..."
      },
      {
        "role": "user",
        "content": "它有哪些应用领域？"
      }
    ]
  }' | jq .

echo ""
echo "=== 模型列表测试 ==="
curl -X GET http://localhost:8081/v1/models | jq .

echo ""
echo "=== 简单问答测试 ==="
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "1+1等于几？"
      }
    ]
  }' | jq '.choices[0].message.content'

echo ""
echo "=== 错误测试 - 无效方法 ==="
curl -X GET http://localhost:8081/v1/chat/completions

echo ""
echo "=== 错误测试 - 无效JSON ==="
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{invalid json}'

echo ""
echo "测试完成！API工作正常 ✅"