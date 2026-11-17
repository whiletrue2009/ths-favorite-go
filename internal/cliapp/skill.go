package cliapp

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/whiletrue2009/ths-favorite-go/internal/output"
	"github.com/whiletrue2009/ths-favorite-go/internal/skilldoc"
)

func newSkillCommand() *cobra.Command {
	skillCmd := &cobra.Command{
		Use:   "skill",
		Short: "AI 技能：安装说明书到你的 AI，或输出说明书内容",
		Long: "把本工具的用法教给 AI agent：\n" +
			"  install — 把技能说明书写入 Claude Code 的技能目录（~/.claude/skills/ths/）\n" +
			"  show    — 输出说明书原文（Markdown），可粘贴给豆包/Codex 等任何 AI",
	}
	skillCmd.AddCommand(newSkillInstallCommand(), newSkillShowCommand())
	return skillCmd
}

func newSkillShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "输出技能说明书原文（Markdown）",
		Long: "在 stdout 输出说明书 Markdown 原文（此命令不走 JSON 契约）。\n" +
			"用途：把内容粘贴到豆包/任意 AI 对话，或放进项目 AGENTS.md。",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print(skilldoc.Markdown())
			return nil
		},
	}
}

func newSkillInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "把技能安装到本机 AI（Claude Code）",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return &output.CLIError{Code: output.CodeInternal, Msg: "无法定位用户目录: " + err.Error()}
			}
			targetDir := filepath.Join(home, ".claude", "skills", "ths")
			if err := os.MkdirAll(targetDir, 0o755); err != nil {
				return &output.CLIError{Code: output.CodeInternal, Msg: "创建技能目录失败: " + err.Error()}
			}
			target := filepath.Join(targetDir, "SKILL.md")
			if err := os.WriteFile(target, []byte(skilldoc.Markdown()), 0o644); err != nil {
				return &output.CLIError{Code: output.CodeInternal, Msg: "写入技能文件失败: " + err.Error()}
			}
			output.Success(map[string]any{
				"operation":    "skill.install",
				"installed_to": []string{target},
				"note":         "Claude Code 重启后生效。豆包/Codex 等其他 AI：运行 ths skill show 把说明书内容发给它即可。",
			})
			return nil
		},
	}
}
