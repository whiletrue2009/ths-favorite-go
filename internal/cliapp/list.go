package cliapp

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/whiletrue2009/ths-favorite-go/internal/engine"
	"github.com/whiletrue2009/ths-favorite-go/internal/output"
)

func newListCommand() *cobra.Command {
	var (
		group     string
		noDynamic bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "查看自选股分组（全部或指定分组）",
		Long: "查看自选股分组。\n" +
			"默认列出全部分组并包含「我的自选」；-g 指定分组时只列该分组成员。\n" +
			"动态分组（同花顺条件生成，readonly=true）的成员按需实时查询。",
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, err := newEngine()
			if err != nil {
				return err
			}
			views, err := eng.ListGroups(cmd.Context(), engine.ListOptions{
				Group:          group,
				IncludeSelf:    group == "",
				HydrateDynamic: !noDynamic,
			})
			if err != nil {
				return cliError(err)
			}
			if flags.pretty {
				printGroupsPretty(views, group != "")
				return nil
			}
			if group != "" && len(views) == 1 {
				output.Success(map[string]any{"group": views[0]})
				return nil
			}
			output.Success(map[string]any{"groups": views})
			return nil
		},
	}
	cmd.Flags().StringVarP(&group, "group", "g", "", "分组名称或 ID；省略则列出全部分组概览")
	cmd.Flags().BoolVar(&noDynamic, "no-dynamic", false, "跳过动态分组成员查询（更快）")
	return cmd
}

func newSelfCommand() *cobra.Command {
	selfCmd := &cobra.Command{
		Use:   "self",
		Short: "「我的自选」相关操作",
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "列出我的自选（含加入价格与时间）",
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, err := newEngine()
			if err != nil {
				return err
			}
			view, err := eng.SelfList(cmd.Context())
			if err != nil {
				return cliError(err)
			}
			if flags.pretty {
				printGroupsPretty([]engine.GroupView{*view}, true)
				return nil
			}
			output.Success(map[string]any{"group": view})
			return nil
		},
	}
	selfCmd.AddCommand(listCmd)
	return selfCmd
}

// printGroupsPretty 用对齐表格输出分组。
func printGroupsPretty(views []engine.GroupView, withItems bool) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if withItems {
		for _, view := range views {
			fmt.Fprintf(writer, "分组 %s\t(ID: %s)\t成员 %d\n", view.Name, view.GroupID, len(view.Items))
			fmt.Fprintln(writer, "代码\t市场\t加入价\t加入时间")
			for _, item := range view.Items {
				price := "-"
				if item.Price != nil {
					price = fmt.Sprintf("%.2f", *item.Price)
				}
				added := item.AddedAt
				if added == "" {
					added = "-"
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", item.Symbol, item.Market, price, added)
			}
			fmt.Fprintln(writer)
		}
	} else {
		fmt.Fprintln(writer, "分组名称\t分组ID\t成员数\t只读")
		for _, view := range views {
			fmt.Fprintf(writer, "%s\t%s\t%d\t%s\n", view.Name, view.GroupID, len(view.Items), boolText(view.Readonly))
		}
	}
	writer.Flush()
}

func boolText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}
