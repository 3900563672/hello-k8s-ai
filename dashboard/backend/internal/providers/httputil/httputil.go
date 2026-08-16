package httputil

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NewClient 构造带超时和代理的 HTTP 客户端，供各 Provider 复用。
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: timeout,
		},
	}
}

// ParseBaseURL 解析并校验 Provider 的基础 URL。
func ParseBaseURL(rawURL, providerName string) (*url.URL, error) {
	baseURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse %s URL: %w", providerName, err)
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("%s URL must use http or https", providerName)
	}
	return baseURL, nil
}

// Resolve 把相对路径拼到基础 URL 上，返回新的 URL。
func Resolve(base *url.URL, path string) *url.URL {
	result := *base
	result.Path = strings.TrimRight(result.Path, "/") + path
	result.RawQuery = ""
	return &result
}

// GetJSON 发起 GET 请求并把 JSON 响应解码到 target。
// providerName 只用于错误信息，maxBodyBytes 限制响应体大小。
func GetJSON(
	ctx context.Context,
	client *http.Client,
	endpoint *url.URL,
	target any,
	providerName string,
	maxBodyBytes int64,
) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("create %s request: %w", providerName, err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("query %s: %w", providerName, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("%s returned HTTP %d: %s", providerName, response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxBodyBytes)).Decode(target); err != nil {
		return fmt.Errorf("decode %s response: %w", providerName, err)
	}
	return nil
}
