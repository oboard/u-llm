# 使用官方 Go 镜像作为构建环境
FROM golang:1.24.1-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制 go.mod 和 go.sum 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -o udrive-cli .

# 使用轻量级的 alpine 镜像作为运行环境
FROM alpine:latest

# 安装 ca-certificates
RUN apk --no-cache add ca-certificates

WORKDIR /app

# 从构建阶段复制编译好的应用
COPY --from=builder /app/udrive-cli .

# 暴露端口（如果需要运行 HTTP 服务器）
EXPOSE 2345

# 设置入口点
ENTRYPOINT ["./udrive-cli"] 