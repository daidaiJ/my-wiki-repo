package registry

import (
	"fmt"
	"os"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
	"github.com/daidaiJ/my-wiki-repo/internal/config"
)

// CmdPrepare 是会话初始化 hook 的入口（wiki prepare），在 agent 打开项目时调用：
//
//  1. 当前项目已注册 → 幂等创建/修复项目侧窗口链接（方案 C），必要时新建空知识目录
//  2. 未注册 → 静默退出，不打扰会话
//
// 与 wiki check 对称：prepare 负责「开工前把窗口链好」，check 负责「收工后维护状态」。
// hook 契约：stdout 恒空、日志走 stderr、内部错误不改变退出码。
func CmdPrepare(args []string) error {
	return prepareLogic(config.WikiRoot(), projectDirFromEnv())
}

// prepareLogic 是初始化 hook 的实际逻辑，抽出来便于测试。
func prepareLogic(root, proj string) error {
	if cli.UnderOrEqual(proj, root) {
		return nil
	}
	reg, err := LoadRegistry(root)
	if err != nil {
		return nil
	}
	entry, ok := findEntryByRoot(reg, proj)
	if !ok {
		return nil // 未注册：静默
	}
	return prepareEntry(root, entry)
}

func prepareEntry(root string, entry ProjectEntry) error {
	res, err := EnsureRegistered(root, entry.Root, &WikiSyncDecl{
		Paths:   entry.Paths,
		Intro:   entry.Intro,
		Summary: entry.Summary,
		Mode:    entry.Mode,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "wiki prepare: 同步 %s 失败: %v\n", entry.Root, err)
		return nil
	}
	logPrepareResult(res)
	return nil
}

func logPrepareResult(res *EnsureResult) {
	if res.LinksRepaired > 0 {
		fmt.Fprintf(os.Stderr, "wiki prepare: 已为 %s 建立/修复 %d 个项目侧窗口链接\n", res.Entry.Name, res.LinksRepaired)
	}
	if res.Migrated > 0 {
		fmt.Fprintf(os.Stderr, "wiki prepare: 已将 %s 的 %d 个知识目录迁入知识库\n", res.Entry.Name, res.Migrated)
	}
	if res.Synced > 0 {
		fmt.Fprintf(os.Stderr, "wiki prepare: 已增量同步 %s 的 %d 个知识目录\n", res.Entry.Name, res.Synced)
	}
	if res.GitignoreAction != "" {
		fmt.Fprintf(os.Stderr, "wiki prepare: 已%s %s 的 .gitignore\n", res.GitignoreAction, res.Entry.Name)
	}
	for _, w := range res.ReadmeWarnings {
		fmt.Fprintln(os.Stderr, "wiki prepare ⚠ "+w)
	}
}
