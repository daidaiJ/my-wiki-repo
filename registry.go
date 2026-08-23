package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const defaultWikiRoot = `D:\CODE\ai\my-wiki`

func wikiRoot() string {
	if v := os.Getenv("WIKI_ROOT"); v != "" {
		return v
	}
	return defaultWikiRoot
}

// ProjectEntry 登记一个已接入项目：名字、根目录、介绍、接入的相对路径。
type ProjectEntry struct {
	Name  string   `json:"name"`
	Root  string   `json:"root"`
	Intro string   `json:"intro"`
	Paths []string `json:"paths"`
}

type Registry struct {
	Projects []ProjectEntry `json:"projects"`
}

var (
	wikiSyncRe   = regexp.MustCompile(`(?s)<!--\s*wiki-sync\s*(\{.*?\})\s*-->`)
	wikiRegRe    = regexp.MustCompile(`(?s)<!--\s*wiki-registry\s*(\{.*?\})\s*-->`)
	badNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)
)

// --- 注册表持久化：index.md 顶部隐藏 JSON 块为数据源，其余为渲染视图 ---

func registryPath(root string) string { return filepath.Join(root, "index.md") }

func loadRegistry(root string) (*Registry, error) {
	reg := &Registry{}
	data, err := os.ReadFile(registryPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return reg, nil
	}
	if err != nil {
		return nil, err
	}
	m := wikiRegRe.FindSubmatch(data)
	if m == nil {
		return reg, nil
	}
	if err := json.Unmarshal(m[1], reg); err != nil {
		return nil, fmt.Errorf("index.md 注册块解析失败: %w", err)
	}
	return reg, nil
}

func saveRegistry(root string, reg *Registry) error {
	if reg.Projects == nil {
		reg.Projects = []ProjectEntry{}
	}
	sort.Slice(reg.Projects, func(i, j int) bool { return reg.Projects[i].Name < reg.Projects[j].Name })
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("<!-- wiki-registry\n")
	b.Write(data)
	b.WriteString("\n-->\n\n")
	b.WriteString("# 项目索引\n\n")
	b.WriteString("> 本文件由 `wiki register/unlink/sync` 自动生成维护，不要手改。\n\n")
	b.WriteString("| 项目 | 目录 | 介绍 | 接入路径 |\n|---|---|---|---|\n")
	for _, p := range reg.Projects {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", p.Name, p.Root, p.Intro, strings.Join(p.Paths, ", "))
	}
	return os.WriteFile(registryPath(root), []byte(b.String()), 0o644)
}

// --- 符号链接：优先 symlink，Windows 无权限时降级 junction ---

func makeLink(target, link string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if err := os.Symlink(target, link); err == nil {
		return nil
	} else if runtime.GOOS == "windows" {
		out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
		if jerr == nil {
			return nil
		}
		return fmt.Errorf("symlink 失败: %v；junction 降级也失败: %v: %s", err, jerr, strings.TrimSpace(string(out)))
	} else {
		return err
	}
}

// linkName 计算某个接入路径在 projects/<项目>/ 下的链接名。
// 同项目内 basename 冲突时退化为清洗后的相对路径（如 docs_wiki）。
func linkName(relPath string, taken map[string]bool) string {
	base := filepath.Base(filepath.ToSlash(relPath))
	if !taken[base] {
		taken[base] = true
		return base
	}
	flat := badNameChars.ReplaceAllString(strings.ReplaceAll(filepath.ToSlash(relPath), "/", "_"), "_")
	for taken[flat] {
		flat += "_"
	}
	taken[flat] = true
	return flat
}

func linkPathFor(root string, e ProjectEntry, relPath string, taken map[string]bool) string {
	return filepath.Join(root, "projects", e.Name, linkName(relPath, taken))
}

