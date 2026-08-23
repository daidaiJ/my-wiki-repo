package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

const usage = `wiki — 跨项目知识库 + Hugo 博客发布 CLI (v` + version + `)

用法:
  wiki register [目录]        接入项目：解析其 AGENTS.md 的 wiki-sync 块，建符号链接并登记到 index.md（目录缺省为当前目录）
  wiki list                   列出已注册项目及链接健康度
  wiki sync [--fix]           检查所有链接；--fix 重建失效链接（目标目录已删除的报 dead）
  wiki unlink <项目名>         移除项目注册与链接
  wiki blog list [--json]     列出博客已有 categories/tags/slug（创建文章前先查，复用已有类别）
  wiki blog new <flags>       创建博客文章（front matter 自动生成，详见 wiki blog new -h）
  wiki blog publish [文件名]   提交并推送指定文章（push 失败不重试，需用户手动处理网络）

环境变量:
  WIKI_ROOT         my-wiki 根目录（缺省 D:\CODE\ai\my-wiki）
  WIKI_BLOG_REPO    博客 git 仓库根（缺省 D:\note\daidaiJ.github.io）
  WIKI_BLOG_POSTS   博客文章目录（缺省 <WIKI_BLOG_REPO>\pandawo\content\post）
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "register":
		err = cmdRegister(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "sync":
		err = cmdSync(os.Args[2:])
	case "unlink":
		err = cmdUnlink(os.Args[2:])
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
