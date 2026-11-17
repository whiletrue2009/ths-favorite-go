package cliapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/whiletrue2009/ths-favorite-go/internal/creds"
	"github.com/whiletrue2009/ths-favorite-go/internal/output"
	"github.com/whiletrue2009/ths-favorite-go/internal/session"
	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

// doctorCheck 是一项自检结果；detail/remedy 都是人话，agent 可直接转述。
// OK 的含义是「这一项不需要用户做任何事」。
type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
}

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "自检：网络、登录态、凭据",
		Long:  "环境自检，逐项给出修复建议。遇到问题先跑这个命令。",
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := runDoctor(cmd.Context())
			healthy := true
			for _, check := range checks {
				if !check.OK {
					healthy = false
				}
			}
			output.Success(map[string]any{
				"operation": "doctor",
				"healthy":   healthy,
				"version":   Version,
				"checks":    checks,
			})
			return nil
		},
	}
}

func runDoctor(ctx context.Context) []doctorCheck {
	checks := []doctorCheck{checkNetwork(ctx)}

	store, storeErr := session.NewStore(flags.sessionPath)
	var sess *session.Session
	if storeErr == nil {
		sess, _ = store.Load()
	}
	checks = append(checks, checkSession(store, storeErr, sess))
	checks = append(checks, checkCredentials())
	checks = append(checks, checkSessionValid(ctx, sess))
	return checks
}

func checkNetwork(ctx context.Context) doctorCheck {
	check := doctorCheck{Name: "network"}
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := thsclient.ProbeNetwork(probeCtx); err != nil {
		check.Detail = err.Error()
		check.Remedy = "无法连接同花顺服务，请检查网络（代理/防火墙）后重试"
		return check
	}
	check.OK = true
	check.Detail = "同花顺接口可达"
	return check
}

func checkSession(store *session.Store, storeErr error, sess *session.Session) doctorCheck {
	check := doctorCheck{Name: "session"}
	switch {
	case storeErr != nil:
		check.Detail = storeErr.Error()
		check.Remedy = "无法定位登录态目录，可用 --session 指定路径"
	case sess == nil:
		check.Detail = "尚未登录"
		check.Remedy = "运行 ths login 登录"
	case sess.Expired():
		check.Detail = fmt.Sprintf("登录态已过期（保存于 %.1f 小时前，有效期 24 小时）", sess.Age().Hours())
		check.Remedy = "运行 ths login 重新登录"
	default:
		check.OK = true
		check.Detail = fmt.Sprintf("登录态有效（保存于 %.1f 小时前）", sess.Age().Hours())
	}
	return check
}

func checkCredentials() doctorCheck {
	check := doctorCheck{Name: "credentials", OK: true}
	storage := creds.New()
	if cred, err := storage.Load(); err == nil {
		masked := cred.Username
		if len(masked) > 3 {
			masked = masked[:3] + "***"
		}
		check.Detail = "已保存账号 " + masked + "（" + storage.Backend() + "），过期后自动重登"
		return check
	} else if !errors.Is(err, creds.ErrNotFound) {
		check.OK = false
		check.Detail = err.Error()
		check.Remedy = "凭据存储不可用: " + storage.Backend()
		return check
	}
	check.Detail = "未保存账号（存储后端: " + storage.Backend() + "）"
	check.Remedy = "登录时默认会保存账号；保存后登录态过期可自动重登"
	return check
}

func checkSessionValid(ctx context.Context, sess *session.Session) doctorCheck {
	check := doctorCheck{Name: "session_valid"}
	if sess == nil {
		check.Remedy = "运行 ths login"
		return check
	}
	client := thsclient.NewClient()
	client.SetCookies(sess.Cookies)
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := client.DownloadSelfStocksV2(checkCtx); err != nil {
		check.Detail = err.Error()
		check.Remedy = "登录态被服务端拒绝，运行 ths login 重新登录"
		return check
	}
	check.OK = true
	check.Detail = "服务端确认登录态有效"
	return check
}