// createLink 建立或重建一条链接；链接位置已有真实目录时拒绝动手。
func createLink(target, link string) error {
	if fi, err := os.Lstat(link); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 && !fi.IsDir() {
			return fmt.Errorf("%s 已存在且不是链接，请手动处理", link)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(link); err != nil {
				return fmt.Errorf("移除旧链接 %s 失败: %w", link, err)
			}
		} else if fi.IsDir() {
			// junction 在 Lstat 下也表现为 symlink，走到这里说明是真实目录
			return fmt.Errorf("%s 已存在真实目录（非链接），请手动处理", link)
		}
	}
	return makeLink(target, link)
}

// --- wiki-sync 块解析 ---

type wikiSyncDecl struct {
	Paths []string `json:"paths"`
	Intro string   `json:"intro"`
}

func parseWikiSync(projectRoot string) (*wikiSyncDecl, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, "AGENTS.md"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s 下没有 AGENTS.md，请先按规约添加 wiki-sync 块", projectRoot)
	}
	if err != nil {
		return nil, err
	}
	m := wikiSyncRe.FindSubmatch(data)
	if m == nil {
		return nil, fmt.Errorf("%s 的 AGENTS.md 中没有 wiki-sync 块", projectRoot)
	}
	var decl wikiSyncDecl
	if err := json.Unmarshal(m[1], &decl); err != nil {
		return nil, fmt.Errorf("wiki-sync 块 JSON 解析失败: %w", err)
	}
	if len(decl.Paths) == 0 {
		return nil, errors.New("wiki-sync 块的 paths 为空")
	}
	return &decl, nil
}

// --- 子命令 ---

func cmdRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	dir := fs.String("dir", "", "项目根目录（可省略，改用位置参数或当前目录）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := wikiRoot()
	dirArg := "."
	switch {
	case fs.NArg() == 1:
		dirArg = fs.Arg(0)
	case *dir != "":
		dirArg = *dir
	case fs.NArg() > 1:
		return errors.New("用法: wiki register [目录]")
	}
	abs, err := filepath.Abs(dirArg)
	if err != nil {
		return err
	}
	decl, err := parseWikiSync(abs)
	if err != nil {
		return err
	}

	// 校验每个接入路径真实存在
	var absPaths []string
	for _, p := range decl.Paths {
		t := filepath.Join(abs, filepath.FromSlash(p))
		if fi, err := os.Stat(t); err != nil || !fi.IsDir() {
			return fmt.Errorf("接入路径不存在或不是目录: %s", t)
		}
		absPaths = append(absPaths, p)
	}

	name := badNameChars.ReplaceAllString(filepath.Base(abs), "_")
	reg, err := loadRegistry(root)
	if err != nil {
		return err
	}
	entry := ProjectEntry{Name: name, Root: abs, Intro: decl.Intro, Paths: absPaths}

	taken := map[string]bool{}
	if err := os.MkdirAll(filepath.Join(root, "projects", name), 0o755); err != nil {
		return err
	}
	for _, p := range absPaths {
		target, _ := filepath.Abs(filepath.Join(abs, filepath.FromSlash(p)))
		link := linkPathFor(root, entry, p, taken)
		if err := createLink(target, link); err != nil {
			return err
		}
		fmt.Printf("链接 %s -> %s\n", link, target)
	}

	// upsert
	replaced := false
	for i := range reg.Projects {
		if reg.Projects[i].Name == name {
			reg.Projects[i] = entry
			replaced = true
		}
	}
	if !replaced {
		reg.Projects = append(reg.Projects, entry)
	}
	if err := saveRegistry(root, reg); err != nil {
		return err
	}
	fmt.Printf("已注册项目 %s（%s），共 %d 个接入路径\n", name, abs, len(absPaths))
	return nil
}

