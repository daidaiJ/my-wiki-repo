# my-wiki 规约（Agent 必读）

本仓库是跨项目知识库的统一入口 + Hugo 博客发布工具，CLI 二进制为 `wiki`（本仓库 `go build` 产物）。
符号链接目录 `projects/` 不入版本库（机器本地），注册表数据源藏在 `index.md` 顶部的 `wiki-registry` 注释块里，**index.md 由工具生成，不要手改**。

## 一、wiki-sync 协议（项目接入规约）

任何项目要把自己的 wiki/issue/调研类文档纳入统一管理，在其 `AGENTS.md` 中加一段机器可读的 HTML 注释：

```markdown
<!-- wiki-sync
{
  "paths": ["wiki", "docs/research"],
  "intro": "一句话项目介绍，写进 index.md"
}
-->
```

- `paths`：相对项目根的目录，多个都行，每个目录会在 `my-wiki/projects/<项目名>/` 下建一个符号链接
- `intro`：项目介绍，渲染进 index.md 表格

然后执行一次 `wiki register <项目绝对路径>` 即完成接入。链接是实时视图，源文档改了 my-wiki 里立刻可见，**不存在同步动作**。

## 二、知识库命令

```
wiki register [dir]     # 解析 AGENTS.md 的 wiki-sync 块，建链接 + 更新 index.md（dir 缺省为 cwd）
wiki list               # 列出已注册项目
wiki sync [--fix]       # 检查链接健康度；--fix 重建失效链接；目标目录已删除的报 dead
wiki unlink <项目名>     # 移除注册与链接
```

Windows 上优先建符号链接，无权限时自动降级为 junction，行为一致。

## 三、博客发布流水线（Agent 工作流）

目标仓库 `D:\note\daidaiJ.github.io`（站点在 `pandawo/`，文章在 `pandawo/content/post/`），push 到 main 后 GitHub Actions 自动构建部署，**发布 = push 成功**。

写博客的标准流程：

1. **先查已有分类**：`wiki blog list`（或 `--json`），categories/tags 按使用次数排序，**优先复用已有类别**，不要新造同义类别（历史上出现过 ai/AI 并存的碎片化）
2. **写正文**：遵循 `tech-blog` skill 的文风规范（第一人称学习笔记、代码优先、ASCII 图、个人评注用 blockquote）
3. **创建文章**：

```bash
wiki blog new \
  --title "文章标题" \
  --slug english-kebab-case \
  --categories "技术笔记,AI" \
  --tags "tag1,tag2" \
  --name file_name \
  --file body.md          # 或 --body "..." 或 --stdin
```

front matter 的确定性部分全部自动生成（date/lastmod/draft/toc/hidden/weight/musicid/qqmusic/image），slug 与文件名查重，冲突直接报错。先加 `--dry-run` 预览再正式执行。

4. **提交推送**：`wiki blog publish <文件名>`（或 `blog new --publish` 一步到位）。
   **push 失败不重试**：工具会把 git 原始错误透传出来并以非 0 退出。此时应如实告知用户「文件已本地提交，GitHub 网络问题请稍后在 D:\note\daidaiJ.github.io 手动执行 git push」。

## 四、分工边界

- `tech-blog` skill：管正文文风
- `wiki` CLI：管确定性元数据生成、查重、提交推送
- 不要用 Edit/Write 工具直接去博客仓库手写文章文件——绕过查重和格式统一，禁止
