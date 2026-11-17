package cliapp

import (
	"strconv"

	"github.com/spf13/cobra"
	"github.com/whiletrue2009/ths-favorite-go/internal/output"
)

// shareDefaultSeconds 分享链接默认有效期：7 天。
const shareDefaultSeconds = 7 * 24 * 60 * 60

func newGroupCommand() *cobra.Command {
	groupCmd := &cobra.Command{
		Use:   "group",
		Short: "分组管理（新建/删除/分享）",
		Long: "自定义分组管理。\n" +
			"「我的自选」是系统分组不可删除；动态分组（readonly）由同花顺条件生成，不可增删改。\n" +
			"新建/删除是远端写操作，需 --yes（支持 --dry-run 预览）。",
	}
	groupCmd.AddCommand(
		newGroupAddCommand(),
		newGroupDelCommand(),
		newGroupShareCommand(),
	)
	return groupCmd
}

func newGroupAddCommand() *cobra.Command {
	var (
		yes    bool
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "add NAME",
		Short: "新建自定义分组",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConfirm(yes, dryRun, "group add"); err != nil {
				return err
			}
			eng, err := newEngine()
			if err != nil {
				return err
			}
			plan, result, err := eng.GroupCreate(cmd.Context(), args[0], dryRun)
			if err != nil {
				return cliError(err)
			}
			return emitWriteResult(plan, result)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "确认执行远端写操作")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只校验并展示将要执行的操作，不写远端")
	return cmd
}

func newGroupDelCommand() *cobra.Command {
	var (
		yes    bool
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "del GROUP",
		Short: "删除自定义分组（组内股票一并删除）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConfirm(yes, dryRun, "group del"); err != nil {
				return err
			}
			eng, err := newEngine()
			if err != nil {
				return err
			}
			plan, result, err := eng.GroupDelete(cmd.Context(), args[0], dryRun)
			if err != nil {
				return cliError(err)
			}
			return emitWriteResult(plan, result)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "确认执行远端写操作")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只校验并展示将要执行的操作，不写远端")
	return cmd
}

func newGroupShareCommand() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "share GROUP [SECONDS]",
		Short: "生成分组分享链接（默认 7 天有效）",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			validSeconds := shareDefaultSeconds
			if len(args) == 2 {
				parsed, err := strconv.Atoi(args[1])
				if err != nil || parsed <= 0 {
					return &output.CLIError{Code: output.CodeUsage, Msg: "有效期（秒）必须是正整数"}
				}
				validSeconds = parsed
			}
			eng, err := newEngine()
			if err != nil {
				return err
			}
			plan, result, err := eng.GroupShare(cmd.Context(), args[0], validSeconds, dryRun)
			if err != nil {
				return cliError(err)
			}
			return emitWriteResult(plan, result)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只校验分组存在，不生成链接")
	return cmd
}
