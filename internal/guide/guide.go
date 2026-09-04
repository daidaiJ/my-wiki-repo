// Package guide 把 wiki 规约引导段注入其他 agent 工具的用户级指令文件
// （目标路径可用 config injectFile 配置——不同 agent 工具的路径不同，
// 缺省 qwen code 的 ~/.qwen/QWEN.md），标记锚定、跨版本原位替换。
package guide

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
	"github.com/daidaiJ/my-wiki-repo/internal/config"
)

const (
	guideStartMarker = "<!-- wiki-guide:start -->"
	guideEndMarker   = "<!-- wiki-guide:end -->"
)

var guideSectionRe = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(guideStartMarker) + `.*?` + regexp.QuoteMeta(guideEndMarker))

// guideSection 是注入的固定引导内容（升级规约后改这里，重跑 wiki inject 即可原位替换）。
func guideSection() string {
	var b strings.Builder
	b.WriteString(guideStartMarker + "\n")
	b.WriteString("## wiki 知识库与博客发布（wiki CLI）\n\n")
	b.WriteString("跨项目知识库与 Hugo 博客发布工具，二进制 `wiki`，完整规约见 wiki 根目录的 `AGENTS.md`（即 wiki 可执行文件所在目录，`wiki config` 可查 wikiRoot）。\n\n")
	b.WriteString("- 接入项目：`wiki init <项目路径> [--paths wiki,issues]`（首次）；已注册项目开工前 `wiki prepare`、收工 `wiki check`（hook 自动或 agent 手动）\n")
	b.WriteString("- 全局检索（路径规格：项目/链接/文件）：`wiki ls` / `wiki grep <模式>` / `wiki cat <项目/链接/文件>` / `wiki tree <项目>`\n")
	b.WriteString("- 发博客：先 `wiki blog list` → `wiki blog new …` → `wiki blog publish <name>`；push 失败不重试\n")
	b.WriteString("- 日常存储是方案 C（正文在 `projects/`，项目侧窗口或 `--mode copy` 真目录）；旧正向链接（方案 A）会被自动反转；跨机器备份用 `wiki bundle`（方案 B，不是日常布局）\n")
	b.WriteString(guideEndMarker)
	return b.String()
}

// injectGuide 对目标文件执行 检测→追加/替换，返回动作：created/appended/replaced/unchanged。
func injectGuide(path string) (string, error) {
	section := guideSection()
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return "created", os.WriteFile(path, []byte(section+"\n"), 0o644)
	}
	content := string(data)
	if guideSectionRe.MatchString(content) {
		updated := guideSectionRe.ReplaceAllString(content, section)
		if updated == content {
			return "unchanged", nil
		}
		return "replaced", os.WriteFile(path, []byte(updated), 0o644)
	}
	sep := ""
	if !strings.HasSuffix(content, "\n") {
		sep = "\n"
	}
	return "appended", os.WriteFile(path, []byte(content+sep+"\n"+section+"\n"), 0o644)
}

// removeGuide 摘除标记段并清理残留的连续空行。
func removeGuide(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	content := string(data)
	if !guideSectionRe.MatchString(content) {
		return "absent", nil
	}
	updated := guideSectionRe.ReplaceAllString(content, "")
	updated = regexp.MustCompile(`\n{3,}`).ReplaceAllString(updated, "\n\n")
	updated = strings.TrimRight(updated, "\n") + "\n"
	return "removed", os.WriteFile(path, []byte(updated), 0o644)
}

// CmdInject 把规约引导段注入用户级指令文件（--remove 摘除）。
// 目标优先级：--file 旗标 > config.json injectFile / WIKI_INJECT_FILE 环境变量 > ~/.qwen/QWEN.md。
func CmdInject(args []string) error {
	fs := flag.NewFlagSet("inject", flag.ContinueOnError)
	file := fs.String("file", "", "目标指令文件（缺省 config injectFile，再缺省 ~/.qwen/QWEN.md）")
	remove := fs.Bool("remove", false, "摘除引导段而非注入")
	if err := cli.ParseWithPositionals(fs, args); err != nil {
		return err
	}
	target := *file
	if target == "" {
		var err error
		target, err = config.InjectFile()
		if err != nil {
			return err
		}
	}
	var action string
	var err error
	if *remove {
		action, err = removeGuide(target)
	} else {
		action, err = injectGuide(target)
	}
	if err != nil {
		return err
	}
	messages := map[string]string{
		"unchanged": "已是最新，无需写入",
		"created":   "已创建并注入引导段",
		"appended":  "已追加引导段",
		"replaced":  "已原位替换旧版引导段",
		"removed":   "已摘除引导段",
		"absent":    "目标文件中没有引导段",
		"missing":   "目标文件不存在，跳过",
	}
	fmt.Printf("%s: %s\n", messages[action], target)
	return nil
}
