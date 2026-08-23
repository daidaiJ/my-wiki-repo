# my-wiki

**一个把散落在各项目里的调研笔记统一管起来的命令行工具**——通过目录链接生成全局视图，配合 coding agent 的钩子自动维护，顺带把 Hugo 博客发布里所有机械性的部分自动化。

## 为什么需要它

如果你经常让 Claude Code / Codex / Qwen Code 这类 agent 做源码调研和方案分析，大概率会遇到同一个问题：产出物（调研 wiki、issue 分析、踩坑记录）散落在十几个仓库的角落里，格式不一、无人索引、想找的时候不知道在哪。为每个项目 fork 一个 wiki 仓库又太重。

my-wiki 的做法是**不动你的文档**：笔记继续留在各自项目里（单一事实源），工具只在统一目录下维护一组目录链接，再用一张本地注册表登记每个项目的位置和介绍。任何时刻 `grep` 一下就能跨项目检索，而各项目仓库保持零改动——不会把你的个人知识配置带上远程。

## 适用场景

**适合：**

- 多仓库并行调研，想在一处检索所有笔记的人
- 用 coding agent 产出文档，希望每轮会话结束时知识目录自动纳入管理的人
- 有个人 Hugo 博客，厌倦了手写 front matter 和手动查分类的人

**不适合：**

- 团队共享知识库——注册表是单机本地的，没有多用户同步
- 需要开箱即用的远程同步——本工具刻意本地优先；要跨机器就用你自己的 git remote（见进阶）

## 依赖

| 依赖 | 何时需要 | 说明 |
|---|---|---|
| Go ≥ 1.25 | 仅构建时 | 唯一第三方库是 `gopkg.in/yaml.v3`，产出单二进制 |
| git | 仅 `blog publish` | 本地 commit + push |
| Hugo | **不需要** | 文章文件直写，部署交给博客仓库自己的 CI |
| 符号链接权限 | 无要求 | Linux/macOS 原生支持；Windows 无需管理员权限（自动降级为 junction） |

运行平台：Windows / Linux / macOS。

## 快速开始

```bash
git clone https://github.com/daidaiJ/my-wiki-repo.git
cd my-wiki-repo && go build -o wiki .

# 接入一个项目（自动发现项目下的 wiki/ 和 issues/ 目录）
./wiki init /path/to/some-project --intro "一句话介绍"

# 跨项目检索
./wiki ls                              # 已接入项目
./wiki grep "controller reconcile"     # 全局内容搜索
./wiki cat some-project/wiki/xxx.md    # 查看命中的文档
```

就这么多了。接入的项目多了之后，装上 agent 钩子（下一节），之后的一切都是自动的。

## 接入你的 Agent（自动同步）

wiki 只需要 agent 做一件事：**每轮回复结束时执行一次 `wiki check`**。它被设计为对任何钩子机制都安全：

- stdout 恒为空（部分工具会把 stdout 当 JSON 严格校验）
- 日志全部走 stderr，内部错误不改变退出码
- 不修改当前项目仓库的任何文件

效果：已注册的项目自动维护链接、修复失效链接、清理声明变更后的残留链接；未注册的项目完全静默，不打扰会话。

**ZCode**（Stop 钩子，`~/.zcode/cli/config.json`）：

```json
{
  "hooks": {
    "enabled": true,
    "events": {
      "Stop": [
        { "hooks": [ { "type": "process", "command": "/path/to/my-wiki/wiki", "args": ["check"], "timeoutMs": 8000 } ] }
      ]
    }
  }
}
```

**Claude Code**（Stop 钩子，`~/.claude/settings.json`）：

```json
{
  "hooks": {
    "Stop": [
      { "hooks": [ { "type": "command", "command": "/path/to/my-wiki/wiki check" } ] }
    ]
  }
}
```

**Qwen Code / Codex / 其他不支持钩子的工具**：用引导注入代替钩子，agent 会按注入的指引在收尾时自行执行 `wiki check`：

```bash
wiki inject --file ~/.qwen/QWEN.md
wiki inject --file ~/.codex/AGENTS.md
```

`wiki inject` 是标记锚定的：无标记则追加、有则原位替换、内容一致则跳过，重复执行安全。

## 日常使用

```bash
# 知识库
wiki init <项目>                                # 接入（省略 --paths 自动发现 wiki/、issues/）
wiki init <项目> --intro "..." --summary "..."   # 事后补充/更新项目介绍与摘要
wiki list                                       # 项目健康度一览
wiki sync [--fix]                               # 链接检查/修复
wiki unlink <项目>                               # 移除

# 检索（路径规格：项目/链接/相对路径）
wiki ls / wiki tree <项目> / wiki grep <模式> / wiki cat <路径>

# 博客（可选功能，见下）
wiki blog list / wiki blog new ... / wiki blog publish <文件名>
```

两条规约值得知道：

- 知识目录用专用名（默认 `wiki/`、`issues/`，可配置），**不要**接入上游项目的官方 `docs/`；克隆的上游项目若自带同名目录，先删掉或整理合并——专用名归个人知识
- 每个接入目录需要一个 `README.md` 索引其中的文档（工具会持续提醒缺失的 agent 补上）

## 博客发布（可选）

面向有个人 Hugo 博客的用户。`wiki config set blogRepo <仓库路径>` 后：

```bash
wiki blog list                      # 已有 categories/tags 及使用频次——新文章优先复用，防止分类碎片化
wiki blog new \
  --title "标题" --slug english-kebab \
  --categories "技术笔记" --tags "go,k8s" \
  --name my_post --file body.md     # front matter 全部自动生成；--dry-run 可预览
wiki blog publish my_post           # git add+commit+push，博客仓库的 CI 负责构建部署
```

push 失败不做重试，原始错误透传出来，人工网络环境下手动补一次 `git push` 即可（文章已本地提交）。已发布文章的四字段元数据懒维护在本地 `blog.json`。

## 进阶配置

**工具与数据分离。** 默认数据（注册表 `index.md`、发布记录 `blog.json`、配置 `config.json`、链接目录 `projects/`）就放在本仓库克隆目录；不想混在工具仓库里的话，把数据挪到别处并设置 `WIKI_ROOT` 指向它。wiki 根解析顺序：`WIKI_ROOT` → wiki 可执行文件所在目录（含 `index.md` 标记）→ 当前目录（含标记）→ 可执行文件目录兜底。

**用 git 管理你自己的知识。** 本仓库本身就是知识库载体，加上你自己的 remote 即可跨机器同步 `index.md` / `blog.json` / 规约文档；`projects/` 是机器本地的链接（已 gitignore），换机器后 `wiki sync --fix` 重建。工具从不自动 push。注意：公开仓库的 fork 默认公开，私有知识请推送到 private remote 而不是 fork。

**配置项。** 优先级：环境变量 > `config.json`（wiki 根下，`wiki config set <键> <值>`）> 默认值。

| config.json 键 | 环境变量 | 默认 | 说明 |
|---|---|---|---|
| `blogRepo` | `WIKI_BLOG_REPO` | 无 | Hugo 博客仓库根，用博客功能必配 |
| `blogPosts` | `WIKI_BLOG_POSTS` | `<blogRepo>/pandawo/content/post` | 文章目录 |
| `knowledgeDirs` | `WIKI_KNOWLEDGE_DIRS` | `wiki, issues` | 知识目录类型名，`wiki init` 自动发现依据 |
| — | `WIKI_ROOT` | 见解析顺序 | wiki 数据根目录 |

完整的 agent 规约与命令契约见 [AGENTS.md](AGENTS.md)。
