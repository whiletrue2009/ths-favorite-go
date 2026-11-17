package thsclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/whiletrue2009/ths-favorite-go/internal/thsxml"
)

// 登录流程的线上抓包常量。
const (
	loginTaAppID = "2022021114090152"
	loginQSID    = "8003"
	loginProduct = "S01"
	loginVersion = "11.4.1.3"
	loginIMEI    = "ZjI6MDY6NGE6NzI6MjQ6NTA="
	loginSDSN    = ""
	// URL 编码后的产品名「同花顺远航版」。
	loginSecurities    = "%E5%90%8C%E8%8A%B1%E9%A1%BA%E8%BF%9C%E8%88%AA%E7%89%88"
	rsaVersionFallback = "default_5"
)

// SessionResult 是一次完整登录的产物。
type SessionResult struct {
	UserID     string
	SessionID  string
	SignValid  string
	Cookies    map[string]string
	AuthParams AuthParams
}

// AuthParams 是 multiStorage 协议（分组批量同步）需要的鉴权参数。
type AuthParams struct {
	UserID    string
	SessionID string
	Expires   string
}

// Login 执行官方移动端四步登录：取 RSA 公钥 → 统一登录 → mainverify → 换 Cookie。
// 账号与密码在本地用官方公钥做 RSA 加密后才上行。
func Login(ctx context.Context, username, password string) (*SessionResult, error) {
	steps := newLoginSteps()
	pubkey, rsaVersion, err := steps.fetchPubKey(ctx)
	if err != nil {
		return nil, err
	}
	accountCipher, err := rsaEncrypt(pubkey, username)
	if err != nil {
		return nil, err
	}
	passwordCipher, err := rsaEncrypt(pubkey, password)
	if err != nil {
		return nil, err
	}
	userID, sessionID, rsaVersion, err := steps.unifiedLogin(ctx, accountCipher, passwordCipher, rsaVersion)
	if err != nil {
		return nil, err
	}
	signValid, err := steps.mainVerify(ctx, userID, sessionID, rsaVersion)
	if err != nil {
		return nil, err
	}
	cookies, err := steps.fetchCookies(ctx, userID, sessionID, signValid)
	if err != nil {
		return nil, err
	}
	result := &SessionResult{
		UserID:    userID,
		SessionID: sessionID,
		SignValid: signValid,
		Cookies:   cookies,
	}
	result.AuthParams = AuthParamsFromCookies(cookies, userID, sessionID)
	return result, nil
}

// loginSteps 聚合登录各步，便于测试替换。
type loginSteps struct {
	http *Client
}

func newLoginSteps() *loginSteps {
	return &loginSteps{http: NewClient()}
}

// fetchPubKey 第一步：获取 RSA 公钥。
func (s *loginSteps) fetchPubKey(ctx context.Context) (string, string, error) {
	query := url.Values{"reqtype": {"do_rsa"}, "type": {"get_pubkey"}}
	body, err := s.http.Get(ctx, "获取公钥", AuthBase+"/verify2", query, map[string]string{"User-Agent": LoginUA})
	if err != nil {
		return "", "", err
	}
	resp, err := thsxml.Parse(body)
	if err != nil {
		return "", "", wrapStepError("获取公钥", err)
	}
	pubkey := resp.Item.Attr("pubkey")
	if pubkey == "" {
		return "", "", &APIError{Action: "获取公钥", Msg: "响应缺少 pubkey 属性"}
	}
	version := resp.Item.Attr("rsa_version")
	if version == "" {
		version = rsaVersionFallback
	}
	return pubkey, version, nil
}

// unifiedLogin 第二步：账号密码（已加密）换取 userid/sessionid。
func (s *loginSteps) unifiedLogin(ctx context.Context, accountCipher, passwordCipher, rsaVersion string) (string, string, string, error) {
	query := url.Values{
		"account":     {accountCipher},
		"msg":         {"1"},
		"passwd":      {passwordCipher},
		"reqtype":     {"unified_login"},
		"rsa_version": {rsaVersion},
		"ta_appid":    {loginTaAppID},
	}
	body, err := s.http.Get(ctx, "登录", AuthBase+"/verify2", query, map[string]string{"User-Agent": LoginUA})
	if err != nil {
		return "", "", "", err
	}
	resp, err := thsxml.Parse(body)
	if err != nil {
		return "", "", "", wrapStepError("登录", err)
	}
	userID := resp.Item.Attr("userid")
	sessionID := resp.Item.Attr("sessionid")
	if userID == "" || sessionID == "" {
		return "", "", "", &APIError{Action: "登录", Msg: "响应缺少 userid/sessionid"}
	}
	version := resp.Item.Attr("rsa_version")
	if version == "" {
		version = rsaVersion
	}
	return userID, sessionID, version, nil
}

