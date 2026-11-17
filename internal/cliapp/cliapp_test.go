package cliapp

import (
	"testing"

	"github.com/whiletrue2009/ths-favorite-go/internal/output"
)

func TestRequireConfirm(t *testing.T) {
	if err := requireConfirm(true, false, "op"); err != nil {
		t.Errorf("--yes 应放行: %v", err)
	}
	if err := requireConfirm(false, true, "op"); err != nil {
		t.Errorf("--dry-run 应放行: %v", err)
	}
	err := requireConfirm(false, false, "stock add")
	if err == nil {
		t.Fatal("无 --yes 且非 dry-run 应拒绝")
	}
	cliErr, ok := err.(*output.CLIError)
	if !ok || cliErr.Code != output.CodeUsage {
		t.Errorf("应为 usage 错误: %+v", err)
	}
	if !contains(cliErr.Msg, "--dry-run") || !contains(cliErr.Msg, "--yes") {
		t.Errorf("提示应包含两步操作说明: %s", cliErr.Msg)
	}
}

func TestExitCodeMapping(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{output.CodeNetwork, output.ExitNetwork},
		{output.CodeAPI, output.ExitAPI},
		{output.CodeAuth, output.ExitAuth},
		{output.CodeUsage, output.ExitUsage},
		{output.CodeInternal, output.ExitUsage},
		{"unknown", output.ExitUsage},
	}
	for _, tc := range cases {
		cliErr := &output.CLIError{Code: tc.code, Msg: "x"}
		if got := cliErr.ExitCode(); got != tc.want {
			t.Errorf("ExitCode(%s) = %d, want %d", tc.code, got, tc.want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
