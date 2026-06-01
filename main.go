package main

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/yanjunhui/chat/crop"
)

const defaultConfigPath = "config.conf"

type Config struct {
	HTTPAddress string
	HTTPPort    int
	CorpID      string
	AgentID     int
	Secret      string
	AuthToken   string
}

type SendRequest struct {
	Tos     string `query:"tos" json:"tos" form:"tos"`
	Content string `query:"content" json:"content" form:"content"`
}

type messageSender interface {
	Send(crop.Message) error
}

type Server struct {
	sender    messageSender
	authToken string
}

func NewServer(sender messageSender, authToken string) *Server {
	return &Server{sender: sender, authToken: authToken}
}

func main() {
	cfg, err := LoadConfig("")
	if err != nil {
		log.Fatal("加载配置失败: ", err)
	}

	client := crop.New(cfg.CorpID, cfg.AgentID, cfg.Secret)
	server := NewServer(client, cfg.AuthToken)

	e := echo.New()
	e.GET("/send", server.MessageRequest)
	e.POST("/send", server.MessageRequest)

	addr := net.JoinHostPort(cfg.HTTPAddress, strconv.Itoa(cfg.HTTPPort))
	err = e.Start(addr)
	if err != nil {
		log.Println("启动服务失败:", err)
	}
}

func (s *Server) MessageRequest(ctx echo.Context) error {
	if !s.authorized(ctx) {
		return ctx.String(http.StatusUnauthorized, "unauthorized")
	}

	req := new(SendRequest)
	if err := ctx.Bind(req); err != nil {
		return ctx.String(http.StatusBadRequest, "invalid request")
	}

	msg, err := buildMessage(*req)
	if err != nil {
		return ctx.String(http.StatusBadRequest, err.Error())
	}

	if err = s.sender.Send(msg); err != nil {
		log.Println("发送企业微信消息失败:", err)
		return ctx.String(http.StatusBadGateway, "send failed")
	}

	return ctx.String(http.StatusOK, "ok")
}

func buildMessage(req SendRequest) (crop.Message, error) {
	tos := strings.TrimSpace(req.Tos)
	content := strings.TrimSpace(req.Content)
	if tos == "" {
		return crop.Message{}, errors.New("tos is required")
	}
	if content == "" {
		return crop.Message{}, errors.New("content is required")
	}
	return crop.Message{
		ToUser:  tos,
		MsgType: "text",
		Text:    crop.Content{Content: content},
	}, nil
}

func (s *Server) authorized(ctx echo.Context) bool {
	if s.authToken == "" {
		return true
	}
	token := ctx.QueryParam("token")
	if token == "" {
		token = ctx.Request().Header.Get("X-Chat-Token")
	}
	if token == "" {
		token = bearerToken(ctx.Request().Header.Get("Authorization"))
	}
	return constantTimeEqual(token, s.authToken)
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func constantTimeEqual(got, want string) bool {
	if len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func LoadConfig(path string) (Config, error) {
	cfg := Config{HTTPAddress: "0.0.0.0", HTTPPort: 4567}
	allowMissing := false
	if path == "" {
		path = os.Getenv("CHAT_CONFIG")
	}
	if path == "" {
		path = defaultConfigPath
		allowMissing = true
	}

	values, err := readConfigFile(path)
	if err != nil && (!allowMissing || !errors.Is(err, os.ErrNotExist)) {
		return Config{}, err
	}
	applyConfigValues(&cfg, values)
	applyEnv(&cfg)
	if err = validateConfig(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func readConfigFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return parseConfig(string(data)), nil
}

func parseConfig(data string) map[string]string {
	values := make(map[string]string)
	section := ""
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fullKey := strings.ToLower(strings.TrimSpace(key))
		if section != "" {
			fullKey = section + "." + fullKey
		}
		values[fullKey] = strings.TrimSpace(value)
	}
	return values
}

func applyConfigValues(cfg *Config, values map[string]string) {
	cfg.HTTPAddress = firstNonEmpty(values["http.address"], cfg.HTTPAddress)
	cfg.HTTPPort = intValue(values["http.port"], cfg.HTTPPort)
	cfg.CorpID = firstNonEmpty(values["weixin.corpid"], cfg.CorpID)
	cfg.AgentID = intValue(firstNonEmpty(values["weixin.agentid"], values["weixin.agent_id"]), cfg.AgentID)
	cfg.Secret = firstNonEmpty(values["weixin.secret"], cfg.Secret)
	cfg.AuthToken = firstNonEmpty(values["server.auth_token"], cfg.AuthToken)
}

func applyEnv(cfg *Config) {
	cfg.HTTPAddress = firstNonEmpty(os.Getenv("CHAT_HTTP_ADDRESS"), cfg.HTTPAddress)
	cfg.HTTPPort = intValue(os.Getenv("CHAT_HTTP_PORT"), cfg.HTTPPort)
	cfg.CorpID = firstNonEmpty(os.Getenv("WEIXIN_CORP_ID"), cfg.CorpID)
	cfg.AgentID = intValue(os.Getenv("WEIXIN_AGENT_ID"), cfg.AgentID)
	cfg.Secret = firstNonEmpty(os.Getenv("WEIXIN_SECRET"), cfg.Secret)
	cfg.AuthToken = firstNonEmpty(os.Getenv("CHAT_AUTH_TOKEN"), cfg.AuthToken)
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.CorpID) == "" {
		return errors.New("weixin CorpID is required")
	}
	if cfg.AgentID <= 0 {
		return errors.New("weixin AgentId is required")
	}
	if strings.TrimSpace(cfg.Secret) == "" {
		return errors.New("weixin Secret is required")
	}
	if strings.TrimSpace(cfg.AuthToken) == "" {
		return errors.New("server auth_token is required")
	}
	if cfg.HTTPPort <= 0 || cfg.HTTPPort > 65535 {
		return fmt.Errorf("invalid http port: %d", cfg.HTTPPort)
	}
	return nil
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func intValue(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return number
}
