package cliapp

import (
	"github.com/spf13/cobra"
	"github.com/whiletrue2009/ths-favorite-go/internal/engine"
	"github.com/whiletrue2009/ths-favorite-go/internal/output"
)

func newStockCommand() *cobra.Command {
	stockCmd := &cobra.Command{
		Use:   "stock",
		Short: "股票增删操作",
		Long: "向分组添加/删除股票。\n" +
			"股票代码格式 code.market，如 600519.SH、000001.SZ、430047.BJ。\n" +
			"分组可以是「我的自选」或自定义分组名；动态分组（readonly）不可写。\n" +
			"写操作需 --yes；先 --dry-run 可预览不执行。",
	}
	var (
		yes    bool
		dryRun bool
	)
	addCmd := &cobra.Command{
		Use:   "add GROUP SYMBOL [SYMBOL...]",
		Short: "把股票加入分组",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConfirm(yes, dryRun, "stock add"); err != nil {
				return err
			}
			eng, err := newEngine()
			if err != nil {
				return err
			}
			plan, result, err := eng.StockAdd(cmd.Context(), args[0], args[1:], dryRun)
			if err != nil {
				return cliError(err)
			}
			return emitWriteResult(plan, result)
		},
	}
	delCmd := &cobra.Command{
		Use:   "del GROUP SYMBOL [SYMBOL...]",
		Short: "从分组删除股票",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConfirm(yes, dryRun, "stock del"); err != nil {
				return err
			}
			eng, err := newEngine()
			if err != nil {
				return err
			}
			plan, result, err := eng.StockDel(cmd.Context(), args[0], args[1:], dryRun)
			if err != nil {
				return cliError(err)
			}
			return emitWriteResult(plan, result)
		},
	}
	for _, sub := range []*cobra.Command{addCmd, delCmd} {
		sub.Flags().BoolVar(&yes, "yes", false, "确认执行远端写操作")
		sub.Flags().BoolVar(&dryRun, "dry-run", false, "只校验并展示将要执行的操作，不写远端")
		stockCmd.AddCommand(sub)
	}
	return stockCmd
}

func newReorderCommand() *cobra.Command {
	var (
		yes    bool
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:   "reorder GROUP SYMBOL [SYMBOL...]",
		Short: "按给定完整顺序重排分组",
		Long: "重排分组内股票顺序。\n" +
			"SYMBOL 列表必须是该分组现有成员的完整排列：不能新增、不能缺少、不能重复，\n" +
			"只改变顺序。可先 ths list -g GROUP 查看当前成员与顺序。",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireConfirm(yes, dryRun, "reorder"); err != nil {
				return err
			}
			eng, err := newEngine()
			if err != nil {
				return err
			}
			plan, result, err := eng.Reorder(cmd.Context(), args[0], args[1:], dryRun)
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

// emitWriteResult 统一输出 dry-run 计划或实际执行结果。
// 用具体指针类型判空，避免接口包裹 typed-nil 指针的坑。
func emitWriteResult(plan *engine.PlanResult, result *engine.WriteResult) error {
	if plan != nil {
		output.Success(plan)
		return nil
	}
	if result != nil {
		output.Success(result)
		return nil
	}
	output.Success(map[string]any{})
	return nil
}
