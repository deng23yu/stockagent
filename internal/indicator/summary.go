package indicator

// Summary 汇总一只股票的最新指标快照。
type Summary struct {
	Price         float64
	ChangePct5    float64
	ChangePct20   float64
	ChangePct60   float64
	ChangePct120  float64
	MA5           float64
	MA10          float64
	MA20          float64
	MA60          float64
	DIF           float64
	DEA           float64
	BAR           float64
	RSI14         float64
	AnnualizedVol float64
	MaxDrawdown   float64
	High52W       float64
	Low52W        float64
}

// Summarize 基于收盘价/最高价/最低价序列 (升序) 计算最新指标快照。
func Summarize(closes, highs, lows []float64) Summary {
	s := Summary{Price: last(closes)}
	s.ChangePct5 = ChangePct(closes, 5)
	s.ChangePct20 = ChangePct(closes, 20)
	s.ChangePct60 = ChangePct(closes, 60)
	s.ChangePct120 = ChangePct(closes, 120)
	s.MA5 = last(SMA(closes, 5))
	s.MA10 = last(SMA(closes, 10))
	s.MA20 = last(SMA(closes, 20))
	s.MA60 = last(SMA(closes, 60))
	dif, dea, bar := MACD(closes, 12, 26, 9)
	s.DIF = last(dif)
	s.DEA = last(dea)
	s.BAR = last(bar)
	s.RSI14 = last(RSI(closes, 14))
	s.AnnualizedVol = AnnualizedVolatility(closes)
	s.MaxDrawdown = MaxDrawdown(closes)
	s.High52W, s.Low52W = rangeOf(highs, lows)
	return s
}

// rangeOf 返回区间最高价与最低价 (忽略非正价格)。
func rangeOf(highs, lows []float64) (hi, lo float64) {
	for _, h := range highs {
		if h > hi {
			hi = h
		}
	}
	for _, l := range lows {
		if l <= 0 {
			continue
		}
		if lo == 0 || l < lo {
			lo = l
		}
	}
	return hi, lo
}
