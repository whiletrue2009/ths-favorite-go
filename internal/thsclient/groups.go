package thsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// 分组元数据接口（ugc.10jqka.com.cn，JSON 协议）。
const (
	groupQueryPath = "/optdata/selfgroup/open/api/group/v1/query"
	groupAddPath   = "/optdata/selfgroup/open/api/group/v1/add"
	groupDelPath   = "/optdata/selfgroup/open/api/group/v1/delete"
	sharePath      = "/optdata/sharing_service/open/api/sharing/v1/create"
	fromParam      = "sjcg_gphone"
)

// GroupMeta 是 query 接口返回的单个分组元数据。
type GroupMeta struct {
	ID      string
	Name    string
	Type    string
	Content string // "code|code,...,market|market" 原始串
	Items   []StockEntry
}

// IsDynamic 判断是否为动态板块分组（只读，id 以 1_ 开头）。
func (g GroupMeta) IsDynamic() bool {
	return len(g.ID) >= 2 && g.ID[:2] == "1_"
}

// groupQueryResponse 是 query 响应结构。
type groupQueryResponse struct {
	StatusCode int             `json:"status_code"`
	StatusMsg  string          `json:"status_msg"`
	Data       *groupQueryData `json:"data"`
}

type groupQueryData struct {
	// 线上 version 可能是数字也可能是字符串，统一 any 后转字符串。
	Version   any            `json:"version"`
	GroupList []groupRawItem `json:"group_list"`
}

type groupRawItem struct {
	ID      any    `json:"id"`
	Name    any    `json:"name"`
	Type    any    `json:"type"`
	Num     any    `json:"num"`
	Content string `json:"content"`
}

// QueryGroups 拉取全部分组元数据与分组内股票（content 字段）。
func (c *Client) QueryGroups(ctx context.Context) ([]GroupMeta, string, error) {
	query := url.Values{"from": {fromParam}, "types": {"0,1"}}
	body, err := c.Get(ctx, "获取分组", UGCBASE+groupQueryPath, query, nil)
	if err != nil {
		return nil, "", err
	}
	var payload groupQueryResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", &NetError{Action: "获取分组", Detail: "响应不是有效 JSON: " + err.Error()}
	}
	if payload.StatusCode != 0 {
		return nil, "", &APIError{Action: "获取分组", Msg: orDefault(payload.StatusMsg, "未知错误"), Code: fmt.Sprint(payload.StatusCode)}
	}
	if payload.Data == nil {
		return nil, "", &APIError{Action: "获取分组", Msg: "响应缺少 data 字段"}
	}
	version := ""
	if payload.Data.Version != nil {
		version = fmt.Sprint(payload.Data.Version)
	}
	groups := make([]GroupMeta, 0, len(payload.Data.GroupList))
	for _, raw := range payload.Data.GroupList {
		group := GroupMeta{
			ID:      fmt.Sprint(raw.ID),
			Name:    fmt.Sprint(raw.Name),
			Type:    fmt.Sprint(raw.Type),
			Content: raw.Content,
			Items:   ParseStockList(raw.Content),
		}
		groups = append(groups, group)
	}
	return groups, version, nil
}

// ParseStockList 解析 "code|code,...,market|market" 格式。
// 与 v1 selfstock、blockstock group data 共用同一格式。
func ParseStockList(content string) []StockEntry {
	if content == "" {
		return nil
	}
	codesSegment, typesSegment := cutStockList(content)
	codes := splitNonEmpty(codesSegment, '|')
	types := splitNonEmpty(typesSegment, '|')
	entries := make([]StockEntry, 0, len(codes))
	for i, code := range codes {
		marketCode := ""
		if i < len(types) {
			marketCode = types[i]
		}
		entries = append(entries, StockEntry{Code: code, MarketCode: marketCode})
	}
	return entries
}

// FormatStockList 把条目列表编码回 "code|code,...,market|market"。
func FormatStockList(entries []StockEntry) string {
	if len(entries) == 0 {
		return ""
	}
	codes := make([]string, len(entries))
	types := make([]string, len(entries))
	for i, e := range entries {
		codes[i] = e.Code
		types[i] = e.MarketCode
	}
	return strings.Join(codes, "|") + "," + strings.Join(types, "|")
}

// GroupCreate 创建自定义分组。version 来自最近一次 QueryGroups。
func (c *Client) GroupCreate(ctx context.Context, name, version string) error {
	form := url.Values{
		"name":    {name},
		"type":    {"0"},
		"version": {version},
		"from":    {fromParam},
	}
	return c.postGroupForm(ctx, "添加分组", groupAddPath, form)
}

// GroupDelete 删除自定义分组。
func (c *Client) GroupDelete(ctx context.Context, groupID, version string) error {
	form := url.Values{
		"ids":     {groupID},
		"version": {version},
		"from":    {fromParam},
	}
	return c.postGroupForm(ctx, "删除分组", groupDelPath, form)
}

func (c *Client) postGroupForm(ctx context.Context, action, path string, form url.Values) error {
	body, err := c.PostForm(ctx, action, UGCBASE+path, form)
	if err != nil {
		return err
	}
	var payload statusEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		return &NetError{Action: action, Detail: "响应不是有效 JSON: " + err.Error()}
	}
	if payload.StatusCode != 0 {
		return &APIError{Action: action, Msg: orDefault(payload.StatusMsg, "未知错误"), Code: fmt.Sprint(payload.StatusCode)}
	}
	return nil
}

// statusEnvelope 是 ugc JSON 接口统一的状态信封。
type statusEnvelope struct {
	StatusCode int             `json:"status_code"`
	StatusMsg  string          `json:"status_msg"`
	Data       json.RawMessage `json:"data"`
}

// GroupShare 生成分组分享链接，validSeconds 为链接有效期（秒）。
func (c *Client) GroupShare(ctx context.Context, userID, groupID, groupName string, validSeconds int) (string, error) {
	// biz_key 取分组 id 中第一个 '_' 之后的部分。
	bizSuffix := groupID
	if idx := strings.IndexByte(groupID, '_'); idx >= 0 {
		bizSuffix = groupID[idx+1:]
	}
	payload := map[string]any{
		"biz":        "selfstock",
		"valid_time": validSeconds,
		"biz_key":    userID + "_" + bizSuffix,
		"name":       groupName,
		"url_style":  0,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("构造分享请求失败: %w", err)
	}
	body, err := c.PostJSON(ctx, "分享分组", UGCBASE+sharePath, encoded)
	if err != nil {
		return "", err
	}
	var resp statusEnvelope
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", &NetError{Action: "分享分组", Detail: "响应不是有效 JSON: " + err.Error()}
	}
	if resp.StatusCode != 0 {
		return "", &APIError{Action: "分享分组", Msg: orDefault(resp.StatusMsg, "未知错误"), Code: fmt.Sprint(resp.StatusCode)}
	}
	var data struct {
		ShareURL string `json:"share_url"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil || data.ShareURL == "" {
		return "", &APIError{Action: "分享分组", Msg: "分享成功但未返回链接"}
	}
	return data.ShareURL, nil
}
