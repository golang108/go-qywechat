// Created by Yanjunhui

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/yanjunhui/chat/crop"
)

type fakeSender struct {
	messages []crop.Message
	err      error
}

func (f *fakeSender) Send(msg crop.Message) error {
	f.messages = append(f.messages, msg)
	return f.err
}

func TestMessageRequestMapsOpenFalconPayload(t *testing.T) {
	sender := &fakeSender{}
	rec := performRequest(NewServer(sender, "secret"), http.MethodGet, "/send?token=secret&tos=alice&content=hello")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("expected one message, got %d", len(sender.messages))
	}

	msg := sender.messages[0]
	if msg.ToUser != "alice" || msg.MsgType != "text" || msg.Text.Content != "hello" {
		t.Fatalf("unexpected message: %#v", msg)
	}
}

func TestMessageRequestRequiresAuthToken(t *testing.T) {
	sender := &fakeSender{}
	rec := performRequest(NewServer(sender, "secret"), http.MethodGet, "/send?tos=alice&content=hello")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if len(sender.messages) != 0 {
		t.Fatalf("expected no messages, got %d", len(sender.messages))
	}
}

func TestMessageRequestValidatesPayload(t *testing.T) {
	sender := &fakeSender{}
	rec := performRequest(NewServer(sender, "secret"), http.MethodGet, "/send?token=secret&tos=alice")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if len(sender.messages) != 0 {
		t.Fatalf("expected no messages, got %d", len(sender.messages))
	}
}

func TestMessageRequestMasksSenderError(t *testing.T) {
	sender := &fakeSender{err: errors.New("wechat secret detail")}
	rec := performRequest(NewServer(sender, "secret"), http.MethodGet, "/send?token=secret&tos=alice&content=hello")

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, rec.Code)
	}
	if rec.Body.String() != "send failed" {
		t.Fatalf("unexpected response body: %q", rec.Body.String())
	}
}

func TestLoadConfigReadsFileAndEnvOverrides(t *testing.T) {
	path := writeTempConfig(t, `
[http]
address = 127.0.0.1
port = 4567

[server]
auth_token = from-file

[weixin]
CorpID = from-file-corp
AgentId = 1000001
Secret = from-file-secret
`)
	t.Setenv("CHAT_HTTP_PORT", "5678")
	t.Setenv("CHAT_AUTH_TOKEN", "from-env")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.HTTPAddress != "127.0.0.1" || cfg.HTTPPort != 5678 {
		t.Fatalf("unexpected HTTP config: %#v", cfg)
	}
	if cfg.CorpID != "from-file-corp" || cfg.AgentID != 1000001 || cfg.Secret != "from-file-secret" {
		t.Fatalf("unexpected weixin config: %#v", cfg)
	}
	if cfg.AuthToken != "from-env" {
		t.Fatalf("expected env auth token, got %q", cfg.AuthToken)
	}
}

func TestLoadConfigRequiresAuthToken(t *testing.T) {
	path := writeTempConfig(t, `
[weixin]
CorpID = corp
AgentId = 1000001
Secret = secret
`)

	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected missing auth token error")
	}
}

func TestLoadConfigReturnsErrorForExplicitMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.conf")

	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected missing config file error")
	}
}

func performRequest(server *Server, method, target string) *httptest.ResponseRecorder {
	e := echo.New()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	_ = server.MessageRequest(ctx)
	return rec
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.conf")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
