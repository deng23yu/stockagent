package eastmoney

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func capitalMock(t *testing.T, body string) (restore func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	oldDC, oldFF := DataCenterURL, FFlowBaseURL
	DataCenterURL, FFlowBaseURL = srv.URL, srv.URL
	return func() { DataCenterURL, FFlowBaseURL = oldDC, oldFF }
}

func TestMargin(t *testing.T) {
	restore := capitalMock(t, `{"success":true,"result":{"data":[{
		"DATE":"2026-07-20 00:00:00","SCODE":"600519","SECNAME":"贵州茅台",
		"RZYE":18189764566,"RQYE":159313275,"RZRQYE":18349077841,
		"RZMRE":1099058777,"RZYEZB":1.0961,"RZMRE5D":3418179149}]}}`)
	defer restore()

	m, err := New(nil).Margin(context.Background(), "600519")
	if err != nil {
		t.Fatal(err)
	}
	if m.Date != "2026-07-20" || m.RZYE != 18189764566 || m.RQYE != 159313275 ||
		m.RZRQYE != 18349077841 || m.RZMRE != 1099058777 || m.RZYEZB != 1.0961 {
		t.Errorf("两融解析异常: %+v", m)
	}
}

func TestFundFlow(t *testing.T) {
	restore := capitalMock(t, `{"rc":0,"data":{"code":"600519","klines":[
		"2026-07-20,100000000,2000000,3000000,60000000,38000000",
		"2026-07-21,-579159136,-367185,579526224,-484363808,-94795328"]}}`)
	defer restore()

	flows, err := New(nil).FundFlow(context.Background(), "1.600519", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(flows) != 2 {
		t.Fatalf("flows = %d, want 2", len(flows))
	}
	d := flows[1]
	if d.Date != "2026-07-21" || d.Main != -579159136 || d.Small != -367185 ||
		d.Medium != 579526224 || d.Large != -484363808 || d.SuperLarge != -94795328 {
		t.Errorf("资金流解析异常: %+v", d)
	}
}

func TestNorthbound(t *testing.T) {
	restore := capitalMock(t, `{"success":true,"result":{"data":[{
		"TRADE_DATE":"2026-06-30 00:00:00","SECURITY_CODE":"600519",
		"HOLD_SHARES":53711656,"HOLD_MARKET_CAP":63674631071.44,"HOLD_SHARES_RATIO":4.29}]}}`)
	defer restore()

	h, err := New(nil).Northbound(context.Background(), "600519")
	if err != nil {
		t.Fatal(err)
	}
	if h.Date != "2026-06-30" || h.Shares != 53711656 || h.MarketCap != 63674631071.44 || h.SharesRatio != 4.29 {
		t.Errorf("北向持股解析异常: %+v", h)
	}
}

func TestCapitalEmpty(t *testing.T) {
	restore := capitalMock(t, `{"success":false,"result":null}`)
	defer restore()
	if _, err := New(nil).Margin(context.Background(), "600519"); err == nil {
		t.Fatal("空数据应报错")
	}
}
