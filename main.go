package main

import (
	"flag"
	"log"
	"os"
)

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
