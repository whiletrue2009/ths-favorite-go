package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/whiletrue2009/ths-favorite-go/internal/market"
	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

// reorderSnapshotAttempts 重排前对远端快照做有界校验重试
// （远端写入有传播延迟，短时间内读到旧版本是正常的）。
const (
	reorderSnapshotAttempts = 5
	// 写入生效等待：上传成功后轮询直到读取可见，超时不报错。
	applyWaitTimeout  = 20 * time.Second
	applyPollInterval = 2 * time.Second
)

// snapshotRetryDelay 是重排快照重试间隔；做成包级变量便于测试缩短。
var snapshotRetryDelay = 2 * time.Second

// applyWaitConfig 允许测试注入更短的等待。
type applyWaitConfig struct {
	Timeout   time.Duration
	Interval  time.Duration
	SleepFunc func(time.Duration)
}

// waitBlockstockApplied 等待异步写入在读取路径可见。
// multiStorage 写入是异步应用的（线上观测延迟可达 ~1 分钟）；
// 超时静默返回——上传已被接受，只影响后续读取的新鲜度。
func (e *Engine) waitBlockstockApplied(ctx context.Context, uploadedVersion uint64) {
	wait := e.applyWait
	if wait.Timeout == 0 {
		wait = applyWaitConfig{Timeout: applyWaitTimeout, Interval: applyPollInterval, SleepFunc: time.Sleep}
	}
	deadline := time.Now().Add(wait.Timeout)
	for time.Now().Before(deadline) {
		download, err := e.api.BlockstockDownload(ctx, e.currentAuthParams())
		if err == nil && download.Version >= uploadedVersion {
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
			wait.SleepFunc(wait.Interval)
		}
	}
}

// PlanResult 是写操作 dry-run 的返回结构。
type PlanResult struct {
	Operation string   `json:"operation"`
	Group     string   `json:"group"`
	GroupID   string   `json:"group_id,omitempty"`
	Symbols   []string `json:"symbols,omitempty"`
	Note      string   `json:"note,omitempty"`
	DryRun    bool     `json:"dry_run"`
}

// WriteResult 是写操作实际执行的返回结构。
type WriteResult struct {
	Operation string   `json:"operation"`
	Group     string   `json:"group"`
	Symbols   []string `json:"symbols,omitempty"`
	Version   string   `json:"version,omitempty"`
	ShareURL  string   `json:"share_url,omitempty"`
}

// StockAdd 向分组批量添加股票。自定义分组走 multiStorage 全量替换，
// 「我的自选」走 v1 全量替换；都是「读-合并-写」一次远端请求。
func (e *Engine) StockAdd(ctx context.Context, groupIdentifier string, symbols []string, dryRun bool) (*PlanResult, *WriteResult, error) {
	return e.stockMutate(ctx, "stock.add", groupIdentifier, symbols, dryRun, mergeAdd)
}

// StockDel 从分组批量删除股票。
func (e *Engine) StockDel(ctx context.Context, groupIdentifier string, symbols []string, dryRun bool) (*PlanResult, *WriteResult, error) {
	return e.stockMutate(ctx, "stock.remove", groupIdentifier, symbols, dryRun, mergeDelete)
}

