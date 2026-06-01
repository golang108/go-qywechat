// Created by Yanjunhui

package crop

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendGetsTokenAndSendsMessage(t *testing.T) {
	var sent Message
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gettoken":
			if r.URL.Query().Get("corpid") != "corp" || r.URL.Query().Get("corpsecret") != "secret" {
				t.Errorf("unexpected token query: %s", r.URL.RawQuery)
				http.Error(w, "bad token query", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
		case "/message/send":
			if r.URL.Query().Get("access_token") != "token" {
				t.Errorf("unexpected send query: %s", r.URL.RawQuery)
				http.Error(w, "bad send query", http.StatusBadRequest)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
				t.Errorf("decode message: %v", err)
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"errcode":0,"errmsg":"ok"}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("corp", 1000001, "secret")
	client.setBaseURL(server.URL)
	client.setHTTPClient(server.Client())

	err := client.Send(Message{ToUser: "alice", Text: Content{Content: "hello"}})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if sent.ToUser != "alice" || sent.AgentID != 1000001 || sent.MsgType != "text" {
		t.Fatalf("unexpected sent message: %#v", sent)
	}
}

func TestSendStopsWhenTokenRequestFails(t *testing.T) {
	sendCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gettoken":
			_, _ = io.WriteString(w, `{"errcode":40013,"errmsg":"invalid corpid"}`)
		case "/message/send":
			sendCalled = true
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("corp", 1000001, "secret")
	client.setBaseURL(server.URL)
	client.setHTTPClient(server.Client())

	err := client.Send(Message{ToUser: "alice", Text: Content{Content: "hello"}})
	if err == nil {
		t.Fatal("expected token error")
	}
	if sendCalled {
		t.Fatal("send endpoint should not be called after token failure")
	}
}

func TestSendReturnsWeComError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gettoken":
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":7200}`)
		case "/message/send":
			_, _ = io.WriteString(w, `{"errcode":40003,"errmsg":"invalid userid"}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New("corp", 1000001, "secret")
	client.setBaseURL(server.URL)
	client.setHTTPClient(server.Client())

	err := client.Send(Message{ToUser: "alice", Text: Content{Content: "hello"}})
	if err == nil {
		t.Fatal("expected send error")
	}
}
