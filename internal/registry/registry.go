// Package registry 实现知识库的核心域：项目注册表（index.md 持久化）、
// 方案 C 存储（projects/ 真目录 + 项目侧窗口链接）、init/register/list/sync/unlink/bundle。
//
// 声明是集中式的：接入信息只存在 wiki 根的本地注册表，不会随项目 commit/push 泄漏。
// 项目 AGENTS.md 里的 wiki-sync 块仅作为可选 opt-in 被 check 识别，工具不再代写。
package registry

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

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
	"github.com/daidaiJ/my-wiki-repo/internal/config"
)

// ProjectsRootName 是 wiki 根下存放各项目知识正文的目录名（方案 C：真实文件，可进 git）。
const ProjectsRootName = "projects"

// BundleRootName 是 wiki bundle 默认输出目录名（方案 B 快照，按需触发）。
const BundleRootName = "bundle"

// ProjectsRoot 返回统一视图目录 <wikiRoot>/projects。
func ProjectsRoot() string { return filepath.Join(config.WikiRoot(), ProjectsRootName) }

// 存储模式：link（缺省，窗口链接）或 copy（项目侧真目录 + 知识库增量拷贝）。
const (
	ModeLink = "link"
	ModeCopy = "copy"
)

// ProjectEntry 登记一个已接入项目：名字、根目录、介绍、摘要、接入的相对路径。
// intro/summary 由 agent 在任意时间补充（wiki init --intro/--summary），
// 会话退出 hook 把最新值同步进注册表。
// Mode 为空按 link 处理（兼容已有 index.md）。
type ProjectEntry struct {
	Name    string   `json:"name"`
	Root    string   `json:"root"`
	Intro   string   `json:"intro"`
	Summary string   `json:"summary,omitempty"`
	Paths   []string `json:"paths"`
	Mode    string   `json:"mode,omitempty"`
}

// effectiveMode 返回归一化后的存储模式（空值视为 link）。
func (e ProjectEntry) effectiveMode() string {
	if e.Mode == ModeCopy {
		return ModeCopy
	}
	return ModeLink
}

// Registry 是注册表数据，持久化为 index.md 顶部的隐藏 JSON 块。
type Registry struct {
	Projects []ProjectEntry `json:"projects"`
}

// WikiSyncDecl 是一份接入声明（集中注册表中的数据形状，也兼容项目 AGENTS.md 的 opt-in 块）。
type WikiSyncDecl struct {
	Paths   []string `json:"paths"`
	Intro   string   `json:"intro"`
	Summary string   `json:"summary,omitempty"`
	Mode    string   `json:"mode,omitempty"` // link（缺省）或 copy
}

var (
	wikiSyncRe   = regexp.MustCompile(`(?s)<!--\s*wiki-sync\s*(\{.*?\})\s*-->`)
	wikiRegRe    = regexp.MustCompile(`(?s)<!--\s*wiki-registry\s*(\{.*?\})\s*-->`)
	badNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)
)

// --- 注册表持久化：index.md 顶部隐藏 JSON 块为数据源，其余为渲染视图 ---

func registryPath(root string) string { return filepath.Join(root, "index.md") }

