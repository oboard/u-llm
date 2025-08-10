package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

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