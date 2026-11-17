package thsclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testServer 在本地起一个模拟同花顺全链路的服务，覆盖登录四步与各数据端点。
type testServer struct {
	server   *httptest.Server
	privKey  *rsa.PrivateKey
	requests map[string]int
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	ts := &testServer{requests: map[string]int{}}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ts.privKey = key

	mux := http.NewServeMux()
	mux.HandleFunc("/verify2", func(w http.ResponseWriter, r *http.Request) {
		ts.requests["verify2:"+r.URL.Query().Get("reqtype")]++
		switch r.URL.Query().Get("reqtype") {
		case "do_rsa":
			spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
			if err != nil {
				t.Fatal(err)
			}
			pemStr := strings.ReplaceAll(string(pem.EncodeToMemory(
				&pem.Block{Type: "PUBLIC KEY", Bytes: spki},
			)), "\n", "&#10;")
			w.Write([]byte(`<xml><ret code="0" msg=""/><item pubkey="` + pemStr + `" rsa_version="v9"/></xml>`))
		case "unified_login":
			// 校验账号密码确实被 RSA 加密且可解密。
			account := r.URL.Query().Get("account")
			plain, err := rsaDecryptPKCS1v15(key, account)
			if err != nil || plain != "13800000000" {
				w.Write([]byte(`<xml><ret code="-1" msg="账号或密码错误"/></xml>`))
				return
			}
			passwd := r.URL.Query().Get("passwd")
			if _, err := rsaDecryptPKCS1v15(key, passwd); err != nil {
				w.Write([]byte(`<xml><ret code="-1" msg="账号或密码错误"/></xml>`))
				return
			}
			w.Write([]byte(`<xml><ret code="0" msg=""/><item userid="u123" sessionid="sess-abc" account="13800000000" rsa_version="v9"/></xml>`))
		case "mainverify":
			w.Write([]byte(`<xml><ret code="0" msg=""/><item passport="userid=u123|signvalid=SIGN-XYZ|x=1"/></xml>`))
		default:
			w.Write([]byte(`<xml><ret code="-1" msg="unknown reqtype"/></xml>`))
		}
	})
	mux.HandleFunc("/docookie2.php", func(w http.ResponseWriter, r *http.Request) {
		ts.requests["docookie"]++
		if r.URL.Query().Get("signvalid") != "SIGN-XYZ" {
			w.Write([]byte("bad"))
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "userid", Value: "u123"})
		http.SetCookie(w, &http.Cookie{Name: "user", Value: buildUserCookie("sess-and")})
	})
	mux.HandleFunc("/optdata/selfgroup/open/api/group/v1/query", func(w http.ResponseWriter, r *http.Request) {
		ts.requests["groups"]++
		// 线上事实：version 是数字。
		w.Write([]byte(`{"status_code":0,"data":{"version":105,"group_list":[` +
			`{"id":"0_1","name":"消费","type":"0","num":"2","content":"600519|000858,17|33"},` +
			`{"id":"1_2","name":"融资融券","type":"1","num":"0","content":""}]}}`))
	})
	mux.HandleFunc("/newcircle/group/getSelfStockWithMarket/", func(w http.ResponseWriter, r *http.Request) {
		ts.requests["selfv2"]++
		// 线上事实：code/marketid 可能是数字。
		w.Write([]byte(`{"errorCode":0,"result":[{"code":"600519","marketid":17},{"code":"000858","marketid":33}]}`))
	})
	mux.HandleFunc("/optdata/selfstock/open/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		ts.requests["selfv1"]++
		w.Write([]byte(`{"status_code":0,"data":{"selfstock":"600519|000858,17|33","version":9}}`))
	})
	mux.HandleFunc("/optdata/selfstock/open/api/v1/modify", func(w http.ResponseWriter, r *http.Request) {
		ts.requests["selfv1modify"]++
		body := r.FormValue("selfstock")
		w.Write([]byte(`{"status_code":0,"data":{"version":"10","echo":"` + body + `"}}`))
	})
	mux.HandleFunc("/multiStorage", func(w http.ResponseWriter, r *http.Request) {
		ts.requests["multiStorage:"+r.FormValue("reqtype")]++
		if r.FormValue("reqtype") == "download" {
			w.Write(buildBlockstockResponse(t, 7, 2))
			return
		}
		w.Write(buildBlockstockResponse(t, 8, 1))
	})
	mux.HandleFunc("/d/platform/dynamicplate/stocks/self/v2/select", func(w http.ResponseWriter, r *http.Request) {
		ts.requests["dynamic"]++
		w.Write([]byte(`{"data":{"codes":[{"code":"600519","market":"17"}]}}`))
	})
	mux.HandleFunc("/selfstock_detail", func(w http.ResponseWriter, r *http.Request) {
		ts.requests["detail"]++
		payload := base64.StdEncoding.EncodeToString([]byte(
			`[{"C":"600519","M":"17","P":1688.5,"T":"20231101"},{"C":"000858","M":"33","P":"88.1","T":1699000000}]`))
		w.Write([]byte(`<xml><ret code="0" msg=""/><item version="107" selfstock_detail="` + payload + `"/></xml>`))
	})
	mux.HandleFunc("/optdata/selfgroup/open/api/group/v1/add", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("name") == "" {
			w.Write([]byte(`{"status_code":400,"status_msg":"参数错误"}`))
			return
		}
		w.Write([]byte(`{"status_code":0,"data":{"id":"0_99"}}`))
	})
	mux.HandleFunc("/optdata/sharing_service/open/api/sharing/v1/create", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status_code":0,"data":{"share_url":"https://t.10jqka.com.cn/s/abc"}}`))
	})

	ts.server = httptest.NewServer(mux)
	t.Cleanup(ts.server.Close)

	// 全部端点指向本地测试服务。
	AuthBase = ts.server.URL
	UpassBase = ts.server.URL
	UGCBASE = ts.server.URL
	SelfStockV2Base = ts.server.URL
	MultiStorageURL = ts.server.URL + "/multiStorage"
	DynamicPlateURL = ts.server.URL

	return ts
}

