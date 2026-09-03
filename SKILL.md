---
name: wiki
description: wiki 知识库全工作流指导：环境配置（一次性）、知识接入与跨项目检索（高频）、可选的 Hugo 博客发布。当用户要沉淀/整理项目调研文档、维护项目介绍摘要、跨项目查资料、或发布技术博客时使用 wiki CLI
---

# wiki 工作流

三条相互独立的流程，按需取用对应章节；完整规约见 wiki 根目录的 `AGENTS.md`（`wiki config` 可查 wikiRoot）。

## 一、环境配置（首次或换机时，一次性）

```bash
# 1. 工具仓库构建（工具与数据分离：工具仓库可开源，个人数据在 WIKI_ROOT 指向的目录）
cd <my-wiki 工具仓库> && go build -o wiki.exe .
# 2. 数据根：用户级环境变量 WIKI_ROOT 指向个人 wiki 数据目录；验证：
wiki config            # 检查 wikiRoot / knowledgeDirs / blogRepo 是否符合预期
# 3. 用博客功能才需要：
wiki config set blogRepo <Hugo 仓库绝对路径>
#    blog new 需要 hugo（按主题 archetype 生成模板）；不在 PATH 时：
wiki config set hugoBin <hugo 可执行文件路径>
# 4. 双 hook：会话开始 wiki prepare（建窗口链接）、退出 wiki check（维护同步）
#    ZCode Start/Stop 或 Claude SessionStart/SessionEnd —— 详见工具仓库 README / AGENTS.md
# 5. 把规约引导注入其他 agent 工具（如 qwen code）：
wiki inject             # 标记锚定：追加/原位替换/幂等
# 6. 健康自检：
wiki list && wiki sync
```

## 二、知识库日常操作（高频）

**接入项目**（首次 `wiki init`，之后 hook 自动维护窗口）：

```bash
wiki init <项目路径> --paths wiki,issues    # 首次接入；可预建空目录
wiki prepare                                 # 开工前（hook 或手动）：已注册项目建/修窗口链接
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

会话退出时自动完成：窗口链接维护、正文迁移、注册表同步——**无需手动做**（开工前 `wiki prepare` 同样自动）。

## 三、发布博客（可选；仅当用户明确要发布时）

发布前先提示用户在 Obsidian 校对（知识库 = Obsidian 仓库根，`projects/` 下是真文件；接入见工具仓库 `docs/obsidian.md`）。

```bash
wiki blog list                          # 只列 categories/tags（按使用次数降序），优先复用已有类别防碎片化
wiki blog new \
  --title "标题" --slug english-kebab-case \
  --categories "技术笔记,AI" --tags "tag1,tag2" \
  --name file_name --file body.md        # --name 必填（hugo new 目标文件）；正文也可 --body "..." 或 --stdin；先 --dry-run 预览
wiki blog publish file_name              # 提交推送，GitHub Actions 自动部署
```

- 正文文风遵循 `tech-blog` skill；`hugo new` 按主题 archetype 生成模板（需 hugo），CLI 只填 title/slug/categories/tags 四字段并拼正文；slug/文件名冲突在此步报错（先于 hugo new）
- **push 网络失败不重试**：如实告知用户「文件已本地提交，请在博客仓库手动 git push」
- 发布成功后四字段记录自动入 `blog.json`（懒维护）
- 禁止绕过 CLI 直接往博客仓库手写文章文件
