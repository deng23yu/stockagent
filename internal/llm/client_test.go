package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChat(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"{\"signal\":\"bullish\"}"}}]}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "sk-test", "deepseek-chat")
	out, err := c.Chat(context.Background(), "sys", "user", true)
	if err != nil {
		t.Fatal(err)
	}
	if out != `{"signal":"bullish"}` {
		t.Errorf("out = %q", out)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"response_format":{"type":"json_object"}`) {
		t.Errorf("jsonMode 应携带 response_format: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"model":"deepseek-chat"`) {
		t.Errorf("请求体缺少模型名: %s", gotBody)
	}
}

func TestChatRetryOn5xx(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer srv.Close()

	out, err := New(srv.URL, "k", "m").Chat(context.Background(), "s", "u", false)
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok" || calls != 2 {
		t.Errorf("out=%q calls=%d, want ok/2", out, calls)
	}
}

func TestChatNoRetryOn4xx(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	_, err := New(srv.URL, "bad", "m").Chat(context.Background(), "s", "u", true)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("err = %v, want 包含 401", err)
	}
	if calls != 1 {
		t.Errorf("401 不应重试, calls = %d", calls)
	}
}
