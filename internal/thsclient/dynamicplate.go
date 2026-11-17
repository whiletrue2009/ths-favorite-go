package thsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// 动态板块查询（apigate.10jqka.com.cn）：动态分组（id 1_ 前缀）的成员
// 由同花顺条件实时生成，本地只能镜像，不能增删。
const (
	dynamicPlatePath       = "/d/platform/dynamicplate/stocks/self/v2/select"
	dynamicPlateAttempts   = 2
	dynamicPlateRetryDelay = 500 * time.Millisecond
	dynamicPlateNum        = 1000
)

// QueryDynamicPlate 查询动态板块成员。
// 瞬时无效响应做有界重试；最终失败如实报错，绝不伪装成空列表。
// 合法响应中 data 缺失视为空成员（板块本身没有命中股票）。
func (c *Client) QueryDynamicPlate(ctx context.Context, groupName string) ([]StockEntry, error) {
	query := url.Values{"query": {groupName}, "num": {fmt.Sprint(dynamicPlateNum)}}
	var lastErr error
	for attempt := 1; attempt <= dynamicPlateAttempts; attempt++ {
		body, err := c.Get(ctx, "动态分组", DynamicPlateURL+dynamicPlatePath, query, nil)
		if err != nil {
			lastErr = err
			if attempt < dynamicPlateAttempts {
				time.Sleep(dynamicPlateRetryDelay)
				continue
			}
			break
		}
		var payload struct {
			Data *struct {
				Codes []struct {
					// 线上 code/market 可能是数字也可能是字符串。
					Code   any `json:"code"`
					Market any `json:"market"`
				} `json:"codes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			lastErr = &NetError{Action: "动态分组", Detail: "响应不是有效 JSON: " + err.Error()}
			if attempt < dynamicPlateAttempts {
				time.Sleep(dynamicPlateRetryDelay)
				continue
			}
			break
		}
		if payload.Data == nil {
			return []StockEntry{}, nil
		}
		items := make([]StockEntry, 0, len(payload.Data.Codes))
		for _, code := range payload.Data.Codes {
			items = append(items, StockEntry{Code: fmt.Sprint(code.Code), MarketCode: fmt.Sprint(code.Market)})
		}
		return items, nil
	}
	if lastErr == nil {
		lastErr = &NetError{Action: "动态分组", Detail: "未知失败"}
	}
	return nil, lastErr
}
