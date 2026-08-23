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

// wikiRoot 的解析见下方函数；开源工具不含任何硬编码个人路径。

// wikiRoot 解析顺序（开源可移植，无任何硬编码个人路径）：
//
//  1. WIKI_ROOT 环境变量（显式指定，最高优先）
//  2. wiki.exe 所在目录 —— 存在 index.md 标记时认定（clone 本仓库后 go build 的默认形态）
//  3. 当前目录 —— 存在 index.md 标记时认定（exe 在 PATH 上、人在 wiki 根里执行的场景）
//  4. 兜底 exe 所在目录（首次使用时由工具在该目录生成 index.md）
func wikiRoot() string {
	if v := os.Getenv("WIKI_ROOT"); v != "" {
		return v
	}
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
		if dir := resolveWikiRoot(exeDir); dir != "" {
			return dir
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if dir := resolveWikiRoot(wd); dir != "" {
			return dir
		}
	}
	if exeDir != "" {
		return exeDir
	}
	wd, _ := os.Getwd()
	return wd
}

// resolveWikiRoot 判断某目录是否为 wiki 根（含 index.md 注册表标记），是则返回该目录。
func resolveWikiRoot(dir string) string {
	if fi, err := os.Stat(filepath.Join(dir, "index.md")); err == nil && !fi.IsDir() {
		return dir
	}
	return ""
}

// ProjectEntry 登记一个已接入项目：名字、根目录、介绍、摘要、接入的相对路径。
// intro/summary 由 agent 在任意时间补充（改项目 AGENTS.md 的 wiki-sync 块即可，
// Stop hook 会自动把新值同步进注册表）。
type ProjectEntry struct {
	Name    string   `json:"name"`
	Root    string   `json:"root"`
	Intro   string   `json:"intro"`
	Summary string   `json:"summary,omitempty"`
	Paths   []string `json:"paths"`
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
	b.WriteString("> 本文件由 wiki 工具自动生成维护，不要手改。\n\n")
	b.WriteString("| 项目 | 目录 | 介绍 | 接入路径 |\n|---|---|---|---|\n")
	for _, p := range reg.Projects {
		intro := p.Intro
		if intro == "" {
			intro = "（待补充：wiki init --intro）"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", p.Name, p.Root, intro, strings.Join(p.Paths, ", "))
	}
	b.WriteString("\n## 项目摘要（agent 维护，wiki init --summary 可更新）\n\n")
	for _, p := range reg.Projects {
		summary := p.Summary
		if summary == "" {
			summary = "（待补充：wiki init --summary）"
		}
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", p.Name, summary)
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
	name := relPath
	if relPath == "." || relPath == "" {
		// 平铺知识目录：项目根本身接入，链接名用项目名
		name = e.Name
	}
	return filepath.Join(root, "projects", e.Name, linkName(name, taken))
}

// createLink 建立或重建一条链接；链接位置已有真实目录时拒绝动手。
func createLink(target, link string) error {
	if fi, err := os.Lstat(link); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(link); err != nil {
				return fmt.Errorf("移除旧链接 %s 失败: %w", link, err)
			}
		} else {
			return fmt.Errorf("%s 已存在且不是链接，请手动处理", link)
		}
	}
	return makeLink(target, link)
}

// --- wiki-sync 声明块 ---

type wikiSyncDecl struct {
	Paths   []string `json:"paths"`
	Intro   string   `json:"intro"`
	Summary string   `json:"summary,omitempty"`
}

func parseWikiSync(projectRoot string) (*wikiSyncDecl, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, "AGENTS.md"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s 下没有 AGENTS.md", projectRoot)
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

// validatePaths 校验接入路径真实存在且是目录。
func validatePaths(abs string, paths []string) error {
	for _, p := range paths {
		t := filepath.Join(abs, filepath.FromSlash(p))
		if fi, err := os.Stat(t); err != nil || !fi.IsDir() {
			return fmt.Errorf("接入路径不存在或不是目录: %s", t)
		}
	}
	return nil
}

// readmeWarnings 规约要求：每个接入目录用 README.md 索引其中的文档，缺失则告警。
func readmeWarnings(root string, e ProjectEntry) []string {
	var ws []string
	taken := map[string]bool{}
	for _, rel := range e.Paths {
		dir := filepath.Join(e.Root, filepath.FromSlash(rel))
		if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
			ws = append(ws, fmt.Sprintf("%s: 接入目录 %s 缺少 README.md（规约要求其索引目录内文档，请 agent 维护）", e.Name, filepath.Join(e.Root, rel)))
		}
		_ = linkPathFor(root, e, rel, taken) // 消耗 taken，保持与链接命名一致
	}
	return ws
}

// EnsureResult 描述一次接入操作的幂等结果。
type EnsureResult struct {
	Entry           ProjectEntry
	RegistryChanged bool     // 注册表内容有变（新项目、元数据更新、路径变化）
	LinksRepaired   int      // 重建/修复的链接数
	ReadmeWarnings  []string // 接入目录缺 README 索引
}

