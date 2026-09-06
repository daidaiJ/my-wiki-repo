# my-wiki 规约（Agent 必读）

本仓库是跨项目知识库的统一入口 + Hugo 博客发布工具，CLI 二进制为 `wiki`（本仓库 `go build` 产物）。
现行存储是**方案 C（反转）**：`projects/` 下是各项目知识正文（真文件，可进 git）；项目侧 `wiki/`、`issues/` 等为指向知识库的窗口链接。旧版方案 A（知识库正向链接聚合项目）遇到即迁到 C；方案 B 是 `wiki bundle` 按需归档，不是日常布局。`index.md` 是工具生成的项目索引（数据源在顶部隐藏 JSON 块，**不要手改**）。

## 三条相互独立的流程（互不阻塞）

| 流程 | 触发方 | 机制 |
|---|---|---|
| ① 修复（可选） | agent，**任意时间** | `wiki prepare`：已注册项目幂等建立/修复窗口链接，不创建新接入（自动反转全部在 check 触发） |
| ② 同步 | **全自动**（会话退出 hook） | 会话退出（/quit）时执行 `wiki check`：维护窗口链接、迁移正文、同步注册表 |
| ③ 元数据 | agent，**任意时间** | `wiki init --intro/--summary` 更新项目介绍与摘要 |
| ④ 发布 | agent，主动 | `wiki blog new` → `wiki blog publish` |

## 知识工作流（agent 视角）

三段流水线，agent 只负责第一和第三段，中间是人工关卡：

| 阶段 | 执行者 | 产出 |
|---|---|---|
| ① Agent 驱动总结 | agent | 写入项目 `wiki/` 窗口（正文落在知识库 `projects/`） |
| ② Obsidian 查看校对 | 人 | 校对结论（agent 不跳过） |
| ③ Hugo 发布 / git 同步 | agent | 已发布文章（blog new → publish） |

知识库可同时作为 Obsidian 仓库根：`projects/` 下是真文件，Obsidian 直接索引（接入见 `docs/obsidian.md`）。沉淀成博客前，先提示用户在 Obsidian 校对。

## 一、接入声明（集中式注册表 + 方案 C 反转存储）

接入信息（paths/intro/summary/mode）**只存在 wiki 根的本地注册表 `index.md`**。`wiki init` 写注册表、按模式建立存储布局，并维护项目侧窗口。`projectGitignore` 默认 true，会在项目仓创建/追加 `.gitignore` 条目（仅 link 模式）。

**两种存储模式**（`wiki init --mode` 选择，或 `config.json` 的 `defaultMode` 设默认；仅新接入项目生效，已注册项目保留原模式，换模式需 unlink 后重新 init）：

- `link`（缺省）：知识正文迁入 `projects/<项目>/`，项目侧为窗口链接；agent 经链接直写，无同步开销
- `copy`：项目侧保持真目录（正文归项目 git 管，不写 .gitignore），知识库存增量合并拷贝——hook 同步只拷新增/修改（mtime 新者胜），永不删文件，Obsidian 侧校对修改保留；不支持平铺接入（`.`），遇残留链接布局拒绝动手防误覆盖

- 知识目录类型名默认 `wiki/` 与 `issues/`，可在 `config.json` 的 `knowledgeDirs` 配置；`wiki init --paths wiki` 可在目录尚不存在时预建空知识库与窗口
- **上游同名目录冲突**：clone 的上游项目若自带 `wiki/` 或 `issues/`，`wiki init` / `wiki sync --fix` 会先迁移正文再替换为窗口链接
- `docs/` 等官方文档同名目录默认不接入，确需时显式 `--paths` 指定
- 可选 opt-in：项目 `AGENTS.md` 里的 `<!-- wiki-sync {...} -->` 声明块仍被 `wiki check` 识别

## 二、知识库命令

```
wiki init [目录] --paths <目录列表> [--mode copy|link] [--intro ...] [--summary ...]   # 显式接入：声明 paths/模式、补 intro/summary
wiki list                   # 已注册项目 + 健康度（含存储模式）
wiki sync [--fix]           # 健康检查；--fix 迁移/增量同步并重建窗口
wiki unlink <项目名> [--purge]  # 移除注册与窗口（正文默认保留）
wiki prepare                # 可选：建立/修复已注册项目的窗口链接（不创建新目录）
wiki check                  # 会话退出 hook：幂等同步
wiki bundle [--dir <目录>] [--archive zip|tgz]   # 按需克隆/压缩归档（方案 B）
```

Windows 上优先符号链接，无权限自动降级 junction，行为一致。

## 三、全局查看（路径规格：`项目/链接/相对路径`）

```
wiki ls                     # 列已接入项目
wiki ls <项目>[/<子路径>]    # 列目录
wiki tree [<项目>] [--depth N]
wiki grep <模式> [<子路径>] [--fixed]   # 输出 `项目/链接/文件:行号: 内容`，可直接喂给 wiki cat
wiki cat <项目/.../文件>
```

跨项目调研先 `wiki grep` 定位再 `wiki cat` 查看，不必知道各项目绝对路径。

