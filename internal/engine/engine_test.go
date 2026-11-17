package engine

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/whiletrue2009/ths-favorite-go/internal/creds"
	"github.com/whiletrue2009/ths-favorite-go/internal/session"
	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

// fakeAPI 记录调用并返回预设数据。
type fakeAPI struct {
	queryGroups     []thsclient.GroupMeta
	queryVersion    string
	queryErr        error
	selfV2          []thsclient.StockEntry
	selfV1          *thsclient.SelfStockList
	download        *thsclient.BlockstockDownload
	uploadedGroup   *thsclient.BlockstockGroup
	uploadedVersion string
	detail          []thsclient.StockDetail
	cookies         map[string]string

	modifyCalls int
	uploadCalls int
	queryCalls  int
}

func (f *fakeAPI) QueryGroups(_ context.Context) ([]thsclient.GroupMeta, string, error) {
	f.queryCalls++
	return f.queryGroups, f.queryVersion, f.queryErr
}

func (f *fakeAPI) DownloadSelfStocksV2(_ context.Context) ([]thsclient.StockEntry, error) {
	return f.selfV2, nil
}

func (f *fakeAPI) DownloadSelfStocksV1(_ context.Context) (*thsclient.SelfStockList, error) {
	return f.selfV1, nil
}

func (f *fakeAPI) ModifySelfStocksV1(_ context.Context, entries []thsclient.StockEntry, _ string) (string, error) {
	f.modifyCalls++
	f.selfV1 = &thsclient.SelfStockList{Version: "v2", Items: entries}
	return "v2", nil
}

func (f *fakeAPI) BlockstockDownload(_ context.Context, _ thsclient.AuthParams) (*thsclient.BlockstockDownload, error) {
	return f.download, nil
}

func (f *fakeAPI) BlockstockUpload(_ context.Context, _ thsclient.AuthParams, group thsclient.BlockstockGroup, version string) (uint64, error) {
	f.uploadCalls++
	copied := group
	f.uploadedGroup = &copied
	f.uploadedVersion = version
	// 模拟真实服务端：应用后读取路径立即可见新版本。
	if f.download != nil {
		f.download.Version = 99
	}
	return 99, nil
}

func (f *fakeAPI) QueryDynamicPlate(_ context.Context, _ string) ([]thsclient.StockEntry, error) {
	return nil, nil
}

func (f *fakeAPI) DownloadSelfStockDetail(_ context.Context) (string, []thsclient.StockDetail, error) {
	return "", f.detail, nil
}

func (f *fakeAPI) GroupCreate(_ context.Context, _ string, _ string) error { return nil }

func (f *fakeAPI) GroupDelete(_ context.Context, _ string, _ string) error { return nil }

func (f *fakeAPI) GroupShare(_ context.Context, _, _, _ string, _ int) (string, error) {
	return "https://t.10jqka.com.cn/share/abc", nil
}

func (f *fakeAPI) Cookies() map[string]string { return f.cookies }

func (f *fakeAPI) SetCookies(cookies map[string]string) { f.cookies = cookies }

// memoryCreds 是凭据接口的内存实现。
type memoryCreds struct {
	cred *creds.Credentials
}

func (m *memoryCreds) Load() (*creds.Credentials, error) {
	if m.cred == nil {
		return nil, creds.ErrNotFound
	}
	return m.cred, nil
}

func (m *memoryCreds) Save(cred *creds.Credentials) error {
	m.cred = cred
	return nil
}

func (m *memoryCreds) Delete() error {
	m.cred = nil
	return nil
}

