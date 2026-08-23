# my-wiki

跨项目知识库统一入口 + Hugo 博客发布 CLI。

## 为什么

分项目让智能体调研/分析产出的 wiki、issue 类文档天然碎片化：散落在各项目目录里，没有统一入口，也犯不上为每个项目 fork 远程仓库维护一份 wiki。这里用**符号链接**解决：知识库本体仍在各项目里（单一事实源），`my-wiki/projects/` 下只放链接，`index.md` 登记每个项目属于哪个绝对路径目录和项目介绍。

博客发布同理：Hugo front matter 里只有 `title/slug/categories/tags` 和正文需要动脑，其余全是确定性模板，交给 CLI。

## 快速开始

```bash
go build -o wiki.exe
```

### 接入一个项目的文档目录

在目标项目 `AGENTS.md` 加 wiki-sync 块（见 [AGENTS.md](AGENTS.md)），然后：

```bash
./wiki.exe register D:/CODE/ai/某项目
./wiki.exe list
./wiki.exe sync --fix   # 链接健康检查/修复
```

### 发一篇博客

```bash
./wiki.exe blog list                       # 查已有 categories/tags/slug，优先复用
./wiki.exe blog new \
  --title "标题" --slug my-post \
  --categories "技术笔记" --tags "golang" \
  --name my_post --file body.md --dry-run  # 预览
./wiki.exe blog publish my_post            # 提交推送，Actions 自动部署
```

push 因 GitHub 网络失败不重试，文件已在本地提交，稍后手动 `git push` 即可。

## 布局

- `AGENTS.md` — 面向智能体的完整规约（wiki-sync 协议 + 博客流水线）
- `SKILL.md` — 速查版规约，可 symlink 到 `~/.zcode/skills/wiki/`
- `index.md` — 工具生成的项目索引（数据源在顶部隐藏 JSON 块）
- `projects/` — 各项目文档目录的符号链接（gitignore，机器本地）

本仓库只有本地 git，无 remote——不引入任何同步负担。
