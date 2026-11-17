package cliapp

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/whiletrue2009/ths-favorite-go/internal/output"
	"golang.org/x/term"
)

func newLoginCommand() *cobra.Command {
	var (
		username  string
		password  string
		fromStdin bool
		noSave    bool
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "登录同花顺并保存登录态（首次使用必做）",
		Long: "登录同花顺：账号密码只发往同花顺官方接口，本地保存到 OS 钥匙串。\n" +
			"交互式运行会隐藏密码输入；AI agent 可用 --stdin 从标准输入逐行读取\n" +
			"（第一行账号，第二行密码），避免密码出现在命令行参数里。",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromStdin {
				lines := readStdinLines(cmd, 2)
				if len(lines) >= 2 {
					username, password = lines[0], lines[1]
				}
			}
			if username == "" {
				var err error
				username, err = promptPlain("同花顺账号（手机号）: ")
				if err != nil {
					return &output.CLIError{Code: output.CodeUsage, Msg: err.Error()}
				}
			}
			if password == "" {
				var err error
				password, err = promptHidden("同花顺密码: ")
				if err != nil {
					return &output.CLIError{Code: output.CodeUsage, Msg: err.Error()}
				}
			}
			eng, err := newEngine()
			if err != nil {
				return err
			}
			result, err := eng.Login(cmd.Context(), username, password, !noSave)
			if err != nil {
				return cliError(err)
			}
			groupCount := -1
			if count, qErr := eng.GroupCount(cmd.Context()); qErr == nil {
				groupCount = count
			}
			payload := map[string]any{
				"operation":   "login",
				"userid":      result.UserID,
				"username":    username,
				"group_count": groupCount,
			}
			if noSave {
				payload["credentials_saved"] = false
			}
			output.Success(payload)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "同花顺账号（也可交互输入或 --stdin）")
	cmd.Flags().StringVar(&password, "password", "", "同花顺密码（建议留空改用交互/ --stdin，避免留在命令行历史）")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "从标准输入读取：第一行账号，第二行密码")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "本次登录不保存账号密码（登录态仍会保存 24 小时）")
	return cmd
}

func newLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "删除本地保存的登录态与账号凭据",
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, err := newEngine()
			if err != nil {
				return err
			}
			if err := eng.Logout(); err != nil {
				return cliError(err)
			}
			output.Success(map[string]any{"operation": "logout"})
			return nil
		},
	}
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本号",
		RunE: func(cmd *cobra.Command, args []string) error {
			output.Success(map[string]any{"operation": "version", "version": Version})
			return nil
		},
	}
}

// promptPlain 读一行明文输入。
func promptPlain(prompt string) (string, error) {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", fmt.Errorf("读取账号输入失败")
	}
	value := strings.TrimSpace(scanner.Text())
	if value == "" {
		return "", fmt.Errorf("账号不能为空")
	}
	return value, nil
}

// promptHidden 读一行隐藏输入（终端支持时）。
func promptHidden(prompt string) (string, error) {
	fmt.Print(prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("读取密码输入失败: %w", err)
		}
		value := strings.TrimSpace(string(raw))
		if value == "" {
			return "", fmt.Errorf("密码不能为空")
		}
		return value, nil
	}
	// 非终端（管道）场景退化为明文行。
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return "", fmt.Errorf("读取密码输入失败")
	}
	value := strings.TrimSpace(scanner.Text())
	if value == "" {
		return "", fmt.Errorf("密码不能为空")
	}
	return value, nil
}

// readStdinLines 读取标准输入的前 n 行。
func readStdinLines(cmd *cobra.Command, n int) []string {
	scanner := bufio.NewScanner(os.Stdin)
	lines := make([]string, 0, n)
	for len(lines) < n && scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}
	return lines
}