func TestLoginFullFlow(t *testing.T) {
	ts := newTestServer(t)
	result, err := Login(context.Background(), "13800000000", "pw123456")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if result.UserID != "u123" || result.SessionID != "sess-abc" || result.SignValid != "SIGN-XYZ" {
		t.Errorf("登录结果不符: %+v", result)
	}
	if result.Cookies["userid"] != "u123" {
		t.Errorf("Cookie 不符: %+v", result.Cookies)
	}
	// auth_params 优先使用登录流程的 sessionid（与参考实现一致）。
	if result.AuthParams.UserID != "u123" || result.AuthParams.SessionID != "sess-abc" {
		t.Errorf("AuthParams 不符: %+v", result.AuthParams)
	}
	if result.AuthParams.Expires == "" {
		t.Error("AuthParams.Expires 应填充")
	}
	for _, step := range []string{"verify2:do_rsa", "verify2:unified_login", "verify2:mainverify", "docookie"} {
		if ts.requests[step] == 0 {
			t.Errorf("登录步骤未触发: %s", step)
		}
	}
}

func TestLoginWrongPassword(t *testing.T) {
	newTestServer(t)
	// 服务端校验解密结果是 13800000000，用别的账号会被拒。
	_, err := Login(context.Background(), "13900000000", "pw")
	if err == nil {
		t.Fatal("错误账号应登录失败")
	}
	apiErr, ok := AsAPIError(err)
	if !ok || !strings.Contains(apiErr.Error(), "账号或密码错误") {
		t.Errorf("错误语义不符: %v", err)
	}
}

func TestGroupsQuery(t *testing.T) {
	newTestServer(t)
	client := NewClient()
	client.SetCookies(map[string]string{"userid": "u123"})
	groups, version, err := client.QueryGroups(context.Background())
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if version != "105" || len(groups) != 2 {
		t.Fatalf("结果不符: version=%s groups=%d", version, len(groups))
	}
	consumer := groups[0]
	if consumer.Name != "消费" || !consumer.IsDynamic() == false || len(consumer.Items) != 2 {
		t.Errorf("分组 0 不符: %+v", consumer)
	}
	if consumer.Items[0].Code != "600519" || consumer.Items[0].MarketCode != "17" {
		t.Errorf("成员不符: %+v", consumer.Items)
	}
	if !groups[1].IsDynamic() {
		t.Errorf("1_ 前缀应识别为动态分组: %+v", groups[1])
	}
}

func TestSelfStockProtocols(t *testing.T) {
	newTestServer(t)
	client := NewClient()
	client.SetCookies(map[string]string{"userid": "u123"})

	v2Items, err := client.DownloadSelfStocksV2(context.Background())
	if err != nil || len(v2Items) != 2 || v2Items[0].Code != "600519" {
		t.Fatalf("v2 查询不符: %+v %v", v2Items, err)
	}

	v1, err := client.DownloadSelfStocksV1(context.Background())
	if err != nil || v1.Version != "9" || len(v1.Items) != 2 {
		t.Fatalf("v1 查询不符: %+v %v", v1, err)
	}

	newVersion, err := client.ModifySelfStocksV1(context.Background(), v1.Items, v1.Version)
	if err != nil || newVersion != "10" {
		t.Fatalf("v1 替换不符: %s %v", newVersion, err)
	}
}