// LoadRegistry 读取注册表；index.md 缺失或无注册块时返回空注册表。
func LoadRegistry(root string) (*Registry, error) {
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
	b.WriteString("| 项目 | 目录 | 模式 | 介绍 | 接入路径 |\n|---|---|---|---|---|\n")
	for _, p := range reg.Projects {
		intro := p.Intro
		if intro == "" {
			intro = "（待补充：wiki init --intro）"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", p.Name, p.Root, p.effectiveMode(), intro, strings.Join(p.Paths, ", "))
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

// --- 知识目录链接：优先 symlink，Windows 无权限时降级 junction ---

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
// 注意 "."（项目根即知识库）在 linkPathFor 中已被替换为项目名，不会到这里。
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
		name = e.Name // 平铺知识目录：项目根本身接入，链接名用项目名
	}
	return filepath.Join(root, ProjectsRootName, e.Name, linkName(name, taken))
}

// createLink 建立或重建一条链接；链接位置已有真实目录时拒绝动手。
func createLink(target, link string) error {
	if _, err := os.Lstat(link); err == nil {
		if isLink(link) {
			if err := os.Remove(link); err != nil {
				return fmt.Errorf("移除旧链接 %s 失败: %w", link, err)
			}
		} else {
			return fmt.Errorf("%s 已存在且不是链接，请手动处理", link)
		}
	}
	return makeLink(target, link)
}

// --- wiki-sync 声明（项目 AGENTS.md 的可选 opt-in 块） ---

// ParseWikiSync 解析项目 AGENTS.md 中的 wiki-sync 声明块（opt-in 场景）。
func ParseWikiSync(projectRoot string) (*WikiSyncDecl, error) {
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
	var decl WikiSyncDecl
	if err := json.Unmarshal(m[1], &decl); err != nil {
		return nil, fmt.Errorf("wiki-sync 块 JSON 解析失败: %w", err)
	}
	if len(decl.Paths) == 0 {
		return nil, errors.New("wiki-sync 块的 paths 为空")
	}
	return &decl, nil
}

// readmeWarnings 规约要求：每个接入目录用 README.md 索引其中的文档，缺失则告警。
func readmeWarnings(e ProjectEntry) []string {
	var ws []string
	for _, rel := range e.Paths {
		dir := filepath.Join(e.Root, filepath.FromSlash(rel))
		if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
			ws = append(ws, fmt.Sprintf("%s: 接入目录 %s 缺少 README.md（规约要求其索引目录内文档，请 agent 维护）", e.Name, dir))
		}
	}
	return ws
}

// EnsureResult 描述一次接入操作的幂等结果。
type EnsureResult struct {
	Entry           ProjectEntry // 最终登记的注册项
	RegistryChanged bool         // 注册表内容有变（新项目、元数据更新、路径变化）
	LinksRepaired   int          // 重建/修复的窗口链接数
	Migrated        int          // 本次从项目真目录迁入知识库的路径数
	Synced          int          // 拷贝模式下本次发生增量同步的知识路径数
	GitignoreAction string       // created / appended / 空（未写盘）
	ReadmeWarnings  []string     // 接入目录缺 README 索引
}