// stockMutate 是增删的公共实现。
func (e *Engine) stockMutate(
	ctx context.Context,
	operation, groupIdentifier string,
	symbols []string,
	dryRun bool,
	merge func(current, delta []thsclient.StockEntry) []thsclient.StockEntry,
) (*PlanResult, *WriteResult, error) {
	if len(symbols) == 0 {
		return nil, nil, &thsclient.APIError{Action: operation, Msg: "股票列表不能为空"}
	}
	if err := e.ensureSession(ctx); err != nil {
		return nil, nil, err
	}
	entries, err := ParseSymbols(symbols)
	if err != nil {
		return nil, nil, err
	}

	if IsSelfIdentifier(groupIdentifier) {
		if dryRun {
			return &PlanResult{Operation: operation, Group: SelfGroupName, GroupID: SelfGroupID, Symbols: symbols, DryRun: true}, nil, nil
		}
		current, err := e.api.DownloadSelfStocksV1(ctx)
		if err != nil {
			return nil, nil, err
		}
		version, err := e.api.ModifySelfStocksV1(ctx, merge(current.Items, entries), current.Version)
		if err != nil {
			return nil, nil, err
		}
		return nil, &WriteResult{Operation: operation, Group: SelfGroupName, Symbols: symbols, Version: version}, nil
	}

	meta, err := e.resolveGroup(ctx, groupIdentifier)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureWritable(meta); err != nil {
		return nil, nil, err
	}
	if dryRun {
		return &PlanResult{Operation: operation, Group: meta.Name, GroupID: meta.ID, Symbols: symbols, DryRun: true}, nil, nil
	}
	if _, err := e.replaceGroupContent(ctx, meta.Name, func(current []thsclient.StockEntry) []thsclient.StockEntry {
		return merge(current, entries)
	}); err != nil {
		return nil, nil, err
	}
	return nil, &WriteResult{Operation: operation, Group: meta.Name, Symbols: symbols}, nil
}

// Reorder 按显式完整顺序重排分组。新列表必须恰好是现有成员的排列，
// 不允许夹带增删——防止把重排误用成覆盖。
func (e *Engine) Reorder(ctx context.Context, groupIdentifier string, symbols []string, dryRun bool) (*PlanResult, *WriteResult, error) {
	if len(symbols) == 0 {
		return nil, nil, &thsclient.APIError{Action: "stock.reorder", Msg: "股票列表不能为空"}
	}
	if err := e.ensureSession(ctx); err != nil {
		return nil, nil, err
	}
	entries, err := ParseSymbols(symbols)
	if err != nil {
		return nil, nil, err
	}

	if IsSelfIdentifier(groupIdentifier) {
		result, err := e.replaceSelfOrder(ctx, entries, dryRun)
		if err != nil {
			return nil, nil, err
		}
		if dryRun {
			return &PlanResult{Operation: "stock.reorder", Group: SelfGroupName, GroupID: SelfGroupID, Symbols: symbols, DryRun: true}, nil, nil
		}
		return nil, result, nil
	}

	meta, err := e.resolveGroup(ctx, groupIdentifier)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureWritable(meta); err != nil {
		return nil, nil, err
	}

	// 远端快照可能滞后，做有界重试直到成员集合与请求一致。
	var target *thsclient.BlockstockGroup
	var download *thsclient.BlockstockDownload
	for attempt := 1; attempt <= reorderSnapshotAttempts; attempt++ {
		download, err = e.api.BlockstockDownload(ctx, e.currentAuthParams())
		if err != nil {
			return nil, nil, err
		}
		for idx := range download.Groups {
			if download.Groups[idx].Name == meta.Name {
				target = &download.Groups[idx]
				break
			}
		}
		if target != nil && sameMultiset(target.Items, entries) {
			break
		}
		if attempt < reorderSnapshotAttempts {
			time.Sleep(snapshotRetryDelay)
		}
	}
	if target == nil {
		return nil, nil, &thsclient.APIError{Action: "重排分组", Msg: fmt.Sprintf("远端没有找到分组 %q", meta.Name)}
	}
	if !sameMultiset(target.Items, entries) {
		missing, extra := multisetDiff(target.Items, entries)
		return nil, nil, &thsclient.APIError{
			Action: "重排分组",
			Msg: fmt.Sprintf("远端分组已变化：请求新增 %d 个、缺少 %d 个。重排只允许调整顺序，请先 ths list -g %s 查看最新成员",
				missing, extra, meta.Name),
		}
	}
	if dryRun {
		return &PlanResult{Operation: "stock.reorder", Group: meta.Name, GroupID: meta.ID, Symbols: symbols, DryRun: true}, nil, nil
	}
	version, err := e.api.BlockstockUpload(ctx, e.currentAuthParams(), thsclient.BlockstockGroup{
		Name:      meta.Name,
		GroupType: target.GroupType,
		Items:     entries,
	}, fmt.Sprint(download.Version))
	if err != nil {
		return nil, nil, err
	}
	e.waitBlockstockApplied(ctx, version)
	return nil, &WriteResult{Operation: "stock.reorder", Group: meta.Name, Symbols: symbols, Version: fmt.Sprint(version)}, nil
}

