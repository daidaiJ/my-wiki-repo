package registry

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
	"github.com/daidaiJ/my-wiki-repo/internal/config"
)

// CmdInit 是 agent 的接入入口：声明与元数据只写入 wiki 根的本地注册表（index.md）。
// link 模式（缺省）把知识正文迁入 projects/<项目>/，再把项目侧知识目录换成窗口链接；
// copy 模式项目侧保持真目录（归项目 git 管），知识库存增量合并拷贝。
// 可选维护项目 .gitignore（config projectGitignore，缺省 true，仅 link 模式）。
//
// --paths 省略时按 config.KnowledgeDirs 自动发现项目下的知识目录；
// intro/summary 可在任意时间事后补充（省略时保留已有值）；
// --mode 仅对新接入项目生效，已注册项目保留原模式。
func CmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dir := fs.String("dir", "", "项目根目录（可省略，改用位置参数或当前目录）")
	paths := fs.String("paths", "", "接入的知识目录，逗号分隔相对路径（省略则自动发现已配置类型名的目录；平铺知识库用 .）")
	intro := fs.String("intro", "", "一句话项目介绍（可事后补充/更新）")
	summary := fs.String("summary", "", "项目摘要，几句话（可事后补充/更新）")
	mode := fs.String("mode", "", "存储模式：link（窗口链接，缺省）或 copy（项目侧真目录，知识库存增量拷贝；缺省取 defaultMode 配置）")
	if err := cli.ParseWithPositionals(fs, args); err != nil {
		return err
	}
	dirArg := "."
	switch {
	case fs.NArg() == 1:
		dirArg = fs.Arg(0)
	case *dir != "":
		dirArg = *dir
	case fs.NArg() > 1:
		return errors.New("用法: wiki init [目录] [--paths <目录列表>] [--intro ...] [--summary ...]")
	}
	abs, err := filepath.Abs(dirArg)
	if err != nil {
		return err
	}

	root := config.WikiRoot()
	reg, err := LoadRegistry(root)
	if err != nil {
		return err
	}
	old, had := findEntryByRoot(reg, abs)

	kdirs := config.KnowledgeDirs()
	decl := &WikiSyncDecl{}
	switch {
	case *paths != "":
		decl.Paths = cli.SplitCSV(*paths)
	case had:
		decl.Paths = old.Paths
	default:
		// 自动发现：项目根下存在哪些已配置的知识目录类型名就接哪些
		decl.Paths = discoverKnowledgeDirs(abs, kdirs)
		if len(decl.Paths) == 0 {
			return fmt.Errorf("未在 %s 发现知识目录（类型名 %s）。用 --paths 显式指定，或在 config.json 的 knowledgeDirs 里调整类型名", abs, strings.Join(kdirs, ", "))
		}
	}
	decl.Intro = cli.Pick(*intro, old.Intro)
	decl.Summary = cli.Pick(*summary, old.Summary)

	// 存储模式解析：显式 --mode > config defaultMode > link；已注册项目保留原模式
	modeArg := strings.TrimSpace(*mode)
	if modeArg == "" {
		modeArg = config.DefaultMode()
	}
	switch modeArg {
	case "", ModeLink, ModeCopy:
		if modeArg == "" {
			modeArg = ModeLink
		}
	default:
		return fmt.Errorf("未知存储模式 %q（可用 link 或 copy）", modeArg)
	}
	if had {
		oldMode := old.effectiveMode()
		if modeArg != oldMode {
			fmt.Fprintf(os.Stderr, "⚠ %s 已按 %s 模式接入，模式切换暂不支持，保留原模式（wiki unlink 后重新 init 可换模式）\n", old.Name, oldMode)
		}
		modeArg = oldMode
	}
	decl.Mode = modeArg
	if modeArg == ModeCopy {
		for _, p := range decl.Paths {
			if p == "." || p == "" {
				return fmt.Errorf("拷贝模式不支持平铺接入（.）：会把整个项目根拷进知识库")
			}
		}
	}

	for _, p := range decl.Paths {
		if p != "." && !containsStr(kdirs, p) {
			fmt.Fprintf(os.Stderr, "⚠ %s 不在配置的知识目录类型名（%s）内，确认不是上游官方文档目录再接入\n", p, strings.Join(kdirs, ", "))
		}
	}

	res, err := EnsureRegistered(root, abs, decl)
	if err != nil {
		return err
	}
	fmt.Printf("已接入项目 %s（%s）\n  声明位置: %s（本地注册表，不会随项目 push 外泄）\n  接入路径: %s\n",
		res.Entry.Name, abs, registryPath(root), strings.Join(decl.Paths, ", "))
	if modeArg == ModeCopy {
		fmt.Printf("  存储: copy 模式——项目侧为真目录（归项目 git 管），知识库 %s 存增量拷贝（永不删文件，Obsidian 侧修改保留）\n",
			filepath.Join(root, ProjectsRootName, res.Entry.Name))
	} else {
		fmt.Printf("  存储: link 模式——%s 下真实文件；项目侧为窗口链接\n",
			filepath.Join(root, ProjectsRootName, res.Entry.Name))
	}
	if res.Migrated > 0 {
		fmt.Printf("  本次迁移 %d 个知识目录（先拷贝再替换项目侧目录）\n", res.Migrated)
	}
	if res.GitignoreAction != "" {
		fmt.Printf("  .gitignore: 已%s知识目录条目\n", map[string]string{"created": "创建并写入", "appended": "追加"}[res.GitignoreAction])
	}
	if len(res.VSCodeFiles) > 0 {
		fmt.Printf("  VSCode: 已生成 %s（用 VSCode 打开 projects 目录，点 .md 直接进预览）\n", strings.Join(res.VSCodeFiles, ", "))
	}
	printWarnings(res.ReadmeWarnings)
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