// ensureRegistered 幂等地把项目接入知识库：校验路径、跳过健康链接、修复失效链接、
// upsert 注册表（无变化则不写盘）。供 register/init/check（Stop hook）共用。
func ensureRegistered(root, abs string, decl *wikiSyncDecl) (*EnsureResult, error) {
	if err := validatePaths(abs, decl.Paths); err != nil {
		return nil, err
	}
	res := &EnsureResult{}
	res.Entry = ProjectEntry{
		Name:    badNameChars.ReplaceAllString(filepath.Base(abs), "_"),
		Root:    abs,
		Intro:   decl.Intro,
		Summary: decl.Summary,
		Paths:   decl.Paths,
	}

	reg, err := loadRegistry(root)
	if err != nil {
		return nil, err
	}
	old, existed := findEntry(reg, res.Entry.Name)
	if !existed || !sameEntry(old, res.Entry) {
		res.RegistryChanged = true
	}

	taken := map[string]bool{}
	if err := os.MkdirAll(filepath.Join(root, "projects", res.Entry.Name), 0o755); err != nil {
		return nil, err
	}
	// 先按声明算出 链接路径 → 目标绝对路径 的映射，再逐个核对/补建，最后清理孤儿
	targets := map[string]string{}
	for _, p := range res.Entry.Paths {
		target, _ := filepath.Abs(filepath.Join(abs, filepath.FromSlash(p)))
		targets[linkPathFor(root, res.Entry, p, taken)] = target
	}
	for link, target := range targets {
		if resolved, err := filepath.EvalSymlinks(link); err == nil && samePath(resolved, target) {
			continue // 链接已健康，不动它
		}
		if err := createLink(target, link); err != nil {
			return nil, err
		}
		res.LinksRepaired++
	}
	// 声明收缩时清理孤儿链接（只删链接本身，绝不碰真实目录）
	if entries, err := os.ReadDir(filepath.Join(root, "projects", res.Entry.Name)); err == nil {
		for _, en := range entries {
			link := filepath.Join(root, "projects", res.Entry.Name, en.Name())
			if _, declared := targets[link]; !declared && en.Type()&os.ModeSymlink != 0 {
				if err := os.Remove(link); err == nil {
					fmt.Fprintf(os.Stderr, "wiki: 已清理不再声明的链接 %s\n", link)
				}
			}
		}
	}

	if res.RegistryChanged {
		if existed {
			for i := range reg.Projects {
				if reg.Projects[i].Name == res.Entry.Name {
					reg.Projects[i] = res.Entry
				}
			}
		} else {
			reg.Projects = append(reg.Projects, res.Entry)
		}
		if err := saveRegistry(root, reg); err != nil {
			return nil, err
		}
	}
	res.ReadmeWarnings = readmeWarnings(root, res.Entry)
	return res, nil
}

func findEntry(reg *Registry, name string) (ProjectEntry, bool) {
	for _, p := range reg.Projects {
		if p.Name == name {
			return p, true
		}
	}
	return ProjectEntry{}, false
}

// findEntryByRoot 按项目根目录精确匹配注册项（Stop hook 据此定位当前项目）。
func findEntryByRoot(reg *Registry, abs string) (ProjectEntry, bool) {
	for _, p := range reg.Projects {
		if samePath(p.Root, abs) {
			return p, true
		}
	}
	return ProjectEntry{}, false
}

func sameEntry(a, b ProjectEntry) bool {
	if a.Name != b.Name || a.Root != b.Root || a.Intro != b.Intro || a.Summary != b.Summary ||
		len(a.Paths) != len(b.Paths) {
		return false
	}
	for i := range a.Paths {
		if a.Paths[i] != b.Paths[i] {
			return false
		}
	}
	return true
}

// --- 子命令 ---

func cmdRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	dir := fs.String("dir", "", "项目根目录（可省略，改用位置参数或当前目录）")
	if err := parseWithPositionals(fs, args); err != nil {
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
	res, err := ensureRegistered(root, abs, decl)
	if err != nil {
		return err
	}
	fmt.Printf("已注册项目 %s（%s），接入路径 %d 个，本次修复链接 %d 个\n", res.Entry.Name, abs, len(res.Entry.Paths), res.LinksRepaired)
	printWarnings(res.ReadmeWarnings)
	return nil
}

func printWarnings(ws []string) {
	for _, w := range ws {
		fmt.Fprintln(os.Stderr, "⚠ "+w)
	}
}

func cmdList(args []string) error {
	root := wikiRoot()
	reg, err := loadRegistry(root)
	if err != nil {
		return err
	}
	if len(reg.Projects) == 0 {
		fmt.Println("尚未注册任何项目。agent 执行 wiki init --paths <目录> 即可接入。")
		return nil
	}
	fmt.Printf("%-20s %-6s %s\n", "项目", "状态", "介绍")
	var allWarnings []string
	for _, p := range reg.Projects {
		status, extra := "正常", ""
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
			status, extra = "dead", "项目目录已不存在"
		case dead > 0:
			status, extra = "失效", fmt.Sprintf("%d 个链接异常（wiki sync --fix 修复）", dead)
		}
		intro := p.Intro
		if intro == "" {
			intro = "（介绍待补充）"
		}
		fmt.Printf("%-20s %-6s %s %s\n", p.Name, status, intro, extra)
		allWarnings = append(allWarnings, readmeWarnings(root, p)...)
	}
	printWarnings(allWarnings)
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