func TestBlockstockRoundtripViaHTTP(t *testing.T) {
	newTestServer(t)
	client := NewClient()
	auth := AuthParams{UserID: "u1", SessionID: "s1", Expires: "2100-01-01 00:00:00"}

	download, err := client.BlockstockDownload(context.Background(), auth)
	if err != nil {
		t.Fatalf("下载失败: %v", err)
	}
	if download.Version != 7 || len(download.Groups) != 2 {
		t.Fatalf("下载结果不符: %+v", download)
	}

	version, err := client.BlockstockUpload(context.Background(), auth, download.Groups[0], "7")
	if err != nil || version != 8 {
		t.Fatalf("上传结果不符: %d %v", version, err)
	}
}

func TestDynamicPlateAndDetail(t *testing.T) {
	newTestServer(t)
	client := NewClient()
	client.SetCookies(map[string]string{"userid": "u123"})

	entries, err := client.QueryDynamicPlate(context.Background(), "融资融券")
	if err != nil || len(entries) != 1 || entries[0].MarketCode != "17" {
		t.Fatalf("动态板块不符: %+v %v", entries, err)
	}

	version, details, err := client.DownloadSelfStockDetail(context.Background())
	if err != nil || version != "107" {
		t.Fatalf("明细不符: %s %v", version, err)
	}
	if len(details) != 2 {
		t.Fatalf("明细数量不符: %+v", details)
	}
	if details[0].Code != "600519" || details[0].Market != "SH" || details[0].Price == nil || *details[0].Price != 1688.5 {
		t.Errorf("明细 0 不符: %+v", details[0])
	}
	if details[1].Price == nil || *details[1].Price != 88.1 {
		t.Errorf("字符串价格应转数值: %+v", details[1])
	}
	if details[1].AddedAt != "1699000000" {
		t.Errorf("时间戳不符: %+v", details[1])
	}
}

func TestGroupCreateAndShare(t *testing.T) {
	newTestServer(t)
	client := NewClient()
	if err := client.GroupCreate(context.Background(), "新组", "105"); err != nil {
		t.Fatalf("建组失败: %v", err)
	}
	shareURL, err := client.GroupShare(context.Background(), "u123", "0_1", "消费", 604800)
	if err != nil || shareURL != "https://t.10jqka.com.cn/s/abc" {
		t.Fatalf("分享不符: %s %v", shareURL, err)
	}
}

func TestBusinessErrorMapping(t *testing.T) {
	// 组查询打到 multiStorage 路径之外的 404 会变成 NetError；
	// 业务失败（status_code != 0）会变成 APIError。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status_code":1001,"status_msg":"分组不存在"}`))
	}))
	t.Cleanup(server.Close)
	UGCBASE = server.URL
	client := NewClient()
	_, _, err := client.QueryGroups(context.Background())
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("应为 APIError: %T %v", err, err)
	}
	if apiErr.Msg != "分组不存在" || apiErr.Code != "1001" {
		t.Errorf("错误内容不符: %+v", apiErr)
	}
}

func rsaDecryptPKCS1v15(key *rsa.PrivateKey, b64 string) (string, error) {
	cipher, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	plain, err := rsa.DecryptPKCS1v15(rand.Reader, key, cipher)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// TestBlockstockUploadRejected 线上事实：被拒的上传返回 3 字节响应（version=0），
// 必须识别为业务失败而不是静默成功。
func TestBlockstockUploadRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("reqtype") == "upload" {
			w.Write([]byte{0x08, 0x03}) // field1 varint(3)，无版本号
			return
		}
		w.Write(buildBlockstockResponse(t, 7, 1))
	}))
	t.Cleanup(server.Close)
	MultiStorageURL = server.URL + "/multiStorage"
	client := NewClient()
	auth := AuthParams{UserID: "u1", SessionID: "s1", Expires: "2100-01-01 00:00:00"}
	group := BlockstockGroup{Name: "测试", GroupType: 1, Items: []StockEntry{{Code: "600519", MarketCode: "17"}}}
	_, err := client.BlockstockUpload(context.Background(), auth, group, "7")
	if err == nil {
		t.Fatal("被拒的上传应当报错")
	}
	apiErr, ok := AsAPIError(err)
	if !ok {
		t.Fatalf("应为 APIError: %T %v", err, err)
	}
	if !strings.Contains(apiErr.Msg, "拒绝") {
		t.Errorf("错误语义不符: %s", apiErr.Msg)
	}
}
