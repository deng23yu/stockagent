// Package indicator 计算常用技术分析指标。
//
// 所有函数接收按时间升序排列的价格序列，返回等长序列
// (数据不足的头部位置为 NaN)。
package indicator

import "math"

// SMA 计算 n 日简单移动平均。
func SMA(v []float64, n int) []float64 {
	out := nanSlice(len(v))
	if n <= 0 || len(v) < n {
		return out
	}
	sum := 0.0
	for i := range v {
		sum += v[i]
		if i >= n {
			sum -= v[i-n]
		}
		if i >= n-1 {
			out[i] = sum / float64(n)
		}
	}
	return out
}

// EMA 计算 n 日指数移动平均，以首个值为种子。
func EMA(v []float64, n int) []float64 {
	out := make([]float64, len(v))
	if len(v) == 0 || n <= 0 {
		return out
	}
	k := 2 / (float64(n) + 1)
	out[0] = v[0]
	for i := 1; i < len(v); i++ {
		out[i] = v[i]*k + out[i-1]*(1-k)
	}
	return out
}

// MACD 计算 DIF / DEA / BAR，其中 BAR = 2×(DIF-DEA)，与国内行情软件一致。
func MACD(closes []float64, fast, slow, signal int) (dif, dea, bar []float64) {
	ef, es := EMA(closes, fast), EMA(closes, slow)
	dif = make([]float64, len(closes))
	for i := range closes {
		dif[i] = ef[i] - es[i]
	}
	dea = EMA(dif, signal)
	bar = make([]float64, len(closes))
	for i := range closes {
		bar[i] = 2 * (dif[i] - dea[i])
	}
	return dif, dea, bar
}

// RSI 使用 Wilder 平滑法计算 n 日相对强弱指标。
func RSI(closes []float64, n int) []float64 {
	out := nanSlice(len(closes))
	if n <= 0 || len(closes) <= n {
		return out
	}
	var avgGain, avgLoss float64
	for i := 1; i <= n; i++ {
		ch := closes[i] - closes[i-1]
		if ch > 0 {
			avgGain += ch
		} else {
			avgLoss -= ch
		}
	}
	avgGain /= float64(n)
	avgLoss /= float64(n)
	out[n] = rsi(avgGain, avgLoss)
	for i := n + 1; i < len(closes); i++ {
		ch := closes[i] - closes[i-1]
		var gain, loss float64
		if ch > 0 {
			gain = ch
		} else {
			loss = -ch
		}
		avgGain = (avgGain*float64(n-1) + gain) / float64(n)
		avgLoss = (avgLoss*float64(n-1) + loss) / float64(n)
		out[i] = rsi(avgGain, avgLoss)
	}
	return out
}

func rsi(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}

// LogReturns 计算日对数收益率序列。
func LogReturns(closes []float64) []float64 {
	out := make([]float64, 0, len(closes))
	for i := 1; i < len(closes); i++ {
		if closes[i-1] > 0 && closes[i] > 0 {
			out = append(out, math.Log(closes[i]/closes[i-1]))
		}
	}
	return out
}

// AnnualizedVolatility 由日对数收益率计算年化波动率 (%，按 252 个交易日)。
func AnnualizedVolatility(closes []float64) float64 {
	r := LogReturns(closes)
	if len(r) < 2 {
		return 0
	}
	var mean float64
	for _, v := range r {
		mean += v
	}
	mean /= float64(len(r))
	var variance float64
	for _, v := range r {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(r))
	return math.Sqrt(variance) * math.Sqrt(252) * 100
}

// MaxDrawdown 计算序列内最大回撤 (%，历史峰值到其后最低点的最大跌幅)。
func MaxDrawdown(closes []float64) float64 {
	var peak, mdd float64
	for _, v := range closes {
		if v > peak {
			peak = v
		}
		if peak > 0 {
			if dd := (peak - v) / peak * 100; dd > mdd {
				mdd = dd
			}
		}
	}
	return mdd
}

// ChangePct 计算最近 n 日涨跌幅 (%)；数据不足时返回 0。
func ChangePct(closes []float64, n int) float64 {
	if n <= 0 || len(closes) <= n {
		return 0
	}
	base := closes[len(closes)-1-n]
	if base == 0 {
		return 0
	}
	return (closes[len(closes)-1]/base - 1) * 100
}

func nanSlice(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	return out
}

func last(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	return v[len(v)-1]
}