func newTestEngine(t *testing.T, api *fakeAPI) *Engine {
	t.Helper()
	origDelay := snapshotRetryDelay
	snapshotRetryDelay = time.Millisecond
	t.Cleanup(func() { snapshotRetryDelay = origDelay })
	store, err := session.NewStore(filepath.Join(t.TempDir(), "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	return New(api, store, &memoryCreds{})
}

// loggedIn 把引擎置为已登录状态（保存一个新鲜 session）。
func loggedIn(t *testing.T, e *Engine) {
	t.Helper()
	sess := &session.Session{
		Cookies:    map[string]string{"userid": "123", "user": "dXNlcg=="},
		AuthParams: thsclient.AuthParams{UserID: "123", SessionID: "s", Expires: "2100-01-01 00:00:00"},
	}
	if err := e.SaveSessionForTest(sess); err != nil {
		t.Fatal(err)
	}
}

// sampleGroups 模拟 QueryGroups 的输出形态：Content 已解析为 Items。
var sampleGroups = []thsclient.GroupMeta{
	{ID: "0_1", Name: "消费", Type: "0", Content: "600519|000858,17|33",
		Items: []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}, {Code: "000858", MarketCode: "33"}}},
	{ID: "0_2", Name: "科技", Type: "0", Content: "002415,33",
		Items: []thsclient.StockEntry{{Code: "002415", MarketCode: "33"}}},
	{ID: "1_9", Name: "融资融券", Type: "1", Content: ""},
}

func TestMergeAdd(t *testing.T) {
	current := []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}, {Code: "000858", MarketCode: "33"}}
	add := []thsclient.StockEntry{{Code: "000858", MarketCode: "33"}, {Code: "300750", MarketCode: "33"}}
	got := mergeAdd(current, add)
	if len(got) != 3 {
		t.Fatalf("去重后应为 3 个: %+v", got)
	}
	if got[1].Code != "000858" || got[2].Code != "300750" {
		t.Errorf("合并结果不符: %+v", got)
	}
}

func TestMergeDelete(t *testing.T) {
	current := []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}, {Code: "000858", MarketCode: "33"}}
	got := mergeDelete(current, []thsclient.StockEntry{{Code: "600519"}})
	if len(got) != 1 || got[0].Code != "000858" {
		t.Errorf("删除结果不符: %+v", got)
	}
}

func TestSameMultisetAndDiff(t *testing.T) {
	a := []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}, {Code: "000858", MarketCode: "33"}}
	b := []thsclient.StockEntry{{Code: "000858", MarketCode: "33"}, {Code: "600519", MarketCode: "17"}}
	if !sameMultiset(a, b) {
		t.Error("相同成员不同顺序应判定相等")
	}
	c := []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}, {Code: "300750", MarketCode: "33"}}
	if sameMultiset(a, c) {
		t.Error("不同成员应判定不等")
	}
	missing, extra := multisetDiff(a, c)
	if missing != 1 || extra != 1 {
		t.Errorf("diff 不符: missing=%d extra=%d", missing, extra)
	}
}

func TestParseSymbols(t *testing.T) {
	entries, err := ParseSymbols([]string{"600519.SH", "000001.sz", "430047.BJ"})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	want := []string{"17", "33", "151"}
	for i, entry := range entries {
		if entry.MarketCode != want[i] {
			t.Errorf("条目 %d 市场码 = %s, want %s", i, entry.MarketCode, want[i])
		}
	}
	if _, err := ParseSymbols([]string{"600519"}); err == nil {
		t.Error("缺市场后缀应报错")
	}
	if _, err := ParseSymbols([]string{"600519.XX"}); err == nil {
		t.Error("未知市场应报错")
	}
}

func TestIsVersionConflict(t *testing.T) {
	yes := []thsclient.APIError{
		{Code: "409"},
		{Code: "version_conflict"},
		{Msg: "Version is outdated, please refresh"},
		{Msg: "版本已过期"},
		{Msg: "数据版本不一致，请刷新"},
	}
	for _, tc := range yes {
		errCopy := tc
		if !IsVersionConflict(&errCopy) {
			t.Errorf("%+v 应识别为版本冲突", tc)
		}
	}
	no := []thsclient.APIError{
		{Code: "500", Msg: "server error"},
		{Msg: "分组不存在"},
		{Msg: "refresh token invalid"}, // 无 version 前缀
	}
	for _, tc := range no {
		errCopy := tc
		if IsVersionConflict(&errCopy) {
			t.Errorf("%+v 不应识别为版本冲突", tc)
		}
	}
}

