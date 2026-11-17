// Package engine 是 CLI 与协议层之间的业务编排：
// 会话管理、分组解析、成员合并、只读保护与 dry-run 语义都集中在这里。
package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/whiletrue2009/ths-favorite-go/internal/creds"
	"github.com/whiletrue2009/ths-favorite-go/internal/session"
	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

// SelfGroupID 与 SelfGroupName 是「我的自选」虚拟分组的固定标识。
const (
	SelfGroupID   = "__selfstock__"
	SelfGroupName = "我的自选"
)

// THSApi 是引擎依赖的协议面。*thsclient.Client 天然满足。
type THSApi interface {
	QueryGroups(ctx context.Context) ([]thsclient.GroupMeta, string, error)
	DownloadSelfStocksV2(ctx context.Context) ([]thsclient.StockEntry, error)
	DownloadSelfStocksV1(ctx context.Context) (*thsclient.SelfStockList, error)
	ModifySelfStocksV1(ctx context.Context, entries []thsclient.StockEntry, version string) (string, error)
	BlockstockDownload(ctx context.Context, auth thsclient.AuthParams) (*thsclient.BlockstockDownload, error)
	BlockstockUpload(ctx context.Context, auth thsclient.AuthParams, group thsclient.BlockstockGroup, version string) (uint64, error)
	QueryDynamicPlate(ctx context.Context, groupName string) ([]thsclient.StockEntry, error)
	DownloadSelfStockDetail(ctx context.Context) (string, []thsclient.StockDetail, error)
	GroupCreate(ctx context.Context, name, version string) error
	GroupDelete(ctx context.Context, groupID, version string) error
	GroupShare(ctx context.Context, userID, groupID, groupName string, validSeconds int) (string, error)
	Cookies() map[string]string
	SetCookies(cookies map[string]string)
}

// CredentialStore 是引擎依赖的凭据存取面。*creds.Storage 天然满足。
type CredentialStore interface {
	Load() (*creds.Credentials, error)
	Save(cred *creds.Credentials) error
	Delete() error
}

// Engine 持有协议客户端与本地状态。
type Engine struct {
	api       THSApi
	store     *session.Store
	creds     CredentialStore
	loginFunc func(ctx context.Context, username, password string) (*thsclient.SessionResult, error)
	// applyWait 允许测试注入短的写入生效等待；零值用默认配置。
	applyWait applyWaitConfig
}

// New 构造引擎。
func New(api THSApi, store *session.Store, credStore CredentialStore) *Engine {
	return &Engine{api: api, store: store, creds: credStore, loginFunc: thsclient.Login}
}

// SetLoginFunc 注入登录实现（测试用）。
func (e *Engine) SetLoginFunc(fn func(ctx context.Context, username, password string) (*thsclient.SessionResult, error)) {
	e.loginFunc = fn
}

// currentAuthParams 按线上协议从当前 Cookie 推导 multiStorage 鉴权参数：
// sessionid 取 user Cookie 解码后的第 18 段（写操作以此为准，
// 登录流程返回的 sessionid 对 multiStorage 写入无效）。
func (e *Engine) currentAuthParams() thsclient.AuthParams {
	return thsclient.AuthParamsFromCookies(e.api.Cookies(), "", "")
}

// ensureSession 保证后续调用带着有效登录态：
// 1. 本地 session 存在且未过期 → 装载；
// 2. 缺失、损坏或过期但保存过凭据 → 自动重新登录；
// 3. 都没有 → ErrNoCredentials。
func (e *Engine) ensureSession(ctx context.Context) error {
	sess, err := e.store.Load()
	if err == nil && !sess.Expired() {
		e.api.SetCookies(sess.Cookies)
		return nil
	}
	cred, credErr := e.creds.Load()
	if credErr != nil {
		if errors.Is(credErr, creds.ErrNotFound) {
			return thsclient.ErrNoCredentials
		}
		return fmt.Errorf("读取账号凭据失败: %w", credErr)
	}
	_, loginErr := e.Login(ctx, cred.Username, cred.Password, false)
	return loginErr
}

// Login 执行登录并持久化登录态；persist=true 时同时保存账号凭据。
func (e *Engine) Login(ctx context.Context, username, password string, persist bool) (*thsclient.SessionResult, error) {
	if username == "" || password == "" {
		return nil, &thsclient.APIError{Action: "登录", Msg: "账号和密码都不能为空"}
	}
	result, err := e.loginFunc(ctx, username, password)
	if err != nil {
		return nil, err
	}
	if persist {
		if err := e.creds.Save(&creds.Credentials{Username: username, Password: password}); err != nil {
			return nil, fmt.Errorf("登录成功但保存凭据失败: %w", err)
		}
	}
	sess := &session.Session{
		Cookies:    result.Cookies,
		AuthParams: result.AuthParams,
		Username:   username,
	}
	if err := e.store.Save(sess); err != nil {
		return nil, fmt.Errorf("登录成功但保存登录态失败: %w", err)
	}
	e.api.SetCookies(result.Cookies)
	return result, nil
}

// Logout 清空登录态与凭据。
func (e *Engine) Logout() error {
	credErr := e.creds.Delete()
	sessErr := e.store.Clear()
	if credErr != nil {
		return credErr
	}
	return sessErr
}

// HasCredentials 报告本地是否保存了凭据（doctor 用）。
func (e *Engine) HasCredentials() bool {
	_, err := e.creds.Load()
	return err == nil
}

// SaveSessionForTest 直接写入登录态（测试辅助）。
func (e *Engine) SaveSessionForTest(sess *session.Session) error {
	return e.store.Save(sess)
}

// GroupCount 返回远端分组数量（登录后的轻量验证调用）。
func (e *Engine) GroupCount(ctx context.Context) (int, error) {
	if err := e.ensureSession(ctx); err != nil {
		return 0, err
	}
	metas, _, err := e.api.QueryGroups(ctx)
	if err != nil {
		return 0, err
	}
	return len(metas), nil
}
