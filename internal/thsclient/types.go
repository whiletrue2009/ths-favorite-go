package thsclient

import "github.com/whiletrue2009/ths-favorite-go/internal/market"

// StockEntry 是接口层的原始股票条目：代码 + 同花顺数字市场码。
type StockEntry struct {
	Code       string
	MarketCode string
}

// Symbol 返回人类习惯的 code.market 形式（市场码转缩写，未知码原样保留）。
func (e StockEntry) Symbol() string {
	abbr := market.Abbr(e.MarketCode)
	if abbr == "" {
		return e.Code
	}
	return e.Code + "." + abbr
}