// replaceSelfOrder 校验并上传「我的自选」的新顺序。
func (e *Engine) replaceSelfOrder(ctx context.Context, entries []thsclient.StockEntry, dryRun bool) (*WriteResult, error) {
	var current *thsclient.SelfStockList
	var err error
	for attempt := 1; attempt <= reorderSnapshotAttempts; attempt++ {
		current, err = e.api.DownloadSelfStocksV1(ctx)
		if err != nil {
			return nil, err
		}
		if sameMultiset(current.Items, entries) {
			break
		}
		if attempt < reorderSnapshotAttempts {
			time.Sleep(snapshotRetryDelay)
		}
	}
	if !sameMultiset(current.Items, entries) {
		missing, extra := multisetDiff(current.Items, entries)
		return nil, &thsclient.APIError{
			Action: "重排我的自选",
			Msg: fmt.Sprintf("远端我的自选已变化：请求新增 %d 个、缺少 %d 个。重排只允许调整顺序，请先 ths self list 查看最新成员",
				missing, extra),
		}
	}
	if dryRun {
		return &WriteResult{Operation: "stock.reorder", Group: SelfGroupName}, nil
	}
	version, err := e.api.ModifySelfStocksV1(ctx, entries, current.Version)
	if err != nil {
		return nil, err
	}
	return &WriteResult{Operation: "stock.reorder", Group: SelfGroupName, Version: version}, nil
}

// replaceGroupContent 对自定义分组执行「读-变换-写」，
// 写入后等待服务端异步应用生效（防止连续命令读到旧状态）。
func (e *Engine) replaceGroupContent(ctx context.Context, groupName string, transform func(current []thsclient.StockEntry) []thsclient.StockEntry) (uint64, error) {
	download, err := e.api.BlockstockDownload(ctx, e.currentAuthParams())
	if err != nil {
		return 0, err
	}
	var target *thsclient.BlockstockGroup
	for idx := range download.Groups {
		if download.Groups[idx].Name == groupName {
			target = &download.Groups[idx]
			break
		}
	}
	if target == nil {
		return 0, &thsclient.APIError{Action: "同步分组", Msg: fmt.Sprintf("远端没有找到分组 %q", groupName)}
	}
	newItems := transform(target.Items)
	version, err := e.api.BlockstockUpload(ctx, e.currentAuthParams(), thsclient.BlockstockGroup{
		Name:      groupName,
		GroupType: target.GroupType,
		Items:     newItems,
	}, fmt.Sprint(download.Version))
	if err != nil {
		return 0, err
	}
	e.waitBlockstockApplied(ctx, version)
	return version, nil
}

// GroupCreate 创建自定义分组（版本冲突自动重试一次）。
func (e *Engine) GroupCreate(ctx context.Context, name string, dryRun bool) (*PlanResult, *WriteResult, error) {
	if strings.TrimSpace(name) == "" {
		return nil, nil, &thsclient.APIError{Action: "group.create", Msg: "分组名称不能为空"}
	}
	if dryRun {
		return &PlanResult{Operation: "group.create", Group: name, DryRun: true}, nil, nil
	}
	if err := e.ensureSession(ctx); err != nil {
		return nil, nil, err
	}
	if err := e.withVersionRetry(ctx, "group.create", func(version string) error {
		return e.api.GroupCreate(ctx, name, version)
	}); err != nil {
		return nil, nil, err
	}
	return nil, &WriteResult{Operation: "group.create", Group: name}, nil
}