// mainVerify 第三步：换取 signvalid。
func (s *loginSteps) mainVerify(ctx context.Context, userID, sessionID, rsaVersion string) (string, error) {
	query := url.Values{
		"reqtype":     {"mainverify"},
		"userid":      {userID},
		"sessionid":   {sessionID},
		"qsid":        {loginQSID},
		"product":     {loginProduct},
		"version":     {loginVersion},
		"imei":        {loginIMEI},
		"sdsn":        {loginSDSN},
		"rsa_version": {rsaVersion},
		"nohqlist":    {"0"},
		"securities":  {loginSecurities},
	}
	body, err := s.http.Get(ctx, "Mainverify", AuthBase+"/verify2", query, map[string]string{"User-Agent": LoginUA})
	if err != nil {
		return "", err
	}
	resp, err := thsxml.Parse(body)
	if err != nil {
		return "", wrapStepError("Mainverify", err)
	}
	passport := resp.Item.Attr("passport")
	if passport == "" {
		return "", &APIError{Action: "Mainverify", Msg: "响应缺少 passport 数据"}
	}
	for _, chunk := range strings.Split(passport, "|") {
		key, value, found := strings.Cut(chunk, "=")
		if found && strings.TrimSpace(key) == "signvalid" {
			return strings.TrimSpace(value), nil
		}
	}
	return "", &APIError{Action: "Mainverify", Msg: "passport 中缺少 signvalid"}
}

// fetchCookies 第四步：userid+sessionid+signvalid 换正式 Cookie。
func (s *loginSteps) fetchCookies(ctx context.Context, userID, sessionID, signValid string) (map[string]string, error) {
	query := url.Values{"userid": {userID}, "sessionid": {sessionID}, "signvalid": {signValid}}
	// 这里需要读 Set-Cookie，直接用底层 http 而非 c.do。
	req, err := newRequestWithQuery(ctx, UpassBase+"/docookie2.php", query)
	if err != nil {
		return nil, &NetError{Action: "换取Cookie", Detail: err.Error()}
	}
	req.Header.Set("User-Agent", LoginUA)
	resp, err := s.http.http.Do(req)
	if err != nil {
		return nil, &NetError{Action: "换取Cookie", Detail: err.Error()}
	}
	defer resp.Body.Close()
	cookies := map[string]string{}
	for _, cookie := range resp.Cookies() {
		cookies[cookie.Name] = cookie.Value
	}
	if len(cookies) == 0 {
		return nil, &APIError{Action: "换取Cookie", Msg: "docookie2.php 未返回任何 Cookie"}
	}
	return cookies, nil
}

// rsaEncrypt 用服务端 PEM 公钥做 PKCS1v15 加密并 base64。
// 兼容 SPKI（BEGIN PUBLIC KEY）与 PKCS#1（BEGIN RSA PUBLIC KEY）两种格式。
func rsaEncrypt(pubkeyPEM, plaintext string) (string, error) {
	block, _ := pem.Decode([]byte(pubkeyPEM))
	if block == nil {
		return "", &APIError{Action: "登录", Msg: "服务端返回的公钥不是有效 PEM 格式"}
	}
	var rsaKey *rsa.PublicKey
	switch block.Type {
	case "RSA PUBLIC KEY":
		parsed, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("解析 PKCS#1 公钥失败: %w", err)
		}
		rsaKey = parsed
	default:
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("解析公钥失败: %w", err)
		}
		key, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return "", &APIError{Action: "登录", Msg: "服务端公钥类型不是 RSA"}
		}
		rsaKey = key
	}
	cipher, err := rsa.EncryptPKCS1v15(rand.Reader, rsaKey, []byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("加密账号数据失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(cipher), nil
}

// AuthParamsFromCookies 从 Cookie 推导 multiStorage 鉴权参数。
// user Cookie 结构：URL 转义的 base64，解码后以 ':' 分段，第 18 段（下标 17）是 sessionid。
func AuthParamsFromCookies(cookies map[string]string, userID, sessionID string) AuthParams {
	if sessionID == "" {
		if userRaw := cookies["user"]; userRaw != "" {
			if decoded, err := url.QueryUnescape(userRaw); err == nil {
				if raw, err := base64.StdEncoding.DecodeString(decoded); err == nil {
					parts := strings.Split(string(raw), ":")
					if len(parts) > 17 {
						sessionID = parts[17]
					}
				}
			}
		}
	}
	if userID == "" {
		userID = cookies["userid"]
	}
	return AuthParams{
		UserID:    userID,
		SessionID: sessionID,
		Expires:   time.Now().Add(24 * time.Hour).Format("2006-01-02 15:04:05"),
	}
}

// wrapStepError 把 XML 解析错误归类为业务错误。
func wrapStepError(action string, err error) error {
	var apiFailure *thsxml.APIFailure
	if ok := asAPIFailure(err, &apiFailure); ok {
		return &APIError{Action: action, Msg: apiFailure.Msg, Code: apiFailure.Code}
	}
	return &NetError{Action: action, Detail: err.Error()}
}

func asAPIFailure(err error, target **thsxml.APIFailure) bool {
	if failure, ok := err.(*thsxml.APIFailure); ok {
		*target = failure
		return true
	}
	return false
}

// newRequestWithQuery 构造带查询串的 GET 请求。
func newRequestWithQuery(ctx context.Context, rawURL string, query url.Values) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	return req, nil
}
