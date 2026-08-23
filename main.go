package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// parseWithPositionals 宽松解析：允许位置参数出现在旗标前（如 `init <目录> --paths wiki`），
// 内部重排为「旗标在前、位置参数在后」再交给标准 FlagSet。
func parseWithPositionals(fs *flag.FlagSet, args []string) error {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if strings.Contains(a, "=") {
				continue
			}
			if f := fs.Lookup(strings.TrimLeft(a, "-")); f != nil {
				if bv, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !bv.IsBoolFlag() {
					if i+1 < len(args) { // 该旗标需要显式值，吞掉下一个 token
						i++
						flags = append(flags, args[i])
					}
				}
			}
			continue
		}
		pos = append(pos, a)
	}
	return fs.Parse(append(flags, pos...))
}

const version = "0.2.0"

const usage = `wiki — 跨项目知识库 + Hugo 博客发布 CLI (v` + version + `)

三条相互独立的流程：
  ① 同步   Stop hook 自动执行 wiki check，维护知识目录链接与注册表（无需人工）
  ② 元数据 agent 在任意时间补充项目介绍/摘要：wiki init --intro/--summary
  ③ 发布   agent 主动执行 wiki blog new / publish

知识库:
  wiki init [目录] --paths <目录列表> [--intro ...] [--summary ...]
                              接入项目：写入/更新 AGENTS.md 的 wiki-sync 块并注册（首次 --paths 必填）
  wiki register [目录]        只注册（声明块已存在时用，init 已包含此步骤）
  wiki list                   列出已注册项目及健康度
  wiki sync [--fix]           链接健康检查/修复
  wiki unlink <项目名>         移除注册与链接
  wiki check                  Stop hook 入口：有声明块则幂等同步，无则静默（一般无需手动跑）

全局查看（路径规格: 项目/链接/相对路径）:
  wiki ls [项目[/子路径]]      列目录（无参列已接入项目）
  wiki tree [项目] [--depth N] 目录树
  wiki grep <模式> [子路径] [--fixed]   内容搜索，输出可直接喂给 wiki cat
  wiki cat <项目/.../文件>     查看文件

博客（Hugo，仓库/目录可用 wiki config 配置）:
  wiki blog list [--json]     只列 categories/tags 两字段（数据来自懒维护的本地记录 blog.json）
  wiki blog new <flags>       创建文章（slug/文件名 apply 时查重；详见 wiki blog new -h）
  wiki blog publish [文件名]   提交推送（push 失败不重试，需用户手动处理网络）

配置:
  wiki config                 查看生效配置
  wiki config set blogRepo <路径>      Hugo 仓库根（默认 D:\note\daidaiJ.github.io）
  wiki config set blogPosts <路径>     文章目录（默认 <blogRepo>\pandawo\content\post）

环境变量（优先级高于 config.json）: WIKI_ROOT / WIKI_BLOG_REPO / WIKI_BLOG_POSTS
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "register":
		err = cmdRegister(os.Args[2:])
	case "check":
		err = cmdCheck(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "sync":
		err = cmdSync(os.Args[2:])
	case "unlink":
		err = cmdUnlink(os.Args[2:])
	case "ls":
		err = cmdLS(os.Args[2:])
	case "tree":
		err = cmdTree(os.Args[2:])
	case "grep":
		err = cmdGrep(os.Args[2:])
	case "cat":
		err = cmdCat(os.Args[2:])
	case "config":
		err = cmdConfig(os.Args[2:])
	case "blog":
		err = cmdBlog(os.Args[2:])
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
