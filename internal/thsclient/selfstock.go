package thsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// 「我的自选」双协议：
// v2（t.10jqka.com.cn）— 查询与单只增删，Cookie 鉴权
// v1（ugc.10jqka.com.cn）— 全量替换（批量增删、重排）
const (
	selfStockV2ListPath   = "/newcircle/group/getSelfStockWithMarket/"
	selfStockV2ModifyPath = "/newcircle/group/modifySelfStock/"
	selfStockV1QueryPath  = "/optdata/selfstock/open/api/v1/query"
	selfStockV1ModifyPath = "/optdata/selfstock/open/api/v1/modify"
	selfStockV1From       = "thspc_hevo"
)

// SelfStockList 是 v1 查询结果：版本号 + 完整条目（含顺序）。
type SelfStockList struct {
	Version string
	Items   []StockEntry
}

// DownloadSelfStocksV2 用 v2 协议拉取「我的自选」。
func (c *Client) DownloadSelfStocksV2(ctx context.Context) ([]StockEntry, error) {
	body, err := c.Get(ctx, "我的自选", SelfStockV2Base+selfStockV2ListPath, nil, nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		ErrorCode int    `json:"errorCode"`
		ErrorMsg  string `json:"errorMsg"`
		Result    []struct {
			// 线上 code/marketid 可能是数字也可能是字符串。
			Code     any `json:"code"`
			MarketID any `json:"marketid"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &NetError{Action: "我的自选", Detail: "响应不是有效 JSON: " + err.Error()}
	}
	if payload.ErrorCode != 0 {
		return nil, &APIError{Action: "我的自选", Msg: orDefault(payload.ErrorMsg, "未知错误"), Code: fmt.Sprint(payload.ErrorCode)}
	}
	items := make([]StockEntry, 0, len(payload.Result))
	for _, entry := range payload.Result {
		items = append(items, StockEntry{Code: fmt.Sprint(entry.Code), MarketCode: fmt.Sprint(entry.MarketID)})
	}
	return items, nil
}

// DownloadSelfStocksV1 用 v1 协议拉取「我的自选」（带版本号，用于全量替换）。
func (c *Client) DownloadSelfStocksV1(ctx context.Context) (*SelfStockList, error) {
	query := url.Values{"support_all": {"0"}, "from": {selfStockV1From}}
	headers := map[string]string{}
	if userid := c.Cookies()["userid"]; userid != "" {
		headers["userid"] = userid
	}
	body, err := c.Get(ctx, "我的自选(v1)", UGCBASE+selfStockV1QueryPath, query, headers)
	if err != nil {
		return nil, err
	}
	var payload struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
		Data       *struct {
			SelfStock string `json:"selfstock"`
			// 线上 version 可能是数字也可能是字符串。
			Version any `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &NetError{Action: "我的自选(v1)", Detail: "响应不是有效 JSON: " + err.Error()}
	}
	if payload.StatusCode != 0 {
		return nil, &APIError{Action: "我的自选(v1)", Msg: orDefault(payload.StatusMsg, "未知错误"), Code: fmt.Sprint(payload.StatusCode)}
	}
	if payload.Data == nil {
		return nil, &APIError{Action: "我的自选(v1)", Msg: "响应缺少 data 字段"}
	}
	version := ""
	if payload.Data.Version != nil {
		version = fmt.Sprint(payload.Data.Version)
	}
	return &SelfStockList{
		Version: version,
		Items:   ParseStockList(payload.Data.SelfStock),
	}, nil
}

// ModifySelfStocksV1 全量替换「我的自选」，返回服务端新版本号。
func (c *Client) ModifySelfStocksV1(ctx context.Context, entries []StockEntry, version string) (string, error) {
	form := url.Values{
		"selfstock": {FormatStockList(entries)},
		"from":      {selfStockV1From},
		"version":   {version},
		"num":       {fmt.Sprint(len(entries))},
	}
	body, err := c.PostForm(ctx, "我的自选(v1)", UGCBASE+selfStockV1ModifyPath, form)
	if err != nil {
		return "", err
	}
	var payload struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
		Data       *struct {
			Version any `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", &NetError{Action: "我的自选(v1)", Detail: "响应不是有效 JSON: " + err.Error()}
	}
	if payload.StatusCode != 0 {
		return "", &APIError{Action: "我的自选(v1)", Msg: orDefault(payload.StatusMsg, "未知错误"), Code: fmt.Sprint(payload.StatusCode)}
	}
	newVersion := version
	if payload.Data != nil && payload.Data.Version != nil {
		newVersion = fmt.Sprint(payload.Data.Version)
	}
	return newVersion, nil
}
