package crop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://qyapi.weixin.qq.com/cgi-bin"

var defaultHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Err 微信返回错误
type Err struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// AccessToken 微信企业号请求Token
type AccessToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Err
	ExpiresInTime time.Time
}

// Client 微信企业号应用配置信息
type Client struct {
	CropID      string
	AgentID     int
	AgentSecret string
	Token       AccessToken

	httpClient *http.Client
	baseURL    string
	mu         sync.Mutex
}

// Result 发送消息返回结果
type Result struct {
	Err
	InvalidUser  string `json:"invaliduser"`
	InvalidParty string `json:"invalidparty"`
	InvalidTag   string `json:"invalidtag"`
}

// Content 文本消息内容
type Content struct {
	Content string `json:"content"`
}

// Message 消息主体参数
type Message struct {
	ToUser  string  `json:"touser"`
	ToParty string  `json:"toparty"`
	ToTag   string  `json:"totag"`
	MsgType string  `json:"msgtype"`
	AgentID int     `json:"agentid"`
	Text    Content `json:"text"`
}

// New 实例化微信企业号应用
func New(cropID string, agentID int, AgentSecret string) *Client {
	return &Client{
		CropID:      cropID,
		AgentID:     agentID,
		AgentSecret: AgentSecret,
		httpClient:  defaultHTTPClient,
		baseURL:     defaultBaseURL,
	}
}

// Send 发送信息
func (c *Client) Send(msg Message) error {
	accessToken, err := c.GetAccessToken()
	if err != nil {
		return fmt.Errorf("获取token失败: %w", err)
	}

	msg.AgentID = c.AgentID
	if msg.MsgType == "" {
		msg.MsgType = "text"
	}

	endpoint := c.endpoint("message/send", url.Values{"access_token": {accessToken}})
	resultByte, err := c.postJSON(endpoint, msg)
	if err != nil {
		return fmt.Errorf("请求微信接口失败: %w", err)
	}

	result := Result{}
	err = json.Unmarshal(resultByte, &result)
	if err != nil {
		return fmt.Errorf("解析微信接口返回数据失败: %w", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("发送消息失败: %s", result.ErrMsg)
	}

	if result.InvalidUser != "" || result.InvalidTag != "" || result.InvalidParty != "" {
		return fmt.Errorf("消息发送成功, 但是有部分目标无法送达: %s%s%s", result.InvalidUser, result.InvalidParty, result.InvalidTag)
	}

	return nil
}

// GetAccessToken 获取回话token
func (c *Client) GetAccessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Token.AccessToken != "" && time.Now().Before(c.Token.ExpiresInTime) {
		return c.Token.AccessToken, nil
	}

	token, err := c.getAccessTokenFromWeixin()
	if err != nil {
		return "", err
	}
	token.ExpiresInTime = time.Now().Add(tokenRefreshDuration(token.ExpiresIn))
	c.Token = token
	return c.Token.AccessToken, nil
}

// 从微信服务器获取token
func (c *Client) getAccessTokenFromWeixin() (AccessToken, error) {
	endpoint := c.endpoint("gettoken", url.Values{
		"corpid":     {c.CropID},
		"corpsecret": {c.AgentSecret},
	})

	result, err := c.http().Get(endpoint)
	if err != nil {
		return AccessToken{}, err
	}
	defer result.Body.Close()

	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return AccessToken{}, fmt.Errorf("微信接口HTTP状态异常: %s", result.Status)
	}

	res, err := io.ReadAll(result.Body)
	if err != nil {
		return AccessToken{}, err
	}

	tokenSession := AccessToken{}
	err = json.Unmarshal(res, &tokenSession)
	if err != nil {
		return AccessToken{}, err
	}

	if tokenSession.ExpiresIn == 0 || tokenSession.AccessToken == "" {
		return AccessToken{}, fmt.Errorf("获取微信错误代码: %v, 错误信息: %v", tokenSession.ErrCode, tokenSession.ErrMsg)
	}

	return tokenSession, nil
}

// JSONPost Post请求json数据
func JSONPost(url string, data interface{}) ([]byte, error) {
	return postJSON(defaultHTTPClient, url, data)
}

func (c *Client) postJSON(url string, data interface{}) ([]byte, error) {
	return postJSON(c.http(), url, data)
}

func postJSON(client *http.Client, url string, data interface{}) ([]byte, error) {
	jsonBody, err := encodeJSON(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	r, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode < http.StatusOK || r.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP状态异常: %s", r.Status)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return body, err
}

func encodeJSON(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func tokenRefreshDuration(expiresIn int) time.Duration {
	if expiresIn <= 0 {
		return 0
	}
	refreshSeconds := expiresIn - 1000
	if refreshSeconds <= 0 {
		refreshSeconds = expiresIn / 2
	}
	if refreshSeconds <= 0 {
		refreshSeconds = expiresIn
	}
	return time.Duration(refreshSeconds) * time.Second
}

func (c *Client) endpoint(path string, query url.Values) string {
	baseURL := strings.TrimRight(c.baseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	u, err := url.Parse(baseURL + "/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return baseURL
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func (c *Client) http() *http.Client {
	if c.httpClient == nil {
		return defaultHTTPClient
	}
	return c.httpClient
}

func (c *Client) setBaseURL(baseURL string) {
	c.baseURL = baseURL
}

func (c *Client) setHTTPClient(client *http.Client) {
	c.httpClient = client
}
