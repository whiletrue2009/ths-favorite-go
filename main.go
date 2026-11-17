// 命令 ths：同花顺自选股管理 CLI。
// 输出契约与退出码语义见 AGENTS.md。
package main

import (
	"errors"
	"os"

	"github.com/whiletrue2009/ths-favorite-go/internal/cliapp"
	"github.com/whiletrue2009/ths-favorite-go/internal/output"
)

func main() {
	if err := cliapp.Execute(); err != nil {
		var cliErr *output.CLIError
		if errors.As(err, &cliErr) {
			output.Fail(cliErr.ExitCode(), cliErr.Code, cliErr.Msg)
			return
		}
		// cobra 的参数错误等：已经在 stderr 打过 usage。
		output.Fail(output.ExitUsage, output.CodeUsage, err.Error())
		return
	}
	os.Exit(output.ExitOK)
}
