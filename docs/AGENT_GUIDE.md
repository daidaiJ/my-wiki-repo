> [English](AGENT_GUIDE.en.md)

# my-wiki — Agent Guide

跨项目知识库 CLI（Go）。正文在 `WIKI_ROOT/projects/`，项目侧 `wiki/` / `issues/` 为窗口。单 hook：会话退出 `wiki check`。**不要手改 `index.md` / `blog.json`。**

> 人类阅读版：[HUMAN_GUIDE.md](HUMAN_GUIDE.md)。本仓库工作区规约：[AGENTS.md](../AGENTS.md)。Skill：[SKILL.md](../SKILL.md)。

## 存储

现行 **方案 C（反转）**：知识库真文件，项目侧窗口链接。遇方案 A（正向链接）自动迁到 C。方案 B = `wiki bundle` 快照，不是日常布局。

| mode | 项目侧 | 同步 |
|---|---|---|
| `link`（缺省） | 窗口 → 知识库；写 `.gitignore` | 无需同步 |
| `copy` | 真目录，归项目 git | 项目→知识库增量合并；mtime 新者胜；**永不删文件** |

`--mode` 仅新接入生效。换模式：`unlink` 后再 `init`。不支持平铺接入 `.`。不要接入上游 `docs/`。每个接入目录须有 `README.md` 索引。

路径规格：`项目/链接/相对路径`。agent 仍写 `wiki/note.md`，不必知道知识库绝对路径。

## 流程（互不阻塞）

| 流程 | 何时 | 命令 |
|---|---|---|
| 修复（可选） | 任意时刻 | `wiki prepare` — 只修已注册项目窗口，不创建新接入 |
| 同步 | 会话退出 hook | `wiki check` |
| 元数据 | 任意时刻 | `wiki init --intro/--summary` |
| 发布 | 用户明确要求时 | `wiki blog new` → `wiki blog publish` |

知识工作流：① agent 写入项目 `wiki/` → ② **人**在 Obsidian 校对（不要跳过）→ ③ `blog new` / `publish`。Obsidian 根 = wiki 根，见 [obsidian.md](obsidian.md)。

## 命令

```
wiki init [目录] --paths <目录列表> [--mode copy|link] [--intro ...] [--summary ...]
wiki list
wiki sync [--fix]
wiki unlink <项目名> [--purge]          # 正文默认保留
wiki prepare                            # 可选，不进 hook
wiki check                              # 唯一 hook；stdout 恒空，日志 stderr
wiki bundle [--dir <目录>] [--archive zip|tgz]
wiki ls [项目[/子路径]]
wiki tree [<项目>] [--depth N]
wiki grep <模式> [<子路径>] [--fixed]   # 输出可直接喂 wiki cat
wiki cat <项目/.../文件>
wiki config / wiki config set <键> <值>
wiki inject [--file <指令文件>] [--remove]
```

Windows：symlink，无权限降级 junction。

## Hook

只注册会话退出 `wiki check`。stdout 空、失败不阻塞。已注册项目：新出现的 knowledgeDirs 目录自动并入（先拷正文入库再反转链接，注册路径永不收缩）。未注册且已有知识目录 → 自动反转（写过后才触发，不预建空目录）。同名注册冲突则拒绝。

`hookMode`：`forbiddenList`（缺省，`forbiddenPaths` 及其子孙跳过）/ `whitelist`（仅 `includePaths` 子孙）。env `WIKI_HOOK_MODE` / `WIKI_FORBIDDEN_PATHS` / `WIKI_INCLUDE_PATHS` 优先于 config。

**ZCode** `~/.zcode/cli/config.json`：

```json
{"hooks":{"enabled":true,"events":{"Stop":[{"hooks":[{"type":"process","command":"/path/to/wiki","args":["check"],"timeoutMs":8000}]}]}}}
```

**Claude Code** `~/.claude/settings.json`：

```json
{"hooks":{"SessionEnd":[{"hooks":[{"type":"command","command":"/path/to/wiki check"}]}]}}
```

无 hook 的工具：`wiki inject`（标记锚定，幂等）。目标：`--file` > `injectFile` / `WIKI_INJECT_FILE` > `~/.qwen/QWEN.md`。

wiki 根：`WIKI_ROOT` > exe 目录（有 `index.md`）> cwd > exe 兜底。hook 内联：`cmd /c "set WIKI_ROOT=D:\vault&& wiki.exe check"`。

## 博客（仅用户明确要发布时）

先 `wiki config set blogRepo <绝对路径>`。

**硬规则：**

1. 创建必须 `wiki blog new`，发布必须 `wiki blog publish`。**禁止** Write/Edit 直接往博客仓库 `content/post/` 新建文章。
2. 更新已有文章：只改正文和 `lastmod`，不动 front matter 字段结构；改完 `wiki blog publish <file_name>`。
3. 正文起草到博客仓库之外的临时文件，再 `--file`。
4. `wiki blog list` 后**优先复用**已有 categories/tags，不要新造同义类别。
5. push 失败不重试：告知「已本地提交，请在博客仓库手动 git push」。

```
wiki blog list
wiki blog new --title "..." --slug english-kebab --categories "..." --tags "..." --name file_name --file body.md [--dry-run]
wiki blog publish file_name
```

`--name` / publish 参数不带 `.md`。slug/文件名冲突在 new 步报错（先于 hugo new）。文风走 `tech-blog` skill。

## 配置速查

优先级：env > `config.json` > 默认。

| 键 | env | 默认 |
|---|---|---|
| `blogRepo` | `WIKI_BLOG_REPO` | 无 |
| `blogPosts` | `WIKI_BLOG_POSTS` | `<blogRepo>/content/post` |
| `hugoBin` | `WIKI_HUGO_BIN` | `hugo` |
| `hugoSite` | `WIKI_HUGO_SITE` | `<blogRepo>` |
| `knowledgeDirs` | `WIKI_KNOWLEDGE_DIRS` | `wiki, issues` |
| `hookMode` | `WIKI_HOOK_MODE` | `forbiddenList` |
| `forbiddenPaths` | `WIKI_FORBIDDEN_PATHS` | 空 |
| `includePaths` | `WIKI_INCLUDE_PATHS` | 空 |
| `projectGitignore` | `WIKI_PROJECT_GITIGNORE` | `true` |
| `defaultMode` | `WIKI_DEFAULT_MODE` | `link` |
| `injectFile` | `WIKI_INJECT_FILE` | `~/.qwen/QWEN.md` |
| — | `WIKI_ROOT` | 见解析顺序 |

可选 opt-in：项目 `AGENTS.md` 里 `<!-- wiki-sync {...} -->` 仍被 `check` 识别。

## 陷阱

- 同批次无关；有依赖的操作拆成多次顺序调用。
- CLI 旗标可放位置参数后（宽容解析）。
- copy 模式：项目侧删除的文件会残留在知识库。
- 上游 clone 自带 `wiki/` / `issues/`：`init` / `sync --fix` 会先迁正文再换窗口——先确认那不是上游官方文档。