func TestStockAddCustomGroup(t *testing.T) {
	api := &fakeAPI{
		queryGroups: sampleGroups,
		download: &thsclient.BlockstockDownload{Version: 7, Groups: []thsclient.BlockstockGroup{
			{Name: "消费", GroupType: 0, Items: []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}}},
		}},
	}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	plan, result, err := e.StockAdd(context.Background(), "消费", []string{"000858.SZ"}, false)
	if err != nil {
		t.Fatalf("StockAdd 失败: %v", err)
	}
	if plan != nil || result == nil {
		t.Fatalf("非 dry-run 应返回 WriteResult")
	}
	if api.uploadCalls != 1 {
		t.Fatalf("应恰好上传一次: %d", api.uploadCalls)
	}
	uploaded := api.uploadedGroup
	if uploaded.Name != "消费" || len(uploaded.Items) != 2 {
		t.Errorf("上传内容不符: %+v", uploaded)
	}
	if uploaded.Items[0].Code != "600519" || uploaded.Items[1].Code != "000858" {
		t.Errorf("合并顺序不符: %+v", uploaded.Items)
	}
}

func TestStockAddDryRunNoMutation(t *testing.T) {
	api := &fakeAPI{queryGroups: sampleGroups}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	plan, result, err := e.StockAdd(context.Background(), "消费", []string{"000858.SZ"}, true)
	if err != nil {
		t.Fatalf("dry-run 失败: %v", err)
	}
	if result != nil || plan == nil || !plan.DryRun {
		t.Fatalf("dry-run 应返回 PlanResult: %+v", plan)
	}
	if api.uploadCalls != 0 || api.modifyCalls != 0 {
		t.Error("dry-run 不应产生任何远端写操作")
	}
}

func TestStockAddSelfStock(t *testing.T) {
	api := &fakeAPI{
		selfV1: &thsclient.SelfStockList{Version: "v1", Items: []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}}},
	}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	_, result, err := e.StockAdd(context.Background(), "我的自选", []string{"000858.SZ"}, false)
	if err != nil {
		t.Fatalf("StockAdd 失败: %v", err)
	}
	if api.modifyCalls != 1 || len(api.selfV1.Items) != 2 {
		t.Fatalf("我的自选应走 v1 全量替换: %+v", api.selfV1)
	}
	if result.Group != SelfGroupName {
		t.Errorf("结果分组不符: %+v", result)
	}
}

func TestStockAddDynamicGroupRejected(t *testing.T) {
	api := &fakeAPI{queryGroups: sampleGroups}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	_, _, err := e.StockAdd(context.Background(), "融资融券", []string{"600519.SH"}, false)
	if err == nil {
		t.Fatal("动态分组应拒绝写入")
	}
	apiErr, ok := thsclient.AsAPIError(err)
	if !ok || !strings.Contains(apiErr.Msg, "只读") {
		t.Errorf("错误语义不符: %v", err)
	}
	if api.uploadCalls != 0 {
		t.Error("拒绝路径不应有远端写操作")
	}
}

func TestStockAddUnknownGroup(t *testing.T) {
	api := &fakeAPI{queryGroups: sampleGroups}
	e := newTestEngine(t, api)
	loggedIn(t, e)
	_, _, err := e.StockAdd(context.Background(), "不存在的组", []string{"600519.SH"}, false)
	if err == nil {
		t.Fatal("未知分组应报错")
	}
}

func TestReorderCustomGroup(t *testing.T) {
	items := []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}, {Code: "000858", MarketCode: "33"}}
	api := &fakeAPI{
		queryGroups: sampleGroups,
		download: &thsclient.BlockstockDownload{Version: 5, Groups: []thsclient.BlockstockGroup{
			{Name: "消费", GroupType: 0, Items: items},
		}},
	}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	_, result, err := e.Reorder(context.Background(), "消费", []string{"000858.SZ", "600519.SH"}, false)
	if err != nil {
		t.Fatalf("Reorder 失败: %v", err)
	}
	if result.Version != "99" {
		t.Errorf("应返回上传后的新版本号: %+v", result)
	}
	if api.uploadedGroup.Items[0].Code != "000858" || api.uploadedGroup.Items[1].Code != "600519" {
		t.Errorf("顺序不符: %+v", api.uploadedGroup.Items)
	}
}

