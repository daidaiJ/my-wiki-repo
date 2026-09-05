package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
	"github.com/daidaiJ/my-wiki-repo/internal/config"
)

// CmdCheck 是会话退出 hook 的入口（wiki check），在会话退出（/quit）时被 agent 工具自动调用：
//
//  1. 当前项目已注册（本地注册表按根目录匹配）→ 幂等同步：迁入知识正文、
//     维护窗口链接、清理声明收缩后的窗口、维护注册表与可选 gitignore
//  2. 项目 AGENTS.md 带 wiki-sync 声明块（可选 opt-in）→ 识别并接入
//  3. 未注册且无声明块，但项目根下已存在知识目录 → 自动反转：迁入知识库、
//     原位换成窗口链接（agent 写入后才触发；无知识目录则完全无感）
//
// 约束：hook 的 stdout 会被部分工具按严格 JSON 校验，因此本命令 stdout 恒为空，
// 全部日志走 stderr；任何内部错误都不打断会话（退出码 0）。
func CmdCheck(args []string) error {
	return checkLogic(config.WikiRoot(), projectDirFromEnv())
}

// checkLogic 是 hook 的实际逻辑，抽出来便于测试。
func checkLogic(root, proj string) error {
	// wiki 根自身（或其子目录）无需接入
	if cli.UnderOrEqual(proj, root) {
		return nil
	}

	// 生效范围判定（hookMode + forbidden/includePaths），跳过时说明原因
	if ok, reason := config.HookApplies(root, proj); !ok {
		fmt.Fprintf(os.Stderr, "wiki check: 跳过 %s（%s）\n", proj, reason)
		return nil
	}

	if reg, err := LoadRegistry(root); err == nil {
		if entry, ok := findEntryByRoot(reg, proj); ok {
			return syncEntry(root, entry)
		}
	}
	decl, err := ParseWikiSync(proj)
	if err != nil {
		return autoInvertDecl(root, proj)
	}
	return syncDecl(root, proj, decl)
}

// autoInvertDecl 对生效范围内未注册的项目做按需自动反转：
// 仅当项目根下已存在知识目录（agent/用户在会话中写入过）才迁入知识库并建窗口链接；
// 一个知识目录都没有则完全无感——绝不主动创建空目录，也不写注册表。
// 同名注册项指向其他根目录时拒绝自动反转，避免覆盖显式 init 的声明。
func autoInvertDecl(root, proj string) error {
	paths := discoverKnowledgeDirs(proj, config.KnowledgeDirs())
	if len(paths) == 0 {
		return nil
	}
	if reg, err := LoadRegistry(root); err == nil {
		if old, ok := findEntry(reg, entryNameFor(proj)); ok && !cli.SamePath(old.Root, proj) {
			fmt.Fprintf(os.Stderr, "wiki check: 跳过 %s（与已注册项目 %s（%s）同名，请用 wiki init 显式接入）\n", proj, old.Name, old.Root)
			return nil
		}
	}
	res, err := EnsureRegistered(root, proj, &WikiSyncDecl{Paths: paths, Mode: config.DefaultMode()})
	if err != nil {
		fmt.Fprintf(os.Stderr, "wiki check: 自动反转 %s 失败: %v\n", proj, err)
		return nil
	}
	logSyncResult(res)
	return nil
}

// syncDecl 按声明同步并输出诊断（stdout 恒空）。
func syncDecl(root, proj string, decl *WikiSyncDecl) error {
	res, err := EnsureRegistered(root, proj, decl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wiki check: 同步 %s 失败: %v\n", proj, err)
		return nil
	}
	logSyncResult(res)
	return nil
}

// syncEntry 按注册表现有内容幂等维护链接与元数据。
func syncEntry(root string, entry ProjectEntry) error {
	return syncDecl(root, entry.Root, &WikiSyncDecl{Paths: entry.Paths, Intro: entry.Intro, Summary: entry.Summary, Mode: entry.Mode})
}

func logSyncResult(res *EnsureResult) {
	if res.RegistryChanged {
		fmt.Fprintf(os.Stderr, "wiki check: 已更新 %s 的注册信息\n", res.Entry.Name)
	}
	if res.Migrated > 0 {
		fmt.Fprintf(os.Stderr, "wiki check: 已将 %s 的 %d 个知识目录迁入知识库\n", res.Entry.Name, res.Migrated)
	}
	if res.Synced > 0 {
		fmt.Fprintf(os.Stderr, "wiki check: 已增量同步 %s 的 %d 个知识目录\n", res.Entry.Name, res.Synced)
	}
	if res.LinksRepaired > 0 {
		fmt.Fprintf(os.Stderr, "wiki check: 已修复 %s 的 %d 个窗口链接\n", res.Entry.Name, res.LinksRepaired)
	}
	if res.GitignoreAction != "" {
		fmt.Fprintf(os.Stderr, "wiki check: 已%s %s 的 .gitignore\n", res.GitignoreAction, res.Entry.Name)
	}
	for _, w := range res.ReadmeWarnings {
		fmt.Fprintln(os.Stderr, "wiki check ⚠ "+w)
	}
}

// projectDirFromEnv 取 hook 注入的项目目录环境变量，缺省回退 cwd。
func projectDirFromEnv() string {
	for _, key := range []string{"ZCODE_PROJECT_DIR", "CLAUDE_PROJECT_DIR"} {
		if v := os.Getenv(key); v != "" {
			return filepath.Clean(v)
		}
	}
	wd, _ := os.Getwd()
	return wd
}
