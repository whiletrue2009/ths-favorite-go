package thsclient

import (
	"context"
	"net/url"
)

// ProbeNetwork 用免鉴权的公钥接口探测同花顺网络连通性。
func ProbeNetwork(ctx context.Context) error {
	query := url.Values{"reqtype": {"do_rsa"}, "type": {"get_pubkey"}}
	_, err := NewClient().Get(ctx, "网络探测", AuthBase+"/verify2", query, map[string]string{"User-Agent": LoginUA})
	return err
}
