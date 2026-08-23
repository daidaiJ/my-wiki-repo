package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 向其他 agent 工具的用户级指令文件（默认 qwen code 的 ~/.qwen/QWEN.md）
// 注入 wiki 规约引导段。流程：先按特殊标记检测 →
//   - 无标记段 → 末尾追加（与既有内容空行分隔；文件不存在则创建）
//   - 有标记段 → 原位替换（标记锚定，跨版本安全）
//   - 内容一致 → 不写盘（幂等）
// --remove 可整段摘除。

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
	b.WriteString("跨项目知识库与 Hugo 博客发布工具，二进制 `wiki`，完整规约见 D:\\CODE\\ai\\my-wiki\\AGENTS.md。\n\n")
	b.WriteString("- 接入项目 / 补充项目介绍与摘要（任意时间）：`wiki init <项目路径> --paths wiki`；事后补充 `wiki init <项目路径> --intro \"…\" --summary \"…\"`（paths 省略保留）。规约只认专用目录 `wiki/`（平铺知识库可声明 `.`），勿接入 `docs/` 等与官方文档同名的通用目录\n")
	b.WriteString("- 全局检索（路径规格：项目/链接/文件）：`wiki ls` / `wiki grep <模式>`（输出可直接喂给 `wiki cat <项目/链接/文件>`）/ `wiki tree <项目>`\n")
	b.WriteString("- 发博客：先 `wiki blog list`（只列 categories/tags，优先复用已有类别）→ `wiki blog new …` → `wiki blog publish <name>`；push 网络失败不重试，转告用户手动 push\n")
	b.WriteString("- 知识目录链接同步由 Stop hook 全自动（wiki check），无需人工干预；每个接入目录须有 README.md 索引（agent 维护）\n")
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

func defaultGuideTarget() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".qwen", "QWEN.md"), nil
}

func cmdInject(args []string) error {
	fs := flag.NewFlagSet("inject", flag.ContinueOnError)
	file := fs.String("file", "", "目标指令文件（缺省 ~/.qwen/QWEN.md）")
	remove := fs.Bool("remove", false, "摘除引导段而非注入")
	if err := parseWithPositionals(fs, args); err != nil {
		return err
	}
	target := *file
	if target == "" {
		var err error
		target, err = defaultGuideTarget()
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
	switch action {
	case "unchanged":
		fmt.Printf("已是最新，无需写入: %s\n", target)
	case "created":
		fmt.Printf("已创建并注入引导段: %s\n", target)
	case "appended":
		fmt.Printf("已追加引导段: %s\n", target)
	case "replaced":
		fmt.Printf("已原位替换旧版引导段: %s\n", target)
	case "removed":
		fmt.Printf("已摘除引导段: %s\n", target)
	case "absent":
		fmt.Printf("目标文件中没有引导段: %s\n", target)
	case "missing":
		fmt.Printf("目标文件不存在，跳过: %s\n", target)
	}
	return nil
}
