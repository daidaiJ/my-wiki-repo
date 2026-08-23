// wiki — 跨项目知识库 + Hugo 博客发布 CLI。
//
// 三条相互独立的流程：① 同步（Stop hook 自动执行 wiki check）
// ② 元数据补充（agent 任意时间 wiki init --intro/--summary）③ 发布（wiki blog）。
package main

import (
	"fmt"
	"os"

	"github.com/daidaiJ/my-wiki-repo/internal/blog"
	"github.com/daidaiJ/my-wiki-repo/internal/config"
	"github.com/daidaiJ/my-wiki-repo/internal/guide"
	"github.com/daidaiJ/my-wiki-repo/internal/registry"
	"github.com/daidaiJ/my-wiki-repo/internal/view"
)

// version 由 release 工作流经 -ldflags "-X main.version=<tag>" 注入；
// 本地 go build 时保持 dev 标记。自带 v 前缀，usage 直接拼接。
var version = "v0.0.0-dev"

var usage = `wiki — 跨项目知识库 + Hugo 博客发布 CLI (` + version + `)

知识库:
  wiki init [目录] [--paths <目录列表>] [--intro ...] [--summary ...]
                              接入项目：声明只写本地注册表（项目仓库零足迹）；省略 --paths 自动发现
  wiki register [目录]        低级命令：注册声明块（AGENTS.md opt-in）已存在的项目
  wiki list                   列出已注册项目及健康度
  wiki sync [--fix]           链接健康检查/修复
  wiki unlink <项目名>         移除注册与链接
  wiki check                  Stop hook 入口：自动同步（一般无需手动跑）

全局查看（路径规格: 项目/链接/相对路径）:
  wiki ls [项目[/子路径]]      列目录（无参列已接入项目）
  wiki tree [项目] [--depth N] 目录树
  wiki grep <模式> [子路径] [--fixed]   内容搜索，输出可直接喂给 wiki cat
  wiki cat <项目/.../文件>     查看文件

博客（Hugo，仓库/目录可用 wiki config 配置）:
  wiki blog list [--json]     只列 categories/tags 两字段（数据来自懒维护的本地记录 blog.json）
  wiki blog new <flags>       创建文章（slug/文件名 apply 时查重；详见 wiki blog new -h）
  wiki blog publish [文件名]   提交推送（push 失败不重试，需用户手动处理网络）

配置与注入:
  wiki config                 查看生效配置
  wiki config set blogRepo <路径>           Hugo 仓库根（用博客功能必配）
  wiki config set blogPosts <路径>          文章目录
  wiki config set hugoBin <路径>            hugo 可执行文件（blog new 用，缺省 PATH 上的 hugo）
  wiki config set hugoSite <路径>           Hugo 站点目录（相对 blogRepo，缺省 pandawo）
  wiki config set knowledgeDirs <逗号列表>  知识目录类型名（默认 wiki,issues）
  wiki inject [--file <指令文件>] [--remove]
                              把规约引导段注入用户级指令文件（默认 ~/.qwen/QWEN.md）

环境变量（优先级高于 config.json）: WIKI_ROOT / WIKI_KNOWLEDGE_DIRS / WIKI_BLOG_REPO / WIKI_BLOG_POSTS / WIKI_HUGO_BIN / WIKI_HUGO_SITE
wiki 根解析：WIKI_ROOT → wiki 可执行文件所在目录（含 index.md 标记）→ 当前目录（含标记）→ 可执行文件目录兜底
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = registry.CmdInit(os.Args[2:])
	case "register":
		err = registry.CmdRegister(os.Args[2:])
	case "check":
		err = registry.CmdCheck(os.Args[2:])
	case "list":
		err = registry.CmdList(os.Args[2:])
	case "sync":
		err = registry.CmdSync(os.Args[2:])
	case "unlink":
		err = registry.CmdUnlink(os.Args[2:])
	case "ls":
		err = view.CmdLS(os.Args[2:])
	case "tree":
		err = view.CmdTree(os.Args[2:])
	case "grep":
		err = view.CmdGrep(os.Args[2:])
	case "cat":
		err = view.CmdCat(os.Args[2:])
	case "config":
		err = config.CmdConfig(os.Args[2:])
	case "inject":
		err = guide.CmdInject(os.Args[2:])
	case "blog":
		err = blog.CmdBlog(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "未知命令 %q\n\n", os.Args[1])
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki: "+err.Error())
		os.Exit(1)
	}
}
