package tencent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mockServer(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	old := KlineURL
	KlineURL = srv.URL
	t.Cleanup(func() { KlineURL = old })
}

func TestDailyCloses(t *testing.T) {
	mockServer(t, `{"code":0,"msg":"","data":{"sh000001":{"day":[
		["2026-07-17","3865.32","3764.15","3869.21","3745.17","650450984"],
		["2026-07-20","3791.66","3796.28","3831.66","3741.11","709234069"]]}}}`)

	closes, err := DailyCloses(context.Background(), "sh000001", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(closes) != 2 || closes[0] != 3764.15 || closes[1] != 3796.28 {
		t.Errorf("closes = %v", closes)
	}
}

func TestDailyClosesQfqKey(t *testing.T) {
	// 部分标的数据在 qfqday 键下
	mockServer(t, `{"code":0,"msg":"","data":{"sz399001":{"qfqday":[
		["2026-07-20","13600.00","13610.23","13700.00","13500.00","123"]]}}}`)

	closes, err := DailyCloses(context.Background(), "sz399001", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(closes) != 1 || closes[0] != 13610.23 {
		t.Errorf("closes = %v", closes)
	}
}

func TestDailyClosesBadParams(t *testing.T) {
	mockServer(t, `{"code":1,"msg":"bad params","data":{"sh000001":{"version":"16"}}}`)
	if _, err := DailyCloses(context.Background(), "sh000001", 5); err == nil {
		t.Fatal("应报错")
	}
}
