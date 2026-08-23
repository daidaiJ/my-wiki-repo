---
name: wiki
description: 跨项目知识库与 Hugo 博客发布工具。接入项目 wiki（wiki-sync 协议）、查已有博客分类、创建并发布博客文章。使用 wiki register / list / sync / blog new / blog list / blog publish
---

# wiki — 知识库与博客发布 CLI

完整规约见 `D:\CODE\ai\my-wiki\AGENTS.md`，速查：

## 把某项目的文档目录纳入统一知识库

项目 `AGENTS.md` 里加（paths 为相对项目根的目录）：

```markdown
<!-- wiki-sync
{
  "paths": ["wiki"],
  "intro": "一句话项目介绍"
}
-->
```

然后 `wiki register <项目绝对路径>`。链接实时生效，无需任何同步。

## 发布博客（Hugo，仓库 D:\note\daidaiJ.github.io）

1. `wiki blog list` — 查已有 categories/tags/slug，**优先复用已有类别**
2. `wiki blog new --title ... --slug kebab-case --categories "A,B" --tags "x,y" --name file_name --file body.md [--dry-run]` — 正文写好传入，front matter 自动生成
3. `wiki blog publish <name>` — 提交推送；push 网络失败不重试，转告用户手动 `git push`（文件已本地提交）

正文文风遵循 `tech-blog` skill。禁止直接往博客仓库手写文章文件（绕过查重与格式统一）。
