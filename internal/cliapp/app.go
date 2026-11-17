// Package cliapp 装配全部 CLI 命令。
package cliapp

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/whiletrue2009/ths-favorite-go/internal/creds"
	"github.com/whiletrue2009/ths-favorite-go/internal/engine"
	"github.com/whiletrue2009/ths-favorite-go/internal/output"
	"github.com/whiletrue2009/ths-favorite-go/internal/session"
	"github.com/whiletrue2009/ths-favorite-go/internal/thsclient"
)

// Version 由构建时通过 -ldflags 注入，默认 dev。
var Version = "dev"

// globalFlags 是所有子命令共享的选项。
type globalFlags struct {
	sessionPath string
	pretty      bool
}

var flags globalFlags

// NewRootCommand 构造根命令。
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "ths",
		Short: "同花顺自选股管理工具（供人和 AI agent 使用）",
		Long: "同花顺自选股管理工具：查看/添加/删除/重排自选股与自定义分组。\n" +
			"所有命令默认输出单行 JSON（stdout），错误带退出码语义，方便 AI agent 调用。\n" +
			"首次使用请先运行: ths login",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&flags.sessionPath, "session", "", "登录态文件路径（默认 OS 用户配置目录）")
	root.PersistentFlags().BoolVar(&flags.pretty, "pretty", false, "人类可读表格输出（默认单行 JSON）")
	root.AddCommand(
		newLoginCommand(),
		newLogoutCommand(),
		newDoctorCommand(),
		newVersionCommand(),
		newListCommand(),
		newSelfCommand(),
		newStockCommand(),
		newGroupCommand(),
		newReorderCommand(),
	)
	return root
}

// Execute 运行 CLI；错误由 main 统一转换为退出码。
func Execute() error {
	return NewRootCommand().Execute()
}

// newEngine 按当前 flags 构造引擎。
func newEngine() (*engine.Engine, error) {
	store, err := session.NewStore(flags.sessionPath)
	if err != nil {
		return nil, &output.CLIError{Code: output.CodeInternal, Msg: err.Error()}
	}
	return engine.New(thsclient.NewClient(), store, creds.New()), nil
}

// requireConfirm 强制写操作走「先 dry-run 后 --yes」两步。
func requireConfirm(yes, dryRun bool, operation string) error {
	if dryRun || yes {
		return nil
	}
	return &output.CLIError{
		Code: output.CodeUsage,
		Msg:  operation + " 是远端写操作：请先加 --dry-run 预览将要执行的改动，向用户确认后再追加 --yes 真正执行",
	}
}

// cliError 把底层错误映射为带类别的 CLI 错误。
func cliError(err error) error {
	var apiErr *thsclient.APIError
	if errors.As(err, &apiErr) {
		return &output.CLIError{Code: output.CodeAPI, Msg: apiErr.Error()}
	}
	var netErr *thsclient.NetError
	if errors.As(err, &netErr) {
		return &output.CLIError{Code: output.CodeNetwork, Msg: netErr.Error() + "（可稍后重试；若持续失败请运行 ths doctor）"}
	}
	var authErr *thsclient.AuthError
	if errors.As(err, &authErr) {
		return &output.CLIError{Code: output.CodeAuth, Msg: authErr.Error() + "，请运行 ths login 重新登录"}
	}
	var cliErr *output.CLIError
	if errors.As(err, &cliErr) {
		return cliErr
	}
	return &output.CLIError{Code: output.CodeInternal, Msg: fmt.Sprintf("未预期的错误: %v", err)}
}
