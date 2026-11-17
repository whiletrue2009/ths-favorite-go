package thsclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 是所有数据接口共享的 HTTP 层。
// Cookie 以显式 map 保存并按请求注入，不经 http.CookieJar——
// 同一批 Cookie 要原样发给多个不同子域，手动管理最可控。
type Client struct {
	http    *http.Client
	cookies map[string]string
}

// NewClient 创建默认超时 10 秒的客户端。
func NewClient() *Client {
	return &Client{
		http:    &http.Client{Timeout: 10 * time.Second},
		cookies: map[string]string{},
	}
}

// Cookies 返回当前 Cookie 副本。
func (c *Client) Cookies() map[string]string {
	out := make(map[string]string, len(c.cookies))
	for k, v := range c.cookies {
		out[k] = v
	}
	return out
}

// SetCookies 整体替换 Cookie。
func (c *Client) SetCookies(cookies map[string]string) {
	c.cookies = make(map[string]string, len(cookies))
	for k, v := range cookies {
		c.cookies[k] = v
	}
}

// cookieHeader 把 map 拼成请求头格式。
func cookieHeader(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for k, v := range cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

// do 执行请求并返回响应体字节。
func (c *Client) do(ctx context.Context, action string, req *http.Request) ([]byte, error) {
	req = req.WithContext(ctx)
	if len(c.cookies) > 0 {
		req.Header.Set("Cookie", cookieHeader(c.cookies))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &NetError{Action: action, Detail: "请求超时或被取消: " + ctx.Err().Error(), Timeout: true}
		}
		return nil, &NetError{Action: action, Detail: err.Error()}
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, &NetError{Action: action, Detail: "读取响应失败: " + readErr.Error()}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, &NetError{Action: action, Detail: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, preview)}
	}
	return body, nil
}

// Get 发起 GET 请求，query 为 nil 时不带查询串。
func (c *Client) Get(ctx context.Context, action, rawURL string, query url.Values, extraHeaders map[string]string) ([]byte, error) {
	if query != nil {
		separator := "?"
		if strings.Contains(rawURL, "?") {
			separator = "&"
		}
		rawURL += separator + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &NetError{Action: action, Detail: "构造请求失败: " + err.Error()}
	}
	req.Header.Set("User-Agent", MobileUA)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	return c.do(ctx, action, req)
}

// PostForm 发起 application/x-www-form-urlencoded POST。
func (c *Client) PostForm(ctx context.Context, action, rawURL string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &NetError{Action: action, Detail: "构造请求失败: " + err.Error()}
	}
	req.Header.Set("User-Agent", MobileUA)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(ctx, action, req)
}

// PostJSON 发起 JSON POST（分享分组接口使用）。
func (c *Client) PostJSON(ctx context.Context, action, rawURL string, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(payload))
	if err != nil {
		return nil, &NetError{Action: action, Detail: "构造请求失败: " + err.Error()}
	}
	req.Header.Set("User-Agent", MobileUA)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return c.do(ctx, action, req)
}

// PostMultipart 发起 multipart POST（multiStorage upload 使用）。
// fields 为普通表单字段；文件字段名固定为 uploadFile。
func (c *Client) PostMultipart(ctx context.Context, action, rawURL string, fields url.Values, fileField, filename string, fileBytes []byte, contentType string) ([]byte, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, values := range fields {
		for _, v := range values {
			if err := writer.WriteField(k, v); err != nil {
				return nil, &NetError{Action: action, Detail: "构造 multipart 失败: " + err.Error()}
			}
		}
	}
	part, err := writer.CreateFormFile(fileField, filename)
	if err != nil {
		return nil, &NetError{Action: action, Detail: "构造文件字段失败: " + err.Error()}
	}
	if _, err := part.Write(fileBytes); err != nil {
		return nil, &NetError{Action: action, Detail: "写入文件字段失败: " + err.Error()}
	}
	if err := writer.Close(); err != nil {
		return nil, &NetError{Action: action, Detail: "关闭 multipart 失败: " + err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, &body)
	if err != nil {
		return nil, &NetError{Action: action, Detail: "构造请求失败: " + err.Error()}
	}
	req.Header.Set("User-Agent", MobileUA)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return c.do(ctx, action, req)
}

// SetTimeout 覆盖默认超时。
func (c *Client) SetTimeout(d time.Duration) {
	c.http.Timeout = d
}
