# my-wiki 规约（Agent 必读）

本仓库是跨项目知识库的统一入口 + Hugo 博客发布工具，CLI 二进制为 `wiki`（本仓库 `go build` 产物）。
`projects/` 下是各项目知识目录的符号链接/junction（机器本地，不入库）；`index.md` 是工具生成的项目索引（数据源在顶部隐藏 JSON 块，**不要手改**）。

## 三条相互独立的流程（互不阻塞）

| 流程 | 触发方 | 机制 |
|---|---|---|
| ① 同步 | **全自动**（Stop hook） | 每轮回复结束执行 `wiki check`：有声明块则幂等维护链接、把 AGENTS.md 里最新元数据同步进注册表 |
| ② 元数据 | agent，**任意时间** | 改项目 AGENTS.md 的 wiki-sync 块（或跑 `wiki init --intro/--summary`），下轮 hook 自动生效 |
| ③ 发布 | agent，主动 | `wiki blog new` → `wiki blog publish` |

## 一、接入声明（集中式，项目仓库零足迹）

接入信息（paths/intro/summary）**只存在 wiki 根的本地注册表 `index.md`**（本地 git，无远程，永不外泄）。`wiki init` 直接写注册表并建链接，**不在项目仓库创建/修改任何文件**——项目自行 commit/push 不会把个人知识库配置带上远程。

- 知识目录类型名默认 `wiki/` 与 `issues/`，可在 `config.json` 的 `knowledgeDirs` 配置（fork 者自定义规约的入口）；`wiki init` 省略 `--paths` 时按类型名自动发现
- **上游同名目录冲突**：clone 的上游项目若自带 `wiki/` 或 `issues/`，看情况处理——无价值直接干掉让出名字；有价值则 agent 整理提炼合并进该项目个人知识库再删源目录，专用名必须归个人知识使用
- `docs/` 等与上游官方文档同名的通用目录默认不接入，确需时由用户显式 `--paths` 指定（工具会告警提醒确认）
- 可选 opt-in：项目 `AGENTS.md` 里的 `<!-- wiki-sync {...} -->` 声明块仍被 `wiki check` 识别（适合团队共享声明的仓库），但工具不再代写——避免污染项目仓库

## 二、知识库命令

```
wiki init [目录] --paths <目录列表> [--intro ...] [--summary ...]   # agent 接入/更新入口
wiki register [目录]        # 等价于声明块已存在时的 init（一般直接用 init）
wiki list                   # 已注册项目 + 健康度 + README 缺失告警
wiki sync [--fix]           # 链接健康检查/修复
wiki unlink <项目名>         # 移除注册与链接
wiki check                  # Stop hook 入口（幂等同步；无声明块则静默，一般不手动跑）
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

目标仓库默认 `D:\note\daidaiJ.github.io`（文章在 `pandawo/content/post/`），**可用 `wiki config set blogRepo <路径>` 配置**；push 到 main 后 GitHub Actions 自动部署，发布 = push 成功。

1. **先查已有分类**：`wiki blog list`（只列 categories/tags 两字段，按使用次数降序）——**优先复用高频类别**，不要新造同义类别（历史上有 ai/AI、go/golang 并存的碎片化）。数据来自懒维护的本地记录 `blog.json`（首次自动扫描引导、之后增量对账、publish 成功即更新）
2. **写正文**：遵循 `tech-blog` skill 的文风规范
3. **创建**：`wiki blog new --title --slug --categories --tags --name (--file|--body|--stdin) [--dry-run]`。`hugo new` 按主题 archetype 生成模板（需 hugo，不在 PATH 时 `wiki config set hugoBin`），CLI 只填 title/slug/categories/tags 四字段并拼正文；slug/文件名重复罕见，撞上会在本步直接报错（apply 时校验，先于 hugo new）
4. **发布**：`wiki blog publish <name>`。**push 失败不重试**：原始 git 错误透传 + 非 0 退出，如实告知用户「文件已本地提交，请在 D:\note\daidaiJ.github.io 手动 git push」

## 五、分工与边界

- `tech-blog` skill：管正文文风；`wiki` CLI：管元数据、查重、发布
- 禁止直接往博客仓库手写文章文件（绕过查重与格式统一）
- 禁止手改 `index.md` / `blog.json`（工具生成的数据文件，用命令维护）

## 六、Stop hook 与跨工具规约注入

- **Stop hook**：用户级 `~/.zcode/cli/config.json` 的 `hooks.events.Stop` 注册了 `wiki check`（process 类型直调 wiki.exe）。stdout 恒空（hook 对 stdout 做严格 JSON 校验），日志全走 stderr，任何失败都不阻塞会话。没有声明块的项目完全无感；这也是元数据「事后补充」能自动生效的原因。
- **跨工具注入**：`wiki inject [--file <指令文件>] [--remove]` 把 wiki 引导段注入其他 agent 工具的用户级指令文件（默认 `~/.qwen/QWEN.md`）。`wiki-guide` 特殊标记锚定：无标记段则末尾追加、有则原位替换（跨版本安全）、一致则跳过；升级规约内容后改 `internal/guide/guide.go` 重新构建并跑一次 `wiki inject` 即可全量更新。
