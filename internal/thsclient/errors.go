// Package thsclient 实现同花顺自选股相关私有接口的 HTTP 客户端。
package thsclient

import (
	"errors"
	"fmt"
)

// 端点域名全部是同花顺官方（10jqka.com.cn），本工具不做任何第三方上传。
// 声明为 var 仅为了测试时指向本地 httptest 服务，线上取值不变。
var (
	AuthBase        = "https://auth.10jqka.com.cn"
	UpassBase       = "https://upass.10jqka.com.cn"
	UGCBASE         = "https://ugc.10jqka.com.cn"
	SelfStockV2Base = "https://t.10jqka.com.cn"
	MultiStorageURL = "https://cs.10jqka.com.cn/multiStorage"
	DynamicPlateURL = "https://apigate.10jqka.com.cn"
)

// 移动端 UA（数据接口）与 iOS 客户端 UA（登录流程），均为线上抓包事实。
const (
	MobileUA = "Hexin_Gphone/11.28.03 (Royal Flush) hxtheme/0 innerversion/G037.09.028.1.32 " +
		"followPhoneSystemTheme/0 userid/000000000 getHXAPPAccessibilityMode/0 " +
		"hxNewFont/1 isVip/0 getHXAPPFontSetting/normal getHXAPPAdaptOldSetting/0 okhttp/3.14.9"
	LoginUA = "同花顺/7.0.10 CFNetwork/1333.0.4 Darwin/21.5.0"
)

// 错误分类决定 CLI 退出码：
// NetError → 2（网络），APIError → 3（业务），AuthError → 4（登录态）。
// 分类依据是可恢复动作：网络重试、业务修正请求、登录态重新登录。

// NetError 表示网络层失败（超时、DNS、5xx、非预期响应格式）。
type NetError struct {
	Action  string
	Detail  string
	Timeout bool
}

func (e *NetError) Error() string {
	return fmt.Sprintf("%s 失败（网络）: %s", e.Action, e.Detail)
}

// APIError 表示同花顺明确返回的业务失败。
type APIError struct {
	Action string
	Msg    string
	Code   string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s 失败: %s (code=%s)", e.Action, e.Msg, e.Code)
	}
	return fmt.Sprintf("%s 失败: %s", e.Action, e.Msg)
}

// AuthError 表示登录态缺失或失效，补救动作是重新登录。
type AuthError struct {
	Msg string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("登录态问题: %s", e.Msg)
}

// ErrNoCredentials 表示本地没有可用凭据，补救提示由 CLI 层统一附加。
var ErrNoCredentials = &AuthError{Msg: "本地没有已保存的登录态"}

// AsNetError / AsAPIError / AsAuthError 供上层判断错误类别。
func AsNetError(err error) (*NetError, bool) {
	var netErr *NetError
	if errors.As(err, &netErr) {
		return netErr, true
	}
	return nil, false
}

// AsAPIError 提取业务错误。
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// AsAuthError 提取登录态错误。
func AsAuthError(err error) (*AuthError, bool) {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr, true
	}
	return nil, false
}
