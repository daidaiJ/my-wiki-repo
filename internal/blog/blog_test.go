package blog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedTime() time.Time {
	return time.Date(2026, 8, 23, 10, 30, 0, 0, time.FixedZone("CST", 8*3600))
}

func newTestPostDir(t *testing.T, seedPosts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range seedPosts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestGenerateFrontMatterGolden(t *testing.T) {
	got := generateFrontMatter(NewPostMeta{
		Title:      "Google ax + substrate：智能体运行时调度架构分析",
		Slug:       "google-ax-agent-runtime",
		Categories: []string{"技术笔记", "AI"},
		Tags:       []string{"智能体", "kubernetes"},
		Now:        fixedTime(),
	})
	want := `---
title: "Google ax + substrate：智能体运行时调度架构分析"
slug: google-ax-agent-runtime
description: ""
date: 2026-08-23T10:30:00+08:00
lastmod: 2026-08-23T10:30:00+08:00
draft: false
toc: true
hidden: false
weight: false
musicid: 5264842
qqmusic: 
categories:
    - 技术笔记
    - AI
tags :
    - 智能体
    - kubernetes
image: https://picsum.photos/seed/3b36cb88/800/600
---`
	if got != want {
		t.Errorf("front matter 与预期不一致:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// axPostFM 是线上真实文章的 front matter（含 `tags :` 冒号前空格的既有写法）。
const axPostFM = `---
title: "Google ax + substrate：智能体运行时调度架构分析"
slug: google-ax-agent-runtime
description: ""
date: 2026-06-07T09:53:32+08:00
lastmod: 2026-06-07T09:53:32+08:00
draft: false
toc: true
hidden: false
weight: false
musicid: 5264842
qqmusic: 
categories:
    - 技术笔记
    - AI
tags :
    - 智能体
    - kubernetes
image: https://picsum.photos/seed/40aa71ea/800/600
---

# 正文标题

正文内容。
`

func TestParseFrontMatterRealPost(t *testing.T) {
	fm, err := parseFrontMatter(axPostFM)
	if err != nil {
		t.Fatal(err)
	}
	if fm.Title != "Google ax + substrate：智能体运行时调度架构分析" {
		t.Errorf("title = %q", fm.Title)
	}
	if fm.Slug != "google-ax-agent-runtime" {
		t.Errorf("slug = %q", fm.Slug)
	}
	if strings.Join(fm.Categories, ",") != "技术笔记,AI" {
		t.Errorf("categories = %v", fm.Categories)
	}
	if strings.Join(fm.Tags, ",") != "智能体,kubernetes" {
		t.Errorf("tags = %v", fm.Tags)
	}
}

func TestParseFrontMatterFlowStyle(t *testing.T) {
	content := "---\ntitle: \"x\"\ncategories: [\"a\", \"b\"]\ntags: [\"golang\"]\nslug: x\n---\nbody"
	fm, err := parseFrontMatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(fm.Categories, ",") != "a,b" {
		t.Errorf("categories = %v", fm.Categories)
	}
	if strings.Join(fm.Tags, ",") != "golang" {
		t.Errorf("tags = %v", fm.Tags)
	}
}

func TestParseFrontMatterErrors(t *testing.T) {
	if _, err := parseFrontMatter("no front matter"); err == nil {
		t.Error("缺开头 --- 应报错")
	}
	if _, err := parseFrontMatter("---\ntitle: \"x\"\n"); err == nil {
		t.Error("缺结束 --- 应报错")
	}
}

func TestYamlQuote(t *testing.T) {
	cases := map[string]string{
		"普通中文":      `"普通中文"`,
		`含"引号"`:     `"含\"引号\""`,
		`含\反斜杠`:     `"含\\反斜杠"`,
		`Agent 运行时`: `"Agent 运行时"`,
	}
	for in, want := range cases {
		if got := yamlQuote(in); got != want {
			t.Errorf("yamlQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestCollectPostsToleratesBadFM(t *testing.T) {
	dir := newTestPostDir(t, map[string]string{
		"good.md":    "---\ntitle: \"A\"\nslug: a\ncategories:\n    - 笔记\ntags :\n    - golang\n---\n正文A\n",
		"bad.md":     "没有 front matter",
		"notapost":   "无扩展名忽略",
		"notpost.md": "---\ntitle: \"B\"\nslug: b\ncategories: [\"笔记\"]\ntags: [\"golang\"]\n---\n正文B\n",
	})
	posts, err := collectPosts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 3 {
		t.Fatalf("应收集 3 篇（含坏 FM 的 bad.md 降级）: %+v", posts)
	}
	byFile := map[string]PostInfo{}
	for _, p := range posts {
		byFile[p.File] = p
	}
	if byFile["bad.md"].Title != "bad.md" {
		t.Errorf("坏 FM 应回退文件名: %+v", byFile["bad.md"])
	}
	if byFile["notpost.md"].Slug != "b" {
		t.Errorf("notpost.md = %+v", byFile["notpost.md"])
	}
}

// createPost 是 cmdBlogNew 的核心写入逻辑等价路径（命令层 flag 解析在冒烟里验证）。
func createPost(t *testing.T, postDir string, meta NewPostMeta, body, fileName string) (string, error) {
	t.Helper()
	posts, err := collectPosts(postDir)
	if err != nil {
		return "", err
	}
	for _, p := range posts {
		if p.Slug == meta.Slug {
			return "", errSlugConflict
		}
		if p.File == fileName+".md" {
			return "", errFileExists
		}
	}
	full := generateFrontMatter(meta) + "\n" + body
	target := filepath.Join(postDir, fileName+".md")
	return target, os.WriteFile(target, []byte(full), 0o644)
}

var (
	errSlugConflict = &testErr{"slug 冲突"}
	errFileExists   = &testErr{"文件已存在"}
)

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }

func TestCreatePostAndConflict(t *testing.T) {
	dir := newTestPostDir(t, map[string]string{
		"existing.md": "---\ntitle: \"已有\"\nslug: existing-post\ncategories: [\"笔记\"]\ntags: [\"golang\"]\n---\n旧正文\n",
	})
	meta := NewPostMeta{Title: "新文章", Slug: "new-post", Categories: []string{"笔记"}, Tags: []string{"k8s"}, Now: fixedTime()}
	target, err := createPost(t, dir, meta, "# 新正文\n", "new_post")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	s := string(data)
	if !strings.Contains(s, "slug: new-post") || !strings.Contains(s, "- k8s") || !strings.HasSuffix(s, "# 新正文\n") {
		t.Errorf("生成文件内容异常:\n%s", s)
	}

	// slug 冲突（apply 时报错）
	if _, err := createPost(t, dir, NewPostMeta{Slug: "existing-post"}, "x", "another"); err != errSlugConflict {
		t.Errorf("slug 冲突未检出: %v", err)
	}
	// 文件名冲突
	if _, err := createPost(t, dir, NewPostMeta{Slug: "fresh-post"}, "x", "new_post"); err != errFileExists {
		t.Errorf("文件名冲突未检出: %v", err)
	}
}

func TestSlugValidation(t *testing.T) {
	for _, ok := range []string{"google-ax-agent-runtime", "vllm-deploy", "a1"} {
		if !slugRe.MatchString(ok) {
			t.Errorf("%s 应合法", ok)
		}
	}
	for _, bad := range []string{"Google-AX", "有中文", "-leading", "trailing-", "double--x", ""} {
		if slugRe.MatchString(bad) {
			t.Errorf("%s 应非法", bad)
		}
	}
}

func TestNowFnInjectable(t *testing.T) {
	orig := nowFn
	nowFn = func() time.Time { return fixedTime() }
	defer func() { nowFn = orig }()
	if !nowFn().Equal(fixedTime()) {
		t.Error("nowFn 注入失败")
	}
}
