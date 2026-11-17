package engine

import (
	"context"
	"fmt"

	"github.com/whiletrue2009/ths-favorite-go/internal/market"
	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

// ItemView 是对外（JSON 输出）的单只股票视图。
type ItemView struct {
	Symbol  string   `json:"symbol"`
	Code    string   `json:"code"`
	Market  string   `json:"market"`
	Price   *float64 `json:"price,omitempty"`
	AddedAt string   `json:"added_at,omitempty"`
}

// GroupView 是对外分组视图，items 保持服务端顺序。
type GroupView struct {
	Name     string     `json:"name"`
	GroupID  string     `json:"group_id"`
	Readonly bool       `json:"readonly"`
	Items    []ItemView `json:"items"`
}

// ListOptions 控制 ListGroups 行为。
type ListOptions struct {
	// Group 非空时只返回该分组（按名称或 ID 匹配）。
	Group string
	// IncludeSelf 把「我的自选」并入结果。
	IncludeSelf bool
	// HydrateDynamic 拉取动态分组（1_ 前缀）的实际成员，较慢，默认只在单组查询时启用。
	HydrateDynamic bool
}

// ListGroups 拉取分组视图。明细（价格/加入时间）尽力附加，失败不影响主列表。
func (e *Engine) ListGroups(ctx context.Context, opts ListOptions) ([]GroupView, error) {
	if err := e.ensureSession(ctx); err != nil {
		return nil, err
	}
	if opts.Group != "" {
		return e.listSingleGroup(ctx, opts.Group)
	}

	metas, _, err := e.api.QueryGroups(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]GroupView, 0, len(metas)+1)
	for _, meta := range metas {
		view := GroupView{
			Name:     meta.Name,
			GroupID:  meta.ID,
			Readonly: meta.IsDynamic(),
			Items:    toItems(meta.Items),
		}
		if meta.IsDynamic() && opts.HydrateDynamic {
			if entries, dynErr := e.api.QueryDynamicPlate(ctx, meta.Name); dynErr == nil {
				view.Items = toItems(entries)
			}
		}
		views = append(views, view)
	}
	if opts.IncludeSelf {
		self, err := e.selfGroup(ctx)
		if err != nil {
			return nil, err
		}
		views = append(views, self)
	}
	e.enrichWithDetail(ctx, views)
	return views, nil
}

// SelfList 返回「我的自选」视图。
func (e *Engine) SelfList(ctx context.Context) (*GroupView, error) {
	if err := e.ensureSession(ctx); err != nil {
		return nil, err
	}
	self, err := e.selfGroup(ctx)
	if err != nil {
		return nil, err
	}
	e.enrichWithDetail(ctx, []GroupView{self})
	return &self, nil
}

// listSingleGroup 返回单个分组；动态分组按需取成员。
func (e *Engine) listSingleGroup(ctx context.Context, identifier string) ([]GroupView, error) {
	if IsSelfIdentifier(identifier) {
		self, err := e.selfGroup(ctx)
		if err != nil {
			return nil, err
		}
		e.enrichWithDetail(ctx, []GroupView{self})
		return []GroupView{self}, nil
	}
	meta, err := e.resolveGroup(ctx, identifier)
	if err != nil {
		return nil, err
	}
	view := GroupView{
		Name:     meta.Name,
		GroupID:  meta.ID,
		Readonly: meta.IsDynamic(),
		Items:    toItems(meta.Items),
	}
	if meta.IsDynamic() {
		entries, dynErr := e.api.QueryDynamicPlate(ctx, meta.Name)
		if dynErr != nil {
			return nil, dynErr
		}
		view.Items = toItems(entries)
	}
	e.enrichWithDetail(ctx, []GroupView{view})
	return []GroupView{view}, nil
}

// selfGroup 用 v2 协议构造「我的自选」视图。
func (e *Engine) selfGroup(ctx context.Context) (GroupView, error) {
	entries, err := e.api.DownloadSelfStocksV2(ctx)
	if err != nil {
		return GroupView{}, err
	}
	return GroupView{
		Name:     SelfGroupName,
		GroupID:  SelfGroupID,
		Readonly: false,
		Items:    toItems(entries),
	}, nil
}

// enrichWithDetail 尽力给视图附加价格/加入时间。任何失败静默跳过。
func (e *Engine) enrichWithDetail(ctx context.Context, views []GroupView) {
	_, details, err := e.api.DownloadSelfStockDetail(ctx)
	if err != nil || len(details) == 0 {
		return
	}
	index := make(map[string]thsclient.StockDetail, len(details))
	for _, detail := range details {
		index[detailKey(detail.Code, detail.Market)] = detail
	}
	for viewIdx := range views {
		for itemIdx := range views[viewIdx].Items {
			item := &views[viewIdx].Items[itemIdx]
			detail, found := index[detailKey(item.Code, item.Market)]
			if !found {
				detail, found = index[detailKey(item.Code, "")]
			}
			if !found {
				continue
			}
			if detail.Price != nil {
				item.Price = detail.Price
			}
			if detail.AddedAt != "" {
				item.AddedAt = detail.AddedAt
			}
		}
	}
}

func detailKey(code, market string) string {
	return code + "|" + market
}

// toItems 把协议条目转为输出视图（市场码 → 缩写）。
func toItems(entries []thsclient.StockEntry) []ItemView {
	items := make([]ItemView, 0, len(entries))
	for _, entry := range entries {
		abbr := market.Abbr(entry.MarketCode)
		items = append(items, ItemView{
			Symbol: entry.Symbol(),
			Code:   entry.Code,
			Market: abbr,
		})
	}
	return items
}

// IsSelfIdentifier 判断分组标识是否指向「我的自选」。
func IsSelfIdentifier(identifier string) bool {
	return identifier == SelfGroupID || identifier == SelfGroupName
}

// resolveGroup 按名称或 ID 找到分组元数据。
func (e *Engine) resolveGroup(ctx context.Context, identifier string) (*thsclient.GroupMeta, error) {
	metas, _, err := e.api.QueryGroups(ctx)
	if err != nil {
		return nil, err
	}
	return findGroupMeta(metas, identifier)
}

// findGroupMeta 在元数据列表中按 ID 优先、名称其次匹配。
func findGroupMeta(metas []thsclient.GroupMeta, identifier string) (*thsclient.GroupMeta, error) {
	for idx := range metas {
		if metas[idx].ID == identifier {
			return &metas[idx], nil
		}
	}
	for idx := range metas {
		if metas[idx].Name == identifier {
			return &metas[idx], nil
		}
	}
	return nil, &thsclient.APIError{
		Action: "查找分组",
		Msg:    fmt.Sprintf("未找到名为 %q 的分组，可先运行 ths list 查看现有分组", identifier),
	}
}

// ensureWritable 拒绝对动态分组（只读）的写操作。
func ensureWritable(meta *thsclient.GroupMeta) error {
	if meta.IsDynamic() {
		return &thsclient.APIError{
			Action: "操作拒绝",
			Msg:    fmt.Sprintf("动态分组 %q 由同花顺条件实时生成，只读不可增删", meta.Name),
		}
	}
	return nil
}
