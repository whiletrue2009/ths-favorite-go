package thsclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/whiletrue2009/ths-favorite-go/internal/market"
	"github.com/whiletrue2009/ths-favorite-go/internal/thsxml"
)

// selfStockDetailPath 是 selfstock_detail 接口路径；完整 URL 在调用时拼接，
// 保证测试可以重定向 UGCBASE。
const selfStockDetailPath = "/selfstock_detail"

// StockDetail 是单只股票的加入元数据。
type StockDetail struct {
	Code    string
	Market  string // 市场缩写（SH/SZ/...）
	Price   *float64
	AddedAt string
}

// DownloadSelfStockDetail 拉取自选明细（价格/加入时间），返回 (版本号, 明细表)。
func (c *Client) DownloadSelfStockDetail(ctx context.Context) (string, []StockDetail, error) {
	userid := c.Cookies()["userid"]
	if userid == "" {
		return "", nil, &AuthError{Msg: "Cookie 中缺少 userid，无法获取自选明细"}
	}
	query := url.Values{"reqtype": {"download"}, "app_flag": {"0E"}, "userid": {userid}}
	headers := map[string]string{"userid": userid}
	body, err := c.Get(ctx, "selfstock_detail", UGCBASE+selfStockDetailPath, query, headers)
	if err != nil {
		return "", nil, err
	}
	resp, err := thsxml.Parse(body)
	if err != nil {
		return "", nil, &NetError{Action: "selfstock_detail", Detail: err.Error()}
	}
	version := resp.Item.Attr("version")
	blob := resp.Item.Attr("selfstock_detail")
	details, err := decodeDetailBlob(blob)
	if err != nil {
		return "", nil, &NetError{Action: "selfstock_detail", Detail: err.Error()}
	}
	return version, details, nil
}

// decodeDetailBlob 解码 base64(JSON 数组)。
func decodeDetailBlob(blob string) ([]StockDetail, error) {
	if blob == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return nil, fmt.Errorf("明细数据 base64 解码失败: %w", err)
	}
	var entries []struct {
		C string `json:"C"`
		M string `json:"M"`
		P any    `json:"P"`
		T any    `json:"T"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("明细数据 JSON 解析失败: %w", err)
	}
	details := make([]StockDetail, 0, len(entries))
	for _, entry := range entries {
		if entry.C == "" {
			continue
		}
		detail := StockDetail{Code: entry.C, Market: detailMarketAbbr(entry.M)}
		if price, ok := toFloat(entry.P); ok {
			detail.Price = &price
		}
		detail.AddedAt = toString(entry.T)
		details = append(details, detail)
	}
	return details, nil
}

// detailMarketAbbr 转换明细中的市场码为缩写。
func detailMarketAbbr(marketCode string) string {
	return market.Abbr(marketCode)
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprint(int64(v))
		}
		return fmt.Sprint(v)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}
