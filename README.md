# my-wiki

**Agent 时代的个人知识库工作流**：分项目调研产出的 wiki/issue 类文档天然碎片化，本工具用「符号链接统一视图 + Stop hook 自动同步 + 集中注册表」把它们管起来，顺带自动化 Hugo 博客发布流水线。

> **开源的是工具与工作流，不是某人的 wiki。** fork 本仓库后，你的知识库数据（`index.md` 注册表、`blog.json` 发布记录）归你自己，用你自己的 git remote 同步管理。

## 核心模型

- **wiki 根**：本仓库的克隆目录（`go build` 出的 `wiki.exe` 就住在这里）。根下有 `index.md`（注册表）、`projects/`（各项目知识目录的符号链接/junction，机器本地、已 gitignore）、`config.json`（你的配置）
- **工具与数据可分离**：不想让知识数据落在工具仓库里的话，把数据目录放别处并设 `WIKI_ROOT` 环境变量指向它（注册表/发布记录/配置/链接全部跟过去；工具仓库保持纯净，随时可开源）
- **知识目录类型名**：默认 `wiki/`、`issues/`（可在 `config.json` 的 `knowledgeDirs` 改），专用名避免与上游官方 `docs/` 混淆；整个目录都是知识的平铺库可声明 `.`
- **集中声明**：接入信息只存在 wiki 根的本地注册表里，**项目仓库零足迹**——声明永远不会随项目 commit/push 泄漏到远程
- **三条解耦流程**：① 同步（Stop hook 全自动）② 元数据补充（agent 任意时间）③ 博客发布（agent 主动）

## 快速开始

```bash
git clone https://github.com/<you>/my-wiki.git && cd my-wiki
go build -o wiki.exe .

# 接入一个项目：省略 --paths 时自动发现项目下的 wiki/ 和 issues/
./wiki.exe init D:/path/to/your-project --intro "一句话介绍" --summary "摘要"

# 全局查看（路径规格：项目/链接/相对路径）
./wiki.exe ls                                    # 已接入项目
./wiki.exe grep "controller"                     # 跨项目内容搜索
./wiki.exe cat myproject/wiki/some-doc.md        # 查看文档
```

## 配置 Stop hook：会话结束自动同步

在 `~/.zcode/cli/config.json`（用户级，对所有工作区生效）加入：

```json
{
  "hooks": {
    "enabled": true,
    "events": {
      "Stop": [
        {
          "hooks": [
            {
              "type": "process",
              "command": "D:\\path\\to\\my-wiki\\wiki.exe",
              "args": ["check"],
              "timeoutMs": 8000,
              "statusMessage": "wiki 知识库自动同步"
            }
          ]
        }
      ]
    }
  }
}
```

每轮 agent 回复结束时 `wiki check` 被自动调用：

- 当前项目已注册 → 幂等维护知识目录链接、自动修复失效链接、同步注册表元数据
- 未注册 → 静默退出，绝不打扰会话（stdout 恒空以满足 hook 的严格 JSON 校验，日志走 stderr）
- 项目 AGENTS.md 里的 `wiki-sync` 声明块（可选 opt-in，工具不再代写）存在时也会被识别接入

Windows 上链接优先符号链接、无权限自动降级 junction，行为一致。

## 用 git 管理你自己的知识

本仓库就是你的知识库根，给它加上你自己的 remote：

```bash
git remote add origin git@github.com:<you>/my-wiki.git
git push -u origin main
```

- 随仓库同步的：`index.md`（项目注册表 + 介绍/摘要）、`blog.json`（已发布文章四字段记录）、`config.json`、规约文档
- 不同步的：`projects/`（机器本地链接，已 gitignore，换机器后 `wiki sync --fix` 重建）
- **工具从不自动 push**；唯一向远程推送的路径是你显式执行的 `wiki blog publish`

## 配置参考

环境变量（优先级最高）：`WIKI_ROOT`（wiki 根目录）、`WIKI_BLOG_REPO`、`WIKI_BLOG_POSTS`、`WIKI_KNOWLEDGE_DIRS`（逗号分隔）

`config.json`（在 wiki 根下，`wiki config set <键> <值>` 写入）：

| 键 | 说明 | 默认 |
|---|---|---|
| `blogRepo` | Hugo 博客 git 仓库根（用博客功能必配） | 无 |
| `blogPosts` | 文章目录 | `<blogRepo>\pandawo\content\post` |
| `knowledgeDirs` | 知识目录类型名 | `wiki, issues` |

wiki 根解析顺序：`WIKI_ROOT` → wiki.exe 所在目录（含 `index.md` 标记）→ 当前目录（含标记）→ exe 所在目录兜底。仓库整体搬移无需改配置。

## 规约要点（完整版见 [AGENTS.md](AGENTS.md)）

- 知识目录用专用名（默认 `wiki/`、`issues/`），勿接入上游官方 `docs/`；上游自带同名目录的先干掉或整理合并，专用名归个人知识
- 每个接入目录必须有 `README.md` 索引目录内文档，由 agent 维护（工具缺失时告警）
- 事后补元数据：`wiki init <项目> --intro "..." --summary "..."`，任意时间、paths 省略保留

## 博客发布（Hugo）

```bash
wiki config set blogRepo D:/path/to/hugo-repo    # 首次
wiki blog list                                     # 已有 categories/tags（优先复用，防碎片化）
wiki blog new --title "标题" --slug kebab-case --categories "A,B" --tags "x,y" --name file_name --file body.md
wiki blog publish file_name                        # 提交推送；push 网络失败不重试，手动处理
```

`blog list` 只列 categories/tags 两字段（数据来自懒维护的本地记录 `blog.json`）；title/slug 冲突罕见，在 `new` 时直接报错。front matter 确定性字段全部自动生成。

## 跨工具规约注入

```bash
wiki inject                 # 把规约引导段注入 ~/.qwen/QWEN.md（标记锚定：无则追加/有则原位替换/一致跳过）
wiki inject --remove        # 摘除
```

升级规约后改 `guide.go` 重新构建，重跑 `wiki inject` 即全量更新。

## 命令速查

```
init / register / list / sync [--fix] / unlink / check     知识库
ls / tree / grep / cat                                      全局查看
blog list|new|publish                                       博客
config [set ...] / inject [--remove]                        配置与注入
```