// GroupDelete 删除自定义分组。
func (e *Engine) GroupDelete(ctx context.Context, groupIdentifier string, dryRun bool) (*PlanResult, *WriteResult, error) {
	if err := e.ensureSession(ctx); err != nil {
		return nil, nil, err
	}
	if IsSelfIdentifier(groupIdentifier) {
		return nil, nil, &thsclient.APIError{Action: "group.delete", Msg: "「我的自选」是系统分组，不能删除"}
	}
	meta, err := e.resolveGroup(ctx, groupIdentifier)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureWritable(meta); err != nil {
		return nil, nil, err
	}
	if dryRun {
		return &PlanResult{Operation: "group.delete", Group: meta.Name, GroupID: meta.ID, DryRun: true, Note: "删除分组会同时删除组内股票"}, nil, nil
	}
	if err := e.withVersionRetry(ctx, "group.delete", func(version string) error {
		return e.api.GroupDelete(ctx, meta.ID, version)
	}); err != nil {
		return nil, nil, err
	}
	return nil, &WriteResult{Operation: "group.delete", Group: meta.Name}, nil
}

// GroupShare 生成分组分享链接。
func (e *Engine) GroupShare(ctx context.Context, groupIdentifier string, validSeconds int, dryRun bool) (*PlanResult, *WriteResult, error) {
	if err := e.ensureSession(ctx); err != nil {
		return nil, nil, err
	}
	meta, err := e.resolveGroup(ctx, groupIdentifier)
	if err != nil {
		return nil, nil, err
	}
	if dryRun {
		return &PlanResult{Operation: "group.share", Group: meta.Name, GroupID: meta.ID, DryRun: true}, nil, nil
	}
	userID := e.api.Cookies()["userid"]
	if userID == "" {
		return nil, nil, &thsclient.AuthError{Msg: "Cookie 中缺少 userid，请重新 ths login"}
	}
	shareURL, err := e.api.GroupShare(ctx, userID, meta.ID, meta.Name, validSeconds)
	if err != nil {
		return nil, nil, err
	}
	return nil, &WriteResult{Operation: "group.share", Group: meta.Name, ShareURL: shareURL}, nil
}

// withVersionRetry 执行带版本号的写操作；版本冲突时刷新版本重试一次。
func (e *Engine) withVersionRetry(ctx context.Context, action string, op func(version string) error) error {
	for attempt := 0; attempt < 2; attempt++ {
		_, version, err := e.api.QueryGroups(ctx)
		if err != nil {
			return err
		}
		err = op(version)
		if err == nil {
			return nil
		}
		if apiErr, ok := thsclient.AsAPIError(err); ok && IsVersionConflict(apiErr) && attempt == 0 {
			continue
		}
		return err
	}
	return &thsclient.APIError{Action: action, Msg: "版本冲突重试后仍失败，请稍后再试"}
}

// ParseSymbols 把 "600519.SH" 形式解析为协议条目。
func ParseSymbols(symbols []string) ([]thsclient.StockEntry, error) {
	entries := make([]thsclient.StockEntry, 0, len(symbols))
	for _, symbol := range symbols {
		code, marketAbbr, found := strings.Cut(symbol, ".")
		if !found || code == "" || marketAbbr == "" {
			return nil, &thsclient.APIError{
				Action: "解析股票代码",
				Msg:    fmt.Sprintf("股票代码 %q 需要 code.market 格式，例如 600519.SH（沪）、000001.SZ（深）、430047.BJ（北交所）", symbol),
			}
		}
		marketCode, err := market.Code(marketAbbr)
		if err != nil {
			return nil, &thsclient.APIError{Action: "解析股票代码", Msg: err.Error()}
		}
		entries = append(entries, thsclient.StockEntry{Code: code, MarketCode: marketCode})
	}
	return entries, nil
}
