---
name: wiki
description: 跨项目知识库与 Hugo 博客发布工具。接入项目知识目录（wiki init）、全局检索（wiki grep/ls/cat/tree）、发布博客（wiki blog new/publish）。同步由 Stop hook 全自动完成
---

# wiki — 知识库与博客发布 CLI

完整规约见 wiki 根目录的 `AGENTS.md`（即 wiki.exe 所在目录，`wiki config` 可查 wikiRoot），速查：

## 接入项目 / 补充元数据（任意时间可做）

```bash
wiki init <项目路径> --paths wiki          # 首次接入（写 AGENTS.md 声明块并注册；规约只认专用目录 wiki/，平铺知识库可声明 "."，勿接入 docs/ 等官方文档同名目录）
wiki init <项目路径> --intro "一句话介绍" --summary "摘要"   # 事后补充/更新介绍（paths 省略保留）
```

规约：每个接入目录必须有 `README.md` 索引目录内文档（agent 维护）。同步全自动（Stop hook 跑 `wiki check`），改完声明块即会在下轮生效。

## 全局查看（路径规格：项目/链接/相对路径）

```bash
wiki ls                       # 已接入项目
wiki grep <模式>              # 跨项目内容搜索 → 输出可直接喂给 wiki cat
wiki cat <项目/链接/文件>
wiki tree <项目> --depth 2
```

## 发布博客（仓库可用 wiki config set blogRepo 配置）

1. `wiki blog list` — 只列 categories/tags（按次数降序），**优先复用已有类别**
2. `wiki blog new --title ... --slug kebab-case --categories "A,B" --tags "x,y" --name file_name --file body.md [--dry-run]`
3. `wiki blog publish <name>` — 提交推送；push 网络失败不重试，转告用户手动 `git push`（文件已本地提交）

正文文风遵循 `tech-blog` skill。禁止直接往博客仓库手写文章文件；禁止手改 index.md / blog.json。