func cmdList(args []string) error {
	root := wikiRoot()
	reg, err := loadRegistry(root)
	if err != nil {
		return err
	}
	if len(reg.Projects) == 0 {
		fmt.Println("尚未注册任何项目。在目标项目 AGENTS.md 加 wiki-sync 块后执行 wiki register。")
		return nil
	}
	fmt.Printf("%-20s %-6s %s\n", "项目", "状态", "目录")
	for _, p := range reg.Projects {
		status, detail := "正常", ""
		dead := 0
		taken := map[string]bool{}
		for _, rel := range p.Paths {
			target := filepath.Join(p.Root, filepath.FromSlash(rel))
			link := linkPathFor(root, p, rel, taken)
			if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
				dead++
			} else if _, err := os.Lstat(link); err != nil {
				dead++
			}
		}
		switch {
		case !dirExists(p.Root):
			status, detail = "dead", "项目目录已不存在"
		case dead > 0:
			status, detail = "失效", fmt.Sprintf("%d 个链接异常（wiki sync --fix 修复）", dead)
		}
		fmt.Printf("%-20s %-6s %s %s\n", p.Name, status, p.Root, detail)
	}
	return nil
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// syncProject 校验（并可选修复）一个项目的全部链接，返回剩余问题数。
func syncProject(root string, e ProjectEntry, fix bool) []string {
	var problems []string
	if !dirExists(e.Root) {
		problems = append(problems, fmt.Sprintf("%s: 项目目录 %s 已不存在（wiki unlink %s 移除，或恢复目录）", e.Name, e.Root, e.Name))
		return problems
	}
	taken := map[string]bool{}
	for _, rel := range e.Paths {
		target := filepath.Join(e.Root, filepath.FromSlash(rel))
		link := linkPathFor(root, e, rel, taken)
		if !dirExists(target) {
			problems = append(problems, fmt.Sprintf("%s: 接入路径 %s 已不存在", e.Name, target))
			continue
		}
		if resolved, err := filepath.EvalSymlinks(link); err == nil && samePath(resolved, target) {
			continue
		}
		if fix {
			if err := createLink(target, link); err != nil {
				problems = append(problems, fmt.Sprintf("%s: 修复链接 %s 失败: %v", e.Name, link, err))
			} else {
				fmt.Printf("已修复 %s -> %s\n", link, target)
			}
		} else {
			problems = append(problems, fmt.Sprintf("%s: 链接 %s 失效", e.Name, link))
		}
	}
	return problems
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func cmdSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fix := fs.Bool("fix", false, "重建失效链接")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := wikiRoot()
	reg, err := loadRegistry(root)
	if err != nil {
		return err
	}
	if len(reg.Projects) == 0 {
		fmt.Println("尚未注册任何项目。")
		return nil
	}
	var problems []string
	for _, p := range reg.Projects {
		problems = append(problems, syncProject(root, p, *fix)...)
	}
	if len(problems) == 0 {
		fmt.Printf("全部 %d 个项目链接健康。\n", len(reg.Projects))
		return nil
	}
	return fmt.Errorf("发现 %d 个问题:\n  %s", len(problems), strings.Join(problems, "\n  "))
}

func cmdUnlink(args []string) error {
	fs := flag.NewFlagSet("unlink", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("用法: wiki unlink <项目名>")
	}
	name := fs.Arg(0)
	root := wikiRoot()
	reg, err := loadRegistry(root)
	if err != nil {
		return err
	}
	idx := -1
	for i, p := range reg.Projects {
		if p.Name == name {
			idx = i
		}
	}
	if idx < 0 {
		return fmt.Errorf("项目 %s 未注册", name)
	}
	projDir := filepath.Join(root, "projects", name)
	if entries, err := os.ReadDir(projDir); err == nil {
		for _, en := range entries {
			// 只删链接本身，绝不递归进目标目录
			_ = os.Remove(filepath.Join(projDir, en.Name()))
		}
	}
	_ = os.RemoveAll(projDir)
	reg.Projects = append(reg.Projects[:idx], reg.Projects[idx+1:]...)
	if err := saveRegistry(root, reg); err != nil {
		return err
	}
	fmt.Printf("已移除项目 %s\n", name)
	return nil
}