## 四、博客发布流水线（agent 工作流）

目标仓库无内置默认值，**用 `wiki config set blogRepo <你的 Hugo 仓库绝对路径>` 配置**（文章目录缺省 `<blogRepo>/content/post/`，站点在子目录时用 `hugoSite`/`blogPosts` 指定）；push 到 main 后 GitHub Actions 自动部署，发布 = push 成功。

1. **先查已有分类**：`wiki blog list`（只列 categories/tags 两字段，按使用次数降序）——**优先复用高频类别**，不要新造同义类别（历史上有 ai/AI、go/golang 并存的碎片化）。数据来自懒维护的本地记录 `blog.json`（首次自动扫描引导、之后增量对账、publish 成功即更新）
2. **写正文**：遵循 `tech-blog` skill 的文风规范
3. **创建**：`wiki blog new --title --slug --categories --tags --name (--file|--body|--stdin) [--dry-run]`。`hugo new` 按主题 archetype 生成模板（需 hugo，不在 PATH 时 `wiki config set hugoBin`），CLI 只填 title/slug/categories/tags 四字段并拼正文；slug/文件名重复罕见，撞上会在本步直接报错（apply 时校验，先于 hugo new）
4. **发布**：`wiki blog publish <name>`。**push 失败不重试**：原始 git 错误透传 + 非 0 退出，如实告知用户「文件已本地提交，请在博客仓库手动 git push」

## 五、分工与边界

- `tech-blog` skill：管正文文风；`wiki` CLI：管元数据、查重、发布
- 禁止直接往博客仓库手写文章文件（绕过查重与格式统一）
- 禁止手改 `index.md` / `blog.json`（工具生成的数据文件，用命令维护）

## 六、会话 hook 与跨工具规约注入

**单 hook（推荐）：只注册会话退出的 `wiki check`，它覆盖全部维护**——已注册项目维护窗口/迁移正文/同步注册表；未注册项目若已有知识目录则自动反转（迁入知识库 + 原位换成窗口链接，agent 写入后才触发，绝不预建空目录）。`wiki prepare` 降级为可选手动命令：仅对已注册项目建立/修复窗口。

| 时机 | 命令 | 作用 |
|---|---|---|
| 会话退出（/quit，唯一 hook） | `wiki check` | 已注册项目：维护窗口、迁移正文/增量同步、同步注册表；有 wiki-sync 声明块则自动接入；未注册且已有知识目录 → 自动反转（同名注册项冲突时拒绝并提示） |
| 任意时刻（可选，手动） | `wiki prepare` | 已注册项目：建立/修复窗口链接（copy 模式则增量同步），不创建新接入 |

两者 stdout 均恒空（hook 对 stdout 做严格 JSON 校验），日志走 stderr，失败不阻塞会话。

**hook 生效范围**（`hookMode` + 名单，prepare/check 共用）：

| 配置 | 值 | 语义 |
|---|---|---|
| `hookMode` | `forbiddenList`（缺省） | 除 `forbiddenPaths` 禁止名单外全部生效；命中名单（条目自身或其任意深度子/孙目录）跳过 |
| | `whitelist` | 仅 `includePaths` 白名单条目之下的子孙目录生效，其余全跳过 |
| `forbiddenPaths` | 逗号分隔路径 | 仅 forbiddenList 模式读取；条目支持 `~` 前缀，相对路径按 wiki 根解析 |
| `includePaths` | 逗号分隔路径 | 仅 whitelist 模式读取；解析规则同上；为空时全不生效 |

跳过时向 stderr 输出一行原因（如 `命中 forbiddenPaths`），不影响退出码。环境变量 `WIKI_HOOK_MODE` / `WIKI_FORBIDDEN_PATHS` / `WIKI_INCLUDE_PATHS` 优先级高于 config.json。

**ZCode**（`~/.zcode/cli/config.json`）：

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

**Claude Code**（`~/.claude/settings.json`）：

```json
{
  "hooks": {
    "SessionEnd": [
      { "hooks": [ { "type": "command", "command": "/path/to/my-wiki/wiki check" } ] }
    ]
  }
}
```

**Qwen Code / 其他无 hook 的工具**：`wiki inject` 注入规约，agent 收工时自行跑 `wiki check`（唯一必需的触发点）。

- **wiki 根解析**：`WIKI_ROOT` 环境变量 > exe 目录（含 `index.md` 标记）> 当前目录 > exe 目录兜底。hook 内联示例：`cmd /c "set WIKI_ROOT=D:\vault&& wiki.exe check"`。
- **跨工具注入**：`wiki inject [--file <指令文件>] [--remove]` 把规约引导段注入用户级指令文件。目标优先级：`--file` > `config.json` 的 `injectFile` / `WIKI_INJECT_FILE`（各 agent 工具的用户级指令文件路径不同，如 `~/.qwen/QWEN.md`、`~/.claude/CLAUDE.md`，用 `wiki config set injectFile <路径>` 配置）> 缺省 `~/.qwen/QWEN.md`。
