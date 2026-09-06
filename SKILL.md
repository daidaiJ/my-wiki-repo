---
name: wiki
description: wiki 知识库全工作流指导：环境配置（一次性）、知识接入与跨项目检索（高频）、可选的 Hugo 博客发布。当用户要沉淀/整理项目调研文档、维护项目介绍摘要、跨项目查资料、或发布技术博客时使用 wiki CLI
---

# wiki 工作流

三条相互独立的流程，按需取用对应章节；完整规约见 wiki 根目录的 `AGENTS.md`（`wiki config` 可查 wikiRoot）。

## 一、环境配置（首次或换机时，一次性）

```bash
# 1. 工具仓库构建（工具与数据分离：工具仓库可开源，个人数据在 WIKI_ROOT 指向的目录）
cd <my-wiki 工具仓库> && go build -o wiki.exe ./cmd/wiki
# 2. 数据根：用户级环境变量 WIKI_ROOT 指向个人 wiki 数据目录；验证：
wiki config            # 检查 wikiRoot / knowledgeDirs / blogRepo 是否符合预期
# 3. 用博客功能才需要：
wiki config set blogRepo <Hugo 仓库绝对路径>
#    blog new 需要 hugo（按主题 archetype 生成模板）；不在 PATH 时：
wiki config set hugoBin <hugo 可执行文件路径>
# 4. 单 hook：会话退出 wiki check（维护窗口；未注册项目已有知识目录时自动反转）；wiki prepare 仅手动修复用
#    ZCode Stop 或 Claude SessionEnd —— 详见工具仓库 README / AGENTS.md
# 5. 把规约引导注入其他 agent 工具（如 qwen code）：
wiki inject             # 标记锚定：追加/原位替换/幂等
# 6. 健康自检：
wiki list && wiki sync
```

## 二、知识库日常操作（高频）

**接入项目**（会话退出 hook 对已有知识目录自动反转并补注册表；`wiki init` 用于显式声明 paths/模式与补 intro/summary 元数据；生效范围见 hookMode/forbiddenPaths/includePaths）：

```bash
wiki init <项目路径> --paths wiki,issues    # 首次接入；可预建空目录
wiki prepare                                 # 可选手动：修复已注册项目的窗口链接（不创建新接入）
```

要点：声明只存本地注册表；知识正文在 `projects/`（可 git），项目侧 `wiki/` 等为窗口链接；`projectGitignore` 默认写入项目 `.gitignore`；每个接入目录须有 `README.md` 索引。

**跨项目检索**（路径规格：`项目/链接/相对路径`）：

```bash
wiki ls                   # 已接入项目
wiki grep <模式>          # 全局内容搜索，输出可直接喂给 cat
wiki cat <项目/链接/文件>
wiki tree <项目> --depth 2
```

**维护**：`wiki list`（健康度）/ `wiki sync [--fix]`（修链接）/ `wiki unlink <项目>`。

会话退出时自动完成：窗口链接维护、正文迁移、未注册项目的按需自动反转、注册表同步——**无需手动做**。

## 三、发布博客（可选；仅当用户明确要发布时）

**硬规则（agent 视角，无需用户提醒）：**

1. **创建文章必须走 `wiki blog new`，发布必须走 `wiki blog publish`**。Hugo 文章的 YAML front matter 由主题 archetype 生成（含 `musicid`/`image` 等主题必需字段的默认值），agent 手写的头大概率不合规——**禁止用 Write/Edit 直接在博客仓库 `content/post/` 下新建文章文件**。如果你发现自己在往博客仓库手写文章文件，停下来，走错路了。
2. **更新已有文章**：只改正文和 `lastmod` 的值，不动 front matter 的字段结构（模板生成的）；改完同样用 `wiki blog publish <file_name>` 推送。
3. 正文先起草到博客仓库**之外**的临时文件（如系统临时目录），再喂给 `--file`；避免草稿污染博客仓库 git 状态。

**标准流程（新文章）：**

```bash
# ① 看已有分类/tags，优先复用（防碎片化）；系列文章统一 slug 前缀 + 标题带序号
wiki blog list

# ② 起草正文（body only，无 front matter；开头格式 "# 标题\n------\n> 引言"；
#    文风遵循 tech-blog skill；代码片段必须溯源，禁止编造）

# ③ 先 --dry-run 预览，确认后去掉再正式创建
#    --name 不带 .md 扩展名（带扩展名/目录符会报"非法字符"，允许字母数字 _ -）
wiki blog new --title "标题" --slug english-kebab-case \
  --categories "技术笔记,AI" --tags "tag1,tag2" \
  --name file_name --file body.md

# ④ 发布：文件名不带 .md 扩展名（带扩展名会报"文件名非法"）
wiki blog publish file_name                 # git add + commit + push，GitHub Actions 自动部署
```

**其余要点：**

- 发布前提示用户可在 Obsidian 校对（知识库 = Obsidian 仓库根，`projects/` 下是真文件；接入见工具仓库 `docs/obsidian.md`）；用户明确要求提交推送时可直接发布
- 正文文风遵循 `tech-blog` skill；`hugo new` 按主题 archetype 生成模板（需 hugo），CLI 只填 title/slug/categories/tags 四字段并拼正文；slug/文件名冲突在此步报错（先于 hugo new）
- **push 网络失败不重试**：如实告知用户「文件已本地提交，请在博客仓库手动 git push」
- 发布成功后四字段记录自动入 `blog.json`（懒维护）
- 多篇文章按系列产出时逐篇走完整流程（草稿 → new → publish），不要攒批手写
