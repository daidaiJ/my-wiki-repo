package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cmdInit 是 agent 的接入入口：声明与元数据只写入 my-wiki 本地注册表（index.md，
// 本地 git 无 remote），并在 projects/ 下建链接。**不在项目仓库留下任何文件**，
// 因此声明块永远不会随项目 commit/push 泄漏到远程。
// paths 首次必填；intro/summary 可在任意时间事后补充（省略时保留已有值）。
func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dir := fs.String("dir", "", "项目根目录（可省略，改用位置参数或当前目录）")
	paths := fs.String("paths", "", "接入的知识目录，逗号分隔相对路径（省略则自动发现已配置类型名的目录；平铺知识库用 .）")
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

	root := wikiRoot()
	reg, err := loadRegistry(root)
	if err != nil {
		return err
	}
	old, had := findEntryByRoot(reg, abs)

	kdirs := knowledgeDirs()
	decl := &wikiSyncDecl{}
	switch {
	case *paths != "":
		decl.Paths = splitCSV(*paths)
	case had:
		decl.Paths = old.Paths
	default:
		// 自动发现：项目根下存在哪些已配置的知识目录类型名就接哪些
		decl.Paths = discoverKnowledgeDirs(abs, kdirs)
		if len(decl.Paths) == 0 {
			return fmt.Errorf("未在 %s 发现知识目录（类型名 %s）。用 --paths 显式指定，或在 config.json 的 knowledgeDirs 里调整类型名", abs, strings.Join(kdirs, ", "))
		}
	}
	decl.Intro = pick(*intro, old.Intro)
	decl.Summary = pick(*summary, old.Summary)

	for _, p := range decl.Paths {
		if p != "." && !containsStr(kdirs, p) {
			fmt.Fprintf(os.Stderr, "⚠ %s 不在配置的知识目录类型名（%s）内，确认不是上游官方文档目录再接入\n", p, strings.Join(kdirs, ", "))
		}
	}

	res, err := ensureRegistered(root, abs, decl)
	if err != nil {
		return err
	}
	fmt.Printf("已接入项目 %s（%s）\n  声明位置: %s（本地注册表，项目仓库零足迹，不会随项目 push 外泄）\n  接入路径: %s\n",
		res.Entry.Name, abs, registryPath(root), strings.Join(decl.Paths, ", "))
	for _, w := range res.ReadmeWarnings {
		fmt.Fprintln(os.Stderr, "⚠ "+w)
	}
	return nil
}

// discoverKnowledgeDirs 扫描项目根，返回存在且已配置的知识目录类型名（按配置顺序）。
func discoverKnowledgeDirs(proj string, kdirs []string) []string {
	var found []string
	for _, name := range kdirs {
		if fi, err := os.Stat(filepath.Join(proj, name)); err == nil && fi.IsDir() {
			found = append(found, name)
		}
	}
	return found
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// pick 新值非空取新值，否则保留旧值。
func pick(newVal, oldVal string) string {
	if newVal != "" {
		return newVal
	}
	return oldVal
}