// EnsureRegistered 幂等地把项目接入知识库（方案 C）：
// link 模式（缺省）先把知识正文迁入 projects/<项目>/，再把项目侧知识目录换成窗口链接；
// copy 模式项目侧保持真目录，知识库存增量合并拷贝；
// 声明收缩时只摘窗口链接与残留正向链接，绝不删知识库真目录；
// upsert 注册表（无变化则不写盘）。供 init/register/check 共用。
func EnsureRegistered(root, abs string, decl *WikiSyncDecl) (*EnsureResult, error) {
	if decl.Mode != "" && decl.Mode != ModeLink && decl.Mode != ModeCopy {
		return nil, fmt.Errorf("未知存储模式 %q（可用 link 或 copy）", decl.Mode)
	}
	res := &EnsureResult{}
	res.Entry = ProjectEntry{
		Name:    entryNameFor(abs),
		Root:    abs,
		Intro:   decl.Intro,
		Summary: decl.Summary,
		Paths:   decl.Paths,
		Mode:    decl.Mode,
	}

	reg, err := LoadRegistry(root)
	if err != nil {
		return nil, err
	}
	old, existed := findEntry(reg, res.Entry.Name)
	if !existed || !sameEntry(old, res.Entry) {
		res.RegistryChanged = true
	}

	if err := os.MkdirAll(filepath.Join(root, ProjectsRootName, res.Entry.Name), 0o755); err != nil {
		return nil, err
	}

	taken := map[string]bool{}
	stores := map[string]string{}
	for _, p := range res.Entry.Paths {
		store := linkPathFor(root, res.Entry, p, taken)
		projPath, _ := filepath.Abs(filepath.Join(abs, filepath.FromSlash(p)))
		stores[store] = projPath

		var changed bool
		switch {
		case res.Entry.Mode == ModeCopy:
			if p == "." || p == "" {
				return nil, fmt.Errorf("拷贝模式不支持平铺接入（.）: %s", abs)
			}
			changed, err = ensureCopyMode(store, projPath, true)
		case p == "." || p == "":
			changed, err = ensureForwardLink(store, projPath)
		default:
			wasLegacy := p != "." && p != "" && legacyForward(store, projPath)
			storeWasReal := isRealDir(store)
			changed, err = ensureInverted(store, projPath, true)
			if err == nil && changed {
				res.LinksRepaired++
				if wasLegacy || (!storeWasReal && isRealDir(store)) {
					res.Migrated++
				}
			}
		}
		if err != nil {
			return nil, err
		}
		if changed && res.Entry.Mode == ModeCopy {
			res.Synced++
		}
	}

	if existed {
		oldTaken := map[string]bool{}
		for _, p := range old.Paths {
			oldStore := linkPathFor(root, old, p, oldTaken)
			if _, still := stores[oldStore]; still {
				continue
			}
			if p != "." && p != "" {
				pp := filepath.Join(abs, filepath.FromSlash(p))
				if isLink(pp) && sameResolved(pp, oldStore) {
					if err := os.Remove(pp); err == nil {
						fmt.Fprintf(os.Stderr, "wiki: 已摘除不再声明的窗口链接 %s\n", pp)
					}
				}
			}
			if isLink(oldStore) {
				if err := os.Remove(oldStore); err == nil {
					fmt.Fprintf(os.Stderr, "wiki: 已清理不再声明的链接 %s\n", oldStore)
				}
			}
		}
	}

	// 拷贝模式正文归项目 git 管，不写 .gitignore；仅链接模式维护
	if gitignoreEnabled(root) && res.Entry.Mode != ModeCopy {
		action, err := ensureProjectGitignore(abs, res.Entry.Paths)
		if err != nil {
			return nil, fmt.Errorf("维护项目 .gitignore 失败: %w", err)
		}
		res.GitignoreAction = action
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
	res.ReadmeWarnings = readmeWarnings(res.Entry)
	return res, nil
}

// entryNameFor 由项目根目录推导注册名（basename，非法字符替换为下划线）。
func entryNameFor(abs string) string {
	return badNameChars.ReplaceAllString(filepath.Base(abs), "_")
}

func findEntry(reg *Registry, name string) (ProjectEntry, bool) {
	for _, p := range reg.Projects {
		if p.Name == name {
			return p, true
		}
	}
	return ProjectEntry{}, false
}

// findEntryByRoot 按项目根目录精确匹配注册项（hook 据此定位当前项目）。
func findEntryByRoot(reg *Registry, abs string) (ProjectEntry, bool) {
	for _, p := range reg.Projects {
		if cli.SamePath(p.Root, abs) {
			return p, true
		}
	}
	return ProjectEntry{}, false
}

func sameEntry(a, b ProjectEntry) bool {
	if a.Name != b.Name || a.Root != b.Root || a.Intro != b.Intro || a.Summary != b.Summary ||
		a.effectiveMode() != b.effectiveMode() || len(a.Paths) != len(b.Paths) {
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

func printWarnings(ws []string) {
	for _, w := range ws {
		fmt.Fprintln(os.Stderr, "⚠ "+w)
	}
}

// CmdList 列出已注册项目、链接健康度与 README 缺失告警。
func CmdList(args []string) error {
	root := config.WikiRoot()
	reg, err := LoadRegistry(root)
	if err != nil {
		return err
	}
	if len(reg.Projects) == 0 {
		fmt.Println("尚未注册任何项目。agent 执行 wiki init --paths <目录> 即可接入。")
		return nil
	}
	fmt.Printf("%-20s %-6s %-6s %s\n", "项目", "状态", "模式", "介绍")
	var allWarnings []string
	for _, p := range reg.Projects {
		status, extra := "正常", ""
		dead, pending := 0, 0
		taken := map[string]bool{}
		for _, rel := range p.Paths {
			projPath := filepath.Join(p.Root, filepath.FromSlash(rel))
			store := linkPathFor(root, p, rel, taken)
			if p.Mode == ModeCopy {
				// 拷贝模式健康 = 两侧均真目录；项目侧出现链接或缺失为异常
				switch {
				case rel == "." || rel == "" || isLink(projPath) || !cli.DirExists(projPath) || isLink(store):
					dead++
				case !isRealDir(store):
					pending++
				}
				continue
			}
			if rel == "." || rel == "" {
				if !cli.DirExists(p.Root) {
					dead++
				} else if !isLink(store) {
					dead++
				}
				continue
			}
			switch {
			case invertedHealthy(store, projPath):
				// ok
			case legacyForward(store, projPath) || (isRealDir(projPath) && !isRealDir(store)):
				pending++
			default:
				dead++
			}
		}
		switch {
		case !cli.DirExists(p.Root):
			status, extra = "dead", "项目目录已不存在"
		case dead > 0:
			status, extra = "失效", fmt.Sprintf("%d 个路径异常（wiki sync --fix 修复）", dead)
		case pending > 0:
			status, extra = "待迁移", fmt.Sprintf("%d 个知识目录仍在项目内（wiki sync --fix 迁入知识库）", pending)
		}
		intro := p.Intro
		if intro == "" {
			intro = "（介绍待补充）"
		}
		fmt.Printf("%-20s %-6s %-6s %s %s\n", p.Name, status, p.effectiveMode(), intro, extra)
		allWarnings = append(allWarnings, readmeWarnings(p)...)
	}
	printWarnings(allWarnings)
	return nil
}

// syncProject 校验（并可选修复）一个项目的全部知识路径，返回剩余问题。
func syncProject(root string, e ProjectEntry, fix bool) []string {
	var problems []string
	if !cli.DirExists(e.Root) {
		problems = append(problems, fmt.Sprintf("%s: 项目目录 %s 已不存在（wiki unlink %s 移除，或恢复目录）", e.Name, e.Root, e.Name))
		return problems
	}
	taken := map[string]bool{}
	for _, rel := range e.Paths {
		projPath := filepath.Join(e.Root, filepath.FromSlash(rel))
		store := linkPathFor(root, e, rel, taken)
		if e.Mode == ModeCopy {
			switch {
			case rel == "." || rel == "":
				problems = append(problems, fmt.Sprintf("%s: 拷贝模式不支持平铺接入（.）", e.Name))
			case isLink(projPath) || !cli.DirExists(projPath):
				problems = append(problems, fmt.Sprintf("%s: %s 项目侧缺失或是链接，与拷贝模式不符", e.Name, rel))
			case isLink(store) || !isRealDir(store):
				if fix {
					if _, err := ensureCopyMode(store, projPath, true); err != nil {
						problems = append(problems, fmt.Sprintf("%s: 同步 %s 失败: %v", e.Name, rel, err))
					} else {
						fmt.Printf("已拷贝同步 %s（知识库 %s）\n", e.Name+"/"+rel, store)
					}
				} else {
					problems = append(problems, fmt.Sprintf("%s: %s 知识库拷贝缺失（wiki sync --fix 首次拷贝）", e.Name, rel))
				}
			default:
				if fix {
					if _, err := ensureCopyMode(store, projPath, true); err != nil {
						problems = append(problems, fmt.Sprintf("%s: 同步 %s 失败: %v", e.Name, rel, err))
					}
				}
			}
			continue
		}
		if rel == "." || rel == "" {
			if resolved, err := filepath.EvalSymlinks(store); err == nil && cli.SamePath(resolved, e.Root) && isLink(store) {
				continue
			}
			if fix {
				if _, err := ensureForwardLink(store, e.Root); err != nil {
					problems = append(problems, fmt.Sprintf("%s: 修复平铺链接 %s 失败: %v", e.Name, store, err))
				} else {
					fmt.Printf("已修复 %s -> %s\n", store, e.Root)
				}
			} else {
				problems = append(problems, fmt.Sprintf("%s: 平铺链接 %s 失效", e.Name, store))
			}
			continue
		}
		if invertedHealthy(store, projPath) {
			continue
		}
		if fix {
			if _, err := ensureInverted(store, projPath, true); err != nil {
				problems = append(problems, fmt.Sprintf("%s: 同步 %s 失败: %v", e.Name, rel, err))
			} else {
				fmt.Printf("已同步 %s（知识库 %s，窗口 %s）\n", e.Name+"/"+rel, store, projPath)
			}
		} else if legacyForward(store, projPath) || isRealDir(projPath) {
			problems = append(problems, fmt.Sprintf("%s: %s 待迁入知识库（wiki sync --fix）", e.Name, rel))
		} else {
			problems = append(problems, fmt.Sprintf("%s: 路径 %s 异常", e.Name, rel))
		}
	}
	if fix && gitignoreEnabled(root) {
		if _, err := ensureProjectGitignore(e.Root, e.Paths); err != nil {
			problems = append(problems, fmt.Sprintf("%s: 维护 .gitignore 失败: %v", e.Name, err))
		}
	}
	return problems
}

// CmdSync 知识路径健康检查；--fix 执行迁移（先拷正文再换窗口链接）并修复失效窗口。
func CmdSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fix := fs.Bool("fix", false, "迁入知识库并重建窗口链接")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := config.WikiRoot()
	reg, err := LoadRegistry(root)
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
		fmt.Printf("全部 %d 个项目知识路径健康。\n", len(reg.Projects))
		return nil
	}
	return fmt.Errorf("发现 %d 个问题:\n  %s", len(problems), strings.Join(problems, "\n  "))
}

// CmdUnlink 移除项目注册与项目侧窗口链接。知识库正文默认保留；--purge 才删除。
func CmdUnlink(args []string) error {
	fs := flag.NewFlagSet("unlink", flag.ContinueOnError)
	purge := fs.Bool("purge", false, "同时删除知识库中的正文")
	if err := cli.ParseWithPositionals(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("用法: wiki unlink <项目名> [--purge]")
	}
	name := fs.Arg(0)
	root := config.WikiRoot()
	reg, err := LoadRegistry(root)
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
	entry := reg.Projects[idx]
	for _, rel := range entry.Paths {
		if rel == "." || rel == "" {
			continue
		}
		pp := filepath.Join(entry.Root, filepath.FromSlash(rel))
		if isLink(pp) {
			if err := os.Remove(pp); err != nil {
				fmt.Fprintf(os.Stderr, "wiki: 摘除窗口链接 %s 失败: %v\n", pp, err)
			}
		}
	}
	projDir := filepath.Join(root, ProjectsRootName, name)
	if *purge {
		if err := os.RemoveAll(projDir); err != nil {
			return fmt.Errorf("删除知识库正文 %s 失败: %w", projDir, err)
		}
		fmt.Printf("已移除项目 %s（含知识库正文）\n", name)
	} else {
		// 只清知识库目录里残留的正向链接，真目录一律保留
		if entries, err := os.ReadDir(projDir); err == nil {
			for _, en := range entries {
				p := filepath.Join(projDir, en.Name())
				if isLink(p) {
					_ = os.Remove(p)
				}
			}
		}
		fmt.Printf("已移除项目 %s 的窗口链接与注册；知识正文保留在 %s（--purge 删除）\n", name, projDir)
	}
	reg.Projects = append(reg.Projects[:idx], reg.Projects[idx+1:]...)
	if err := saveRegistry(root, reg); err != nil {
		return err
	}
	return nil
}
