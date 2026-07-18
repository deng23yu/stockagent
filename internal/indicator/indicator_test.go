package indicator

import (
	"math"
	"testing"
)

func almostEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSMA(t *testing.T) {
	v := []float64{1, 2, 3, 4, 5}
	got := SMA(v, 3)
	if !math.IsNaN(got[0]) || !math.IsNaN(got[1]) {
		t.Fatalf("头部应为 NaN, got %v", got[:2])
	}
	want := []float64{2, 3, 4}
	for i, w := range want {
		if !almostEq(got[i+2], w) {
			t.Errorf("SMA[%d] = %v, want %v", i+2, got[i+2], w)
		}
	}
}

func TestEMAConstant(t *testing.T) {
	v := []float64{5, 5, 5, 5}
	for i, x := range EMA(v, 3) {
		if !almostEq(x, 5) {
			t.Errorf("EMA[%d] = %v, want 5", i, x)
		}
	}
}

func TestMACDConstant(t *testing.T) {
	v := make([]float64, 50)
	for i := range v {
		v[i] = 10
	}
	dif, dea, bar := MACD(v, 12, 26, 9)
	for i := range v {
		if !almostEq(dif[i], 0) || !almostEq(dea[i], 0) || !almostEq(bar[i], 0) {
			t.Fatalf("常数序列 MACD 应全为 0, idx %d: %v %v %v", i, dif[i], dea[i], bar[i])
		}
	}
}

// TestRSIWilder 手工核算 n=2 的 Wilder RSI。
func TestRSIWilder(t *testing.T) {
	closes := []float64{10, 12, 11, 13}
	got := RSI(closes, 2)
	// i=1: +2, i=2: -1 → avgGain=1, avgLoss=0.5 → RSI=100-100/(1+2)≈66.67
	if math.Abs(got[2]-66.6667) > 0.01 {
		t.Errorf("RSI[2] = %v, want ≈66.67", got[2])
	}
	// i=3: +2 → avgGain=(1+2)/2=1.5, avgLoss=(0.5+0)/2=0.25 → RSI≈85.71
	if math.Abs(got[3]-85.7143) > 0.01 {
		t.Errorf("RSI[3] = %v, want ≈85.71", got[3])
	}
}

func TestRSIAllGains(t *testing.T) {
	closes := []float64{1, 2, 3, 4, 5, 6}
	if got := RSI(closes, 3); !almostEq(got[len(got)-1], 100) {
		t.Errorf("全涨序列 RSI 应为 100, got %v", got[len(got)-1])
	}
}

func TestMaxDrawdown(t *testing.T) {
	closes := []float64{100, 120, 90, 110}
	if got := MaxDrawdown(closes); !almostEq(got, 25) {
		t.Errorf("MaxDrawdown = %v, want 25", got)
	}
}

func TestChangePct(t *testing.T) {
	if got := ChangePct([]float64{100, 110}, 1); !almostEq(got, 10) {
		t.Errorf("ChangePct = %v, want 10", got)
	}
	if got := ChangePct([]float64{100}, 5); got != 0 {
		t.Errorf("数据不足应为 0, got %v", got)
	}
}

func TestAnnualizedVolatilityConstant(t *testing.T) {
	if got := AnnualizedVolatility([]float64{7, 7, 7, 7}); got != 0 {
		t.Errorf("常数序列波动率应为 0, got %v", got)
	}
}

func TestSummarize(t *testing.T) {
	const n = 250
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i := range closes {
		closes[i] = 100 + float64(i)
		highs[i] = closes[i] + 2
		lows[i] = closes[i] - 2
	}
	s := Summarize(closes, highs, lows)
	if !almostEq(s.Price, 349) {
		t.Errorf("Price = %v, want 349", s.Price)
	}
	// 最后 5 根收盘 345..349, 均值 347
	if !almostEq(s.MA5, 347) {
		t.Errorf("MA5 = %v, want 347", s.MA5)
	}
	if !almostEq(s.High52W, 351) || !almostEq(s.Low52W, 98) {
		t.Errorf("52周区间 = %v ~ %v, want 98 ~ 351", s.Low52W, s.High52W)
	}
	if s.RSI14 != 100 {
		t.Errorf("单边上涨 RSI14 应为 100, got %v", s.RSI14)
	}
}
