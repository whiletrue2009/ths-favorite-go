package thsclient

import (
	"testing"
)

func TestAuthParamsFromUserCookieFallback(t *testing.T) {
	// sessionID 为空时，从 user Cookie 第 18 段提取。
	cookies := map[string]string{"userid": "u9", "user": buildUserCookie("from-cookie")}
	params := AuthParamsFromCookies(cookies, "", "")
	if params.UserID != "u9" {
		t.Errorf("UserID 应取 Cookie userid: %+v", params)
	}
	if params.SessionID != "from-cookie" {
		t.Errorf("SessionID 应从 user Cookie 提取: %+v", params)
	}
}

func TestAuthParamsPrefersExplicitSession(t *testing.T) {
	cookies := map[string]string{"userid": "u9", "user": buildUserCookie("from-cookie")}
	params := AuthParamsFromCookies(cookies, "u1", "explicit-sess")
	if params.UserID != "u1" || params.SessionID != "explicit-sess" {
		t.Errorf("显式参数应优先: %+v", params)
	}
}

func TestAuthParamsBadUserCookie(t *testing.T) {
	// 坏 Cookie 不应 panic，sessionid 留空。
	params := AuthParamsFromCookies(map[string]string{"user": "!!!not-b64!!!"}, "", "")
	if params.SessionID != "" {
		t.Errorf("坏 Cookie 应得到空 sessionid: %+v", params)
	}
}
