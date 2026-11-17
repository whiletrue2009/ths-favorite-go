package engine

import (
	"context"
	"testing"

	"github.com/whiletrue2009/ths-favorite-go/internal/creds"
	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

func TestGroupShare(t *testing.T) {
	api := &fakeAPI{queryGroups: sampleGroups}
	api.cookies = map[string]string{"userid": "u123"}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	_, result, err := e.GroupShare(context.Background(), "消费", 604800, false)
	if err != nil {
		t.Fatalf("GroupShare 失败: %v", err)
	}
	if result.ShareURL == "" {
		t.Errorf("应返回分享链接: %+v", result)
	}

	plan, _, err := e.GroupShare(context.Background(), "消费", 604800, true)
	if err != nil || plan == nil || !plan.DryRun {
		t.Fatalf("dry-run 不符: %+v %v", plan, err)
	}
}

func TestGroupDeleteSelfRejected(t *testing.T) {
	api := &fakeAPI{queryGroups: sampleGroups}
	e := newTestEngine(t, api)
	loggedIn(t, e)
	if _, _, err := e.GroupDelete(context.Background(), SelfGroupName, false); err == nil {
		t.Fatal("删除我的自选应被拒绝")
	}
}

func TestGroupDelete(t *testing.T) {
	api := &fakeAPI{queryGroups: sampleGroups}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	plan, _, err := e.GroupDelete(context.Background(), "消费", true)
	if err != nil || plan == nil {
		t.Fatalf("dry-run 失败: %+v %v", plan, err)
	}
	_, result, err := e.GroupDelete(context.Background(), "0_2", false)
	if err != nil {
		t.Fatalf("GroupDelete 失败: %v", err)
	}
	if result.Group != "科技" {
		t.Errorf("按 ID 删除应解析出名称: %+v", result)
	}
}

func TestGroupCreateEmptyName(t *testing.T) {
	e := newTestEngine(t, &fakeAPI{})
	if _, _, err := e.GroupCreate(context.Background(), "  ", false); err == nil {
		t.Fatal("空名应被拒绝")
	}
}

func TestStockDelSelfStock(t *testing.T) {
	api := &fakeAPI{
		selfV1: &thsclient.SelfStockList{
			Version: "v1",
			Items:   []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}, {Code: "000858", MarketCode: "33"}},
		},
	}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	_, _, err := e.StockDel(context.Background(), "__selfstock__", []string{"600519.SH"}, false)
	if err != nil {
		t.Fatalf("StockDel 失败: %v", err)
	}
	if len(api.selfV1.Items) != 1 || api.selfV1.Items[0].Code != "000858" {
		t.Fatalf("删除结果不符: %+v", api.selfV1.Items)
	}
}

func TestReorderSelfStock(t *testing.T) {
	api := &fakeAPI{
		selfV1: &thsclient.SelfStockList{
			Version: "v1",
			Items:   []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}, {Code: "000858", MarketCode: "33"}},
		},
	}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	_, result, err := e.Reorder(context.Background(), "我的自选", []string{"000858.SZ", "600519.SH"}, false)
	if err != nil {
		t.Fatalf("Reorder 失败: %v", err)
	}
	if api.modifyCalls != 1 || api.selfV1.Items[0].Code != "000858" {
		t.Fatalf("重排结果不符: %+v", api.selfV1.Items)
	}
	if result.Version != "v2" {
		t.Errorf("版本号不符: %+v", result)
	}

	// 夹带新增应被拒绝
	api2 := &fakeAPI{
		selfV1: &thsclient.SelfStockList{Version: "v1", Items: []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}}},
	}
	e2 := newTestEngine(t, api2)
	loggedIn(t, e2)
	_, _, err = e2.Reorder(context.Background(), "我的自选", []string{"600519.SH", "300750.SZ"}, false)
	if err == nil || api2.modifyCalls != 0 {
		t.Fatalf("夹带新增的重排应被拒绝且不写远端: %v", err)
	}
}

func TestListSingleGroupByName(t *testing.T) {
	api := &fakeAPI{queryGroups: sampleGroups}
	e := newTestEngine(t, api)
	loggedIn(t, e)

	views, err := e.ListGroups(context.Background(), ListOptions{Group: "0_1"})
	if err != nil {
		t.Fatalf("按 ID 查询单组失败: %v", err)
	}
	if len(views) != 1 || views[0].Name != "消费" || len(views[0].Items) != 2 {
		t.Fatalf("单组结果不符: %+v", views)
	}
}

func TestSelfList(t *testing.T) {
	api := &fakeAPI{selfV2: []thsclient.StockEntry{{Code: "600519", MarketCode: "17"}}}
	e := newTestEngine(t, api)
	loggedIn(t, e)
	view, err := e.SelfList(context.Background())
	if err != nil {
		t.Fatalf("SelfList 失败: %v", err)
	}
	if view.GroupID != SelfGroupID || len(view.Items) != 1 || view.Items[0].Market != "SH" {
		t.Fatalf("结果不符: %+v", view)
	}
}

func TestLogout(t *testing.T) {
	api := &fakeAPI{}
	e := newTestEngine(t, api)
	loggedIn(t, e)
	if err := e.creds.Save(&creds.Credentials{Username: "u", Password: "p"}); err != nil {
		t.Fatal(err)
	}
	if !e.HasCredentials() {
		t.Fatal("保存后应有凭据")
	}
	if err := e.Logout(); err != nil {
		t.Fatalf("Logout 失败: %v", err)
	}
	if e.HasCredentials() {
		t.Error("登出后不应有凭据")
	}
}
