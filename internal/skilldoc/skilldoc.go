// Package skilldoc 内嵌分发用的 AI 技能说明书（SKILL.md）。
// 二进制自带说明书：ths skill show / install 无需联网获取文档。
package skilldoc

import _ "embed"

//go:embed SKILL.md
var skillMarkdown string

// Markdown 返回技能说明书原文。
func Markdown() string {
	return skillMarkdown
}