func TestReorderRejectsMemberChange(t *testing.T) {
	api := &fakeAPI{
		queryGroups: sampleGroups,
		download: &thsclient.BlockstockDownload{Version: 5, Groups: []thsclient.BlockstockGroup{
			{Name: "消费", GroupType: 0, Items: []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}}},
		}},
	}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	_, _, err := e.Reorder(context.Background(), "消费", []string{"600519.SH", "300750.SZ"}, false)
	if err == nil {
		t.Fatal("夹带新增成员的重排应被拒绝")
	}
	if api.uploadCalls != 0 {
		t.Error("拒绝路径不应上传")
	}
}

func TestEnsureSessionAutoRelogin(t *testing.T) {
	api := &fakeAPI{}
	e := newTestEngine(t, api)
	e.SetLoginFunc(func(_ context.Context, username, password string) (*thsclient.SessionResult, error) {
		if username != "13800000000" || password != "pw" {
			return nil, errors.New("凭据不符")
		}
		return &thsclient.SessionResult{
			UserID:     "u1",
			SessionID:  "s1",
			SignValid:  "v",
			Cookies:    map[string]string{"userid": "u1"},
			AuthParams: thsclient.AuthParams{UserID: "u1", SessionID: "s1", Expires: "2100-01-01 00:00:00"},
		}, nil
	})
	// 没有凭据 → ErrNoCredentials
	_, _, err := e.StockAdd(context.Background(), "我的自选", []string{"600519.SH"}, false)
	if !errors.Is(err, thsclient.ErrNoCredentials) {
		t.Fatalf("无凭据应返回 ErrNoCredentials: %v", err)
	}
	// 保存凭据后应自动登录并继续
	if err := e.creds.Save(&creds.Credentials{Username: "13800000000", Password: "pw"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.GroupCreate(context.Background(), "新组", false); err != nil {
		t.Fatalf("自动重登后操作应成功: %v", err)
	}
}

func TestListGroupsEnrichDetail(t *testing.T) {
	price := 1234.5
	api := &fakeAPI{
		queryGroups: sampleGroups,
		selfV2:      []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}},
		detail:      []thsclient.StockDetail{{Code: "600519", Market: "SH", Price: &price, AddedAt: "20231101"}},
	}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	views, err := e.ListGroups(context.Background(), ListOptions{IncludeSelf: true})
	if err != nil {
		t.Fatalf("ListGroups 失败: %v", err)
	}
	var self *GroupView
	for i := range views {
		if views[i].GroupID == SelfGroupID {
			self = &views[i]
		}
	}
	if self == nil {
		t.Fatal("应包含我的自选")
	}
	if len(self.Items) != 1 || self.Items[0].Price == nil || *self.Items[0].Price != price {
		t.Fatalf("明细未附加: %+v", self.Items)
	}
	if self.Items[0].AddedAt != "20231101" {
		t.Errorf("加入时间未附加: %+v", self.Items[0])
	}
}

func TestListSingleDynamicGroup(t *testing.T) {
	api := &fakeAPI{queryGroups: sampleGroups}
	api.queryGroups = []thsclient.GroupMeta{{ID: "1_9", Name: "融资融券", Type: "1"}}
	// QueryDynamicPlate 返回 nil, nil → 空成员
	e := newTestEngine(t, api)
	loggedIn(t, e)
	views, err := e.ListGroups(context.Background(), ListOptions{Group: "融资融券"})
	if err != nil {
		t.Fatalf("动态分组查询失败: %v", err)
	}
	if len(views) != 1 || !views[0].Readonly {
		t.Fatalf("动态分组应为只读: %+v", views)
	}
}
