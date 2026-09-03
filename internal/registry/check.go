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
//  3. 都没有 → 静默退出，绝不打扰会话
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

	if reg, err := LoadRegistry(root); err == nil {
		if entry, ok := findEntryByRoot(reg, proj); ok {
			return syncEntry(root, entry)
		}
	}
	decl, err := ParseWikiSync(proj)
	if err != nil {
		return nil // 未注册且无声明块：静默
	}
	return syncDecl(root, proj, decl)
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
	return syncDecl(root, entry.Root, &WikiSyncDecl{Paths: entry.Paths, Intro: entry.Intro, Summary: entry.Summary})
}

func logSyncResult(res *EnsureResult) {
	if res.RegistryChanged {
		fmt.Fprintf(os.Stderr, "wiki check: 已更新 %s 的注册信息\n", res.Entry.Name)
	}
	if res.Migrated > 0 {
		fmt.Fprintf(os.Stderr, "wiki check: 已将 %s 的 %d 个知识目录迁入知识库\n", res.Entry.Name, res.Migrated)
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
