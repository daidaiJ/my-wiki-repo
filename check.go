package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cmdCheck 是 Stop hook 的入口（wiki check），在每轮回复结束时被 ZCode 自动调用：
//
//  1. 项目带 wiki-sync 声明块 → 幂等同步：补建/修复知识目录链接、把 AGENTS.md 里
//     后补的 intro/summary 同步进注册表（元数据补充与同步解耦，agent 改完声明块即可）
//  2. 没有声明块 → 静默退出，绝不打扰会话
//
// 约束：hook 的 stdout 会被按严格 JSON 校验，因此本命令 stdout 恒为空，
// 全部日志走 stderr；任何内部错误都不打断会话（退出码 0）。
func cmdCheck(args []string) error {
	return checkLogic(wikiRoot(), projectDirFromEnv())
}

// checkLogic 是 hook 的实际逻辑，抽出来便于测试。
func checkLogic(root, proj string) error {
	// my-wiki 自身（或其子目录）无需接入
	if underOrEqual(proj, root) {
		return nil
	}

	decl, err := parseWikiSync(proj)
	if err != nil {
		return nil // 无 AGENTS.md 或无声明块：静默
	}
	res, err := ensureRegistered(root, proj, decl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wiki check: 同步 %s 失败: %v\n", proj, err)
		return nil
	}
	if res.RegistryChanged {
		fmt.Fprintf(os.Stderr, "wiki check: 已更新 %s 的注册信息\n", res.Entry.Name)
	}
	if res.LinksRepaired > 0 {
		fmt.Fprintf(os.Stderr, "wiki check: 已修复 %s 的 %d 个链接\n", res.Entry.Name, res.LinksRepaired)
	}
	for _, w := range res.ReadmeWarnings {
		fmt.Fprintln(os.Stderr, "wiki check ⚠ "+w)
	}
	return nil
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

func underOrEqual(path, root string) bool {
	p := strings.ToLower(filepath.Clean(path))
	r := strings.ToLower(filepath.Clean(root))
	return p == r || strings.HasPrefix(p+string(filepath.Separator), r+string(filepath.Separator))
}
