package news

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockFeed(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	old := FeedURL
	FeedURL = srv.URL
	t.Cleanup(func() { FeedURL = old })
}

func TestLatest(t *testing.T) {
	mockFeed(t, `{"result":{"status":{"code":0},"data":{"feed":{"list":[
		{"id":1,"rich_text":"【证监会召开发布会】就市场关切问题答记者问，涉及退市制度与分红新规。","create_time":"2026-07-21 19:22:54"},
		{"id":2,"rich_text":"央行今日开展 500 亿元逆回购操作，净投放 300 亿元。","create_time":"2026-07-21 19:20:01"},
		{"id":3,"rich_text":"","create_time":"2026-07-21 19:00:00"}
	]}}}}`)

	items, err := Latest(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 { // 空文本被过滤
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].Title != "证监会召开发布会" {
		t.Errorf("标题解析异常: %q", items[0].Title)
	}
	if items[0].Content == "" || items[0].Time == "" {
		t.Errorf("正文/时间缺失: %+v", items[0])
	}
	if r := []rune(items[1].Title); len(r) > 31 {
		t.Errorf("无【】标题应截断: %q", items[1].Title)
	}
}

func TestLatestError(t *testing.T) {
	mockFeed(t, `{"result":{"status":{"code":1}}}`)
	if _, err := Latest(context.Background(), 5); err == nil {
		t.Fatal("应报错")
	}
}

func TestTitleOf(t *testing.T) {
	if got := titleOf("【标题】正文"); got != "标题" {
		t.Errorf("titleOf = %q", got)
	}
	long := "这是一条没有书名号标题的快讯内容，长度超过了三十个字符需要被截断处理才行"
	if r := []rune(titleOf(long)); len(r) != 31 { // 30 字 + …
		t.Errorf("截断后应为 31 字: %d %q", len(r), titleOf(long))
	}
}
