# U-Drive CLI

U-Drive CLI 是一个命令行工具，用于管理平台的文件上传和删除操作。

## 功能特点

- 文件上传：支持将本地文件上传到平台
  - 实时显示上传进度条
  - 显示上传速度和文件大小
  - 美观的进度条动画效果
- 文件删除：支持删除已上传的文件
- 自动认证：自动处理登录认证和 token 管理
- 缓存支持：自动缓存认证 token，提高使用效率

## 安装

### 从源码安装

1. 确保已安装 Go 环境（推荐 Go 1.16 或更高版本）
2. 克隆仓库：
   ```bash
   git clone https://github.com/yourusername/u-drive-cli.git
   cd u-drive-cli
   ```
3. 编译安装：
   ```bash
   go build -o udrive
   ```
4. 将可执行文件移动到系统路径：
   ```bash
   sudo mv udrive /usr/local/bin/
   ```

## 使用方法

### 查看帮助

```bash
udrive -h
# 或
udrive --help
```

### 上传文件

```bash
udrive upload <filename>
```

例如：
```bash
udrive upload test.jpg
```

上传时会显示实时进度条：
```
正在上传文件... [===============>----------------] 45% | 4.5MB/10MB | 2.3MB/s
```

### 删除文件

```bash
udrive delete <filename>
```

例如：
```bash
udrive delete test.jpg
```

## 注意事项

1. 首次使用时需要登录认证，认证信息会被缓存
2. 文件上传后会自动生成可访问的 URL
3. 删除操作不可逆，请谨慎操作
4. 支持的文件类型取决于平台的限制
5. 上传大文件时会显示实时进度条，方便监控上传状态

## 错误处理

如果遇到错误，程序会显示详细的错误信息，包括：
- 认证失败
- 文件操作失败
- 网络连接问题
- 其他系统错误

## 贡献

欢迎提交 Issue 和 Pull Request 来帮助改进这个项目。

## 许可证

MIT License 