package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

type ObsToken struct {
	AK            string `json:"ak"`
	SK            string `json:"sk"`
	SecurityToken string `json:"securitytoken"`
	Bucket        string `json:"bucket"`
	Endpoint      string `json:"endpoint"`
	Domain        string `json:"domain"`
}

type ApiResponse struct {
	Result ObsToken `json:"result"`
}

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
	loginReq := LoginRequest{
		LoginName:  "oboard7@teml.net",
		Password:   "45977E064B593F32120B82C095BF5423",
		Device:     "HUAWEI-HELL",
		AppVersion: "36",
		WebEnv:     "1",
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://apps.ulearning.cn/login", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("uversion", "2")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return "", err
	}

	// 缓存token，设置1小时过期
	cache := TokenCache{
		Token:      loginResp.Token,
		ExpireTime: time.Now().Add(time.Hour),
	}

	cacheData, err := json.Marshal(cache)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile("cache.json", cacheData, 0644); err != nil {
		return "", err
	}

	return loginResp.Token, nil
}

func getUploadToken(filename, token string) (*ObsToken, error) {
	url := fmt.Sprintf("https://courseapi.ulearning.cn/obs/uploadToken?path=resources/web/%s", filename)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp ApiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	return &apiResp.Result, nil
}

func uploadFile(filename string) error {
	// 获取token
	token, err := getToken()
	if err != nil {
		return fmt.Errorf("获取token失败: %v", err)
	}

	// 获取上传凭证
	obsToken, err := getUploadToken(filename, token)
	if err != nil {
		return fmt.Errorf("获取上传凭证失败: %v", err)
	}

	// 创建OBS客户端
	obsClient, err := obs.New(obsToken.AK, obsToken.SK, obsToken.Endpoint, obs.WithSecurityToken(obsToken.SecurityToken))
	if err != nil {
		return fmt.Errorf("创建OBS客户端失败: %v", err)
	}

	// 打开要上传的文件
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 创建上传输入
	input := &obs.PutObjectInput{}
	input.Bucket = obsToken.Bucket
	input.Key = fmt.Sprintf("resources/web/%s", filepath.Base(filename))
	input.Body = file

	// 执行上传
	_, err = obsClient.PutObject(input)
	if err != nil {
		return fmt.Errorf("上传文件失败: %v", err)
	}

	// 构建文件访问地址
	fileUrl := fmt.Sprintf("%s/%s", obsToken.Domain, input.Key)
	fmt.Printf("文件上传成功！\n")
	fmt.Printf("文件URL: %s\n", fileUrl)
	return nil
}

func deleteFile(filename string) error {
	// 获取token
	token, err := getToken()
	if err != nil {
		return fmt.Errorf("获取token失败: %v", err)
	}

	// 获取上传凭证
	obsToken, err := getUploadToken(filename, token)
	if err != nil {
		return fmt.Errorf("获取上传凭证失败: %v", err)
	}

	// 创建OBS客户端
	obsClient, err := obs.New(obsToken.AK, obsToken.SK, obsToken.Endpoint, obs.WithSecurityToken(obsToken.SecurityToken))
	if err != nil {
		return fmt.Errorf("创建OBS客户端失败: %v", err)
	}

	// 创建空文件内容
	emptyContent := bytes.NewReader([]byte{})

	// 创建上传输入
	input := &obs.PutObjectInput{}
	input.Bucket = obsToken.Bucket
	input.Key = fmt.Sprintf("resources/web/%s", filepath.Base(filename))
	input.Body = emptyContent

	// 执行上传
	_, err = obsClient.PutObject(input)
	if err != nil {
		return fmt.Errorf("上传空文件失败: %v", err)
	}

	fmt.Printf("文件已清空！\n")
	return nil
}

func main() {
	// 定义命令行参数
	uploadCmd := flag.NewFlagSet("upload", flag.ExitOnError)
	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)

	// 检查命令行参数
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Println("使用方法: udrive <command> [arguments]")
		fmt.Println("可用命令:")
		fmt.Println("  upload <filename>    上传文件")
		fmt.Println("  delete <filename>    删除文件")
		fmt.Println("\n选项:")
		fmt.Println("  -h, --help           显示帮助信息")
		os.Exit(1)
	}

	// 解析命令
	switch os.Args[1] {
	case "upload":
		uploadCmd.Parse(os.Args[2:])
		if uploadCmd.NArg() != 1 {
			fmt.Println("错误: 请指定要上传的文件名")
			fmt.Println("使用方法: udrive upload <filename>")
			os.Exit(1)
		}
		filename := uploadCmd.Arg(0)
		if err := uploadFile(filename); err != nil {
			fmt.Printf("错误: %v\n", err)
			os.Exit(1)
		}
	case "delete":
		deleteCmd.Parse(os.Args[2:])
		if deleteCmd.NArg() != 1 {
			fmt.Println("错误: 请指定要删除的文件名")
			fmt.Println("使用方法: udrive delete <filename>")
			os.Exit(1)
		}
		filename := deleteCmd.Arg(0)
		if err := deleteFile(filename); err != nil {
			fmt.Printf("错误: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("未知命令: %s\n", os.Args[1])
		fmt.Println("使用方法: udrive <command> [arguments]")
		fmt.Println("可用命令:")
		fmt.Println("  upload <filename>    上传文件")
		fmt.Println("  delete <filename>    删除文件")
		os.Exit(1)
	}
}
