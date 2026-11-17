// Package output 定义 CLI 的输出契约：
// 成功 → stdout 单行 JSON；失败 → stdout 一行 JSON 错误对象 + stderr 人话提示，
// 并以退出码区分错误类别，供 AI agent 程序化处理。
package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// 退出码语义（AGENTS.md 有对应用户文档）。
const (
	ExitOK      = 0
	ExitUsage   = 1 // 参数错误或程序内部错误
	ExitNetwork = 2 // 网络层失败，可稍后重试
	ExitAPI     = 3 // 同花顺返回业务失败，需要修正请求
	ExitAuth    = 4 // 登录态缺失或过期，需要 ths login
)

// 错误类别标签（JSON 中的 code 字段）。
const (
	CodeNetwork  = "network"
	CodeAPI      = "api"
	CodeAuth     = "auth"
	CodeUsage    = "usage"
	CodeInternal = "internal"
)

// CLIError 携带错误类别，由 main 统一转换为退出码。
type CLIError struct {
	Code string // CodeNetwork / CodeAPI / CodeAuth / CodeUsage / CodeInternal
	Msg  string
}

func (e *CLIError) Error() string { return e.Msg }

// ExitCode 返回该错误对应的进程退出码。
func (e *CLIError) ExitCode() int {
	switch e.Code {
	case CodeNetwork:
		return ExitNetwork
	case CodeAPI:
		return ExitAPI
	case CodeAuth:
		return ExitAuth
	case CodeUsage:
		return ExitUsage
	default:
		return ExitUsage
	}
}

// Success 输出成功 JSON。map 载荷平铺并附加 ok:true；
// 其他类型（结构体）先转 map 再平铺，保持输出 schema 扁平。
func Success(payload any) {
	final := map[string]any{}
	if m, ok := payload.(map[string]any); ok {
		final = m
	} else if payload != nil {
		if encoded, err := json.Marshal(payload); err == nil {
			var asMap map[string]any
			if json.Unmarshal(encoded, &asMap) == nil {
				final = asMap
			} else {
				final = map[string]any{"result": payload}
			}
		} else {
			final = map[string]any{"result": payload}
		}
	}
	final["ok"] = true
	encoded, err := json.Marshal(final)
	if err != nil {
		// 简单可序列化载荷不会走到这里；保底兜住。
		fmt.Fprintln(os.Stderr, "输出序列化失败:", err)
		os.Exit(ExitUsage)
	}
	fmt.Println(string(encoded))
}

// Fail 输出失败 JSON 到 stdout、人话到 stderr，并结束进程。
func Fail(code int, label, msg string) {
	errPayload, _ := json.Marshal(map[string]any{
		"ok":    false,
		"code":  label,
		"error": msg,
	})
	fmt.Println(string(errPayload))
	fmt.Fprintln(os.Stderr, "错误("+label+"): "+msg)
	os.Exit(code)
}
