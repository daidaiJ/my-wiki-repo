package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildWikiSyncBlock 生成要写入项目 AGENTS.md 的 wiki-sync 声明块。
func buildWikiSyncBlock(decl *wikiSyncDecl) string {
	data, _ := json.MarshalIndent(decl, "", "  ")
	return "<!-- wiki-sync\n" + string(data) + "\n-->"
}

// upsertWikiSyncBlock 把声明块写进项目的 AGENTS.md：
// 已有块则原位替换，没有则追加；文件不存在则创建。其余内容保持不动。
func upsertWikiSyncBlock(projectRoot string, decl *wikiSyncDecl) error {
	path := filepath.Join(projectRoot, "AGENTS.md")
	data, err := os.ReadFile(path)
	block := buildWikiSyncBlock(decl)
	switch {
	case err == nil:
		if wikiSyncRe.Match(data) {
			updated := wikiSyncRe.ReplaceAll(data, []byte(block))
			if string(updated) == string(data) {
				return nil
			}
			return os.WriteFile(path, updated, 0o644)
		}
		sep := "\n"
		if len(data) == 0 || data[len(data)-1] == '\n' {
			sep = ""
		}
		return os.WriteFile(path, append(append(data, []byte(sep+"\n")...), []byte(block+"\n")...), 0o644)
	case errors.Is(err, os.ErrNotExist):
		content := "# AGENTS.md\n\n" + block + "\n"
		return os.WriteFile(path, []byte(content), 0o644)
	default:
		return err
	}
}

// cmdInit 是 agent 的接入入口：写入/更新声明块并立即注册。
// paths 首次必填；intro/summary 可在任意时间事后补充（省略时保留已有值）。
func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dir := fs.String("dir", "", "项目根目录（可省略，改用位置参数或当前目录）")
	paths := fs.String("paths", "", "接入的知识目录，逗号分隔的相对路径（首次必填；省略则保留已有）")
	intro := fs.String("intro", "", "一句话项目介绍（可事后补充/更新）")
	summary := fs.String("summary", "", "项目摘要，几句话（可事后补充/更新）")
	if err := parseWithPositionals(fs, args); err != nil {
		return err
	}
	dirArg := "."
	switch {
	case fs.NArg() == 1:
		dirArg = fs.Arg(0)
	case *dir != "":
		dirArg = *dir
	case fs.NArg() > 1:
		return errors.New("用法: wiki init [目录] --paths <目录列表> [--intro ...] [--summary ...]")
	}
	abs, err := filepath.Abs(dirArg)
	if err != nil {
		return err
	}

	old, oldErr := parseWikiSync(abs)
	hadBlock := oldErr == nil
	decl := &wikiSyncDecl{}
	switch {
	case *paths != "":
		decl.Paths = splitCSV(*paths)
	case hadBlock:
		decl.Paths = old.Paths
	default:
		return errors.New("首次接入必须 --paths 指定知识目录（逗号分隔，如 wiki,docs/research）")
	}
	decl.Intro = old.getIntroOr(*intro)
	decl.Summary = old.getSummaryOr(*summary)

	if err := validatePaths(abs, decl.Paths); err != nil {
		return err
	}
	if err := upsertWikiSyncBlock(abs, decl); err != nil {
		return fmt.Errorf("写入 AGENTS.md 失败: %w", err)
	}

	root := wikiRoot()
	res, err := ensureRegistered(root, abs, decl)
	if err != nil {
		return err
	}
	fmt.Printf("已接入项目 %s（%s）\n  声明块: %s/AGENTS.md（%s）\n  接入路径: %s\n",
		res.Entry.Name, abs, abs, ternary(hadBlock, "原位更新", "新建"), strings.Join(decl.Paths, ", "))
	for _, w := range res.ReadmeWarnings {
		fmt.Fprintln(os.Stderr, "⚠ "+w)
	}
	return nil
}

func (d *wikiSyncDecl) getIntroOr(v string) string {
	if v != "" {
		return v
	}
	if d != nil {
		return d.Intro
	}
	return ""
}

func (d *wikiSyncDecl) getSummaryOr(v string) string {
	if v != "" {
		return v
	}
	if d != nil {
		return d.Summary
	}
	return ""
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
