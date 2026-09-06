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

// archetypeTemplate 模拟主题 archetype 经 `hugo new` 生成的模板：
// musicid/image/date 等主题自有字段由主题决定，CLI 只填四个字段。
const archetypeTemplate = `---
title: "New Post"
slug: ""
description: ""
date: 2026-08-23T10:30:00+08:00
lastmod: 2026-08-23T10:30:00+08:00
draft: false
toc: true
hidden: false
weight: false
musicid: 5264842
qqmusic:
categories: [""]
tags: [""]
image: https://picsum.photos/seed/abc12345/800/600
---
`

func TestFillFrontMatter(t *testing.T) {
	got := fillFrontMatter(archetypeTemplate, NewPostMeta{
		Title:      "Google ax + substrate：智能体运行时调度架构分析",
		Slug:       "google-ax-agent-runtime",
		Categories: []string{"技术笔记", "AI"},
		Tags:       []string{"智能体", "kubernetes"},
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
tags:
    - 智能体
    - kubernetes
image: https://picsum.photos/seed/abc12345/800/600
---
`
	if got != want {
		t.Errorf("fillFrontMatter 结果不一致:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFillFrontMatterBlockStyle(t *testing.T) {
	// 块式列表 archetype：整块替换，不残留旧列表项
	tmpl := "---\ntitle: \"T\"\nslug: \"\"\ncategories:\n    - 旧分类\n    - 旧分类2\ntags:\n    - old\nimage: x\n---\n"
	got := fillFrontMatter(tmpl, NewPostMeta{Title: "新", Slug: "new", Categories: []string{"笔记"}, Tags: []string{"go"}})
	want := "---\ntitle: \"新\"\nslug: new\ncategories:\n    - 笔记\ntags:\n    - go\nimage: x\n---\n"
	if got != want {
		t.Errorf("块式列表替换不一致:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFillFrontMatterMissingKeys(t *testing.T) {
	// archetype 没生成的键：补在结束 --- 之前
	tmpl := "---\ntitle: \"T\"\ndate: 2026-01-01T00:00:00+08:00\n---\n"
	got := fillFrontMatter(tmpl, NewPostMeta{Title: "新", Slug: "new", Categories: []string{"笔记"}, Tags: []string{"go"}})
	want := "---\ntitle: \"新\"\ndate: 2026-01-01T00:00:00+08:00\nslug: new\ncategories:\n    - 笔记\ntags:\n    - go\n---\n"
	if got != want {
		t.Errorf("缺键补齐不一致:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFillFrontMatterNoFence(t *testing.T) {
	// 无 front matter 的极端 archetype：原样返回，调用方直接拼正文
	got := fillFrontMatter("hello\n", NewPostMeta{Title: "新", Slug: "new", Categories: []string{"笔记"}})
	if got != "hello\n" {
		t.Errorf("无围栏 archetype 应原样返回, got %q", got)
	}
}

func TestFillFrontMatterCRLF(t *testing.T) {
	tmpl := "---\r\ntitle: \"T\"\r\nslug: \"\"\r\ncategories: [\"\"]\r\ntags: [\"\"]\r\n---\r\n"
	got := fillFrontMatter(tmpl, NewPostMeta{Title: "新", Slug: "new", Categories: []string{"笔记"}, Tags: []string{"go"}})
	if strings.Contains(got, "\r") {
		t.Errorf("CRLF 模板应规整为 LF: %q", got)
	}
	if !strings.Contains(got, "slug: new") || !strings.Contains(got, "    - 笔记") {
		t.Errorf("CRLF 模板填充异常:\n%s", got)
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
	full := fillFrontMatter(archetypeTemplate, meta)
	if idx := closingFenceIndex(strings.Split(full, "\n")); idx >= 0 {
		full = strings.TrimRight(full, "\n") + "\n" + body
	} else {
		full = full + "\n" + body
	}
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
	meta := NewPostMeta{Title: "新文章", Slug: "new-post", Categories: []string{"笔记"}, Tags: []string{"k8s"}}
	target, err := createPost(t, dir, meta, "# 新正文\n", "new_post")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	s := string(data)
	if !strings.Contains(s, "slug: new-post") || !strings.Contains(s, "- k8s") || !strings.HasSuffix(s, "# 新正文\n") {
		t.Errorf("生成文件内容异常:\n%s", s)
	}
	if !strings.Contains(s, "musicid: 5264842") {
		t.Errorf("主题字段应保留:\n%s", s)
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

// --- cmdBlogNew 全流程（mock runHugoNew，不依赖真实 hugo） ---

// mockHugoNew 替换 runHugoNew：记录调用参数，并在站点目录生成 archetype 模板文件。
func mockHugoNew(t *testing.T, calls *[]string) {
	t.Helper()
	orig := runHugoNew
	runHugoNew = func(hugoBin, siteDir, rel string) error {
		*calls = append(*calls, hugoBin+"|"+siteDir+"|"+rel)
		target := filepath.Join(siteDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, []byte(archetypeTemplate), 0o644)
	}
	t.Cleanup(func() { runHugoNew = orig })
}

// newBlogEnv 搭好 blog new 所需环境：WIKI_ROOT + config.json（blogRepo 指向临时仓库）。
func newBlogEnv(t *testing.T) (wiki, repo, postDir string) {
	t.Helper()
	wiki = t.TempDir()
	t.Setenv("WIKI_ROOT", wiki)
	repo = t.TempDir()
	postDir = filepath.Join(repo, "content", "post")
	if err := os.MkdirAll(postDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `{"blogRepo": "` + strings.ReplaceAll(repo, `\`, `\\`) + `"}`
	if err := os.WriteFile(filepath.Join(wiki, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return wiki, repo, postDir
}

func TestCmdBlogNewFullFlow(t *testing.T) {
	_, repo, postDir := newBlogEnv(t)
	var calls []string
	mockHugoNew(t, &calls)

	err := cmdBlogNew([]string{
		"--title", "新文章", "--slug", "new-post",
		"--categories", "技术笔记,AI", "--tags", "go,k8s",
		"--name", "new_post", "--body", "# 正文\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	// hugo new 调用参数正确
	if len(calls) != 1 {
		t.Fatalf("runHugoNew 应恰好调用 1 次: %v", calls)
	}
	parts := strings.Split(calls[0], "|")
	if parts[0] != "hugo" || parts[1] != repo || parts[2] != "content/post/new_post.md" {
		t.Errorf("hugo new 参数异常: %s", calls[0])
	}
	// 落盘内容：四字段已填、主题字段保留、正文在末尾
	data, err := os.ReadFile(filepath.Join(postDir, "new_post.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`title: "新文章"`,
		"slug: new-post",
		"    - 技术笔记",
		"    - go",
		"musicid: 5264842",
		"image: https://picsum.photos/seed/abc12345/800/600",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("生成文件缺 %q:\n%s", want, s)
		}
	}
	if !strings.HasSuffix(s, "# 正文\n") {
		t.Errorf("正文应拼在末尾:\n%s", s)
	}
}

func TestCmdBlogNewDryRun(t *testing.T) {
	_, _, postDir := newBlogEnv(t)
	var calls []string
	mockHugoNew(t, &calls)

	err := cmdBlogNew([]string{
		"--title", "预览", "--slug", "preview-post",
		"--categories", "笔记", "--name", "preview_post", "--body", "x",
		"--dry-run",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("dry-run 也应走 hugo new: %v", calls)
	}
	if _, err := os.Stat(filepath.Join(postDir, "preview_post.md")); !os.IsNotExist(err) {
		t.Errorf("dry-run 后文件应删除恢复: %v", err)
	}
}

func TestCmdBlogNewConflictBeforeHugo(t *testing.T) {
	_, _, postDir := newBlogEnv(t)
	os.WriteFile(filepath.Join(postDir, "existing.md"),
		[]byte("---\ntitle: \"已有\"\nslug: existing-post\ncategories: [\"笔记\"]\n---\nx\n"), 0o644)
	var calls []string
	mockHugoNew(t, &calls)

	// slug 冲突：先于 hugo new 报错
	err := cmdBlogNew([]string{"--title", "x", "--slug", "existing-post", "--categories", "笔记", "--name", "a", "--body", "x"})
	if err == nil || !strings.Contains(err.Error(), "已被") {
		t.Errorf("slug 冲突应报错: %v", err)
	}
	// 文件名冲突：同样先于 hugo new
	err = cmdBlogNew([]string{"--title", "x", "--slug", "fresh-post", "--categories", "笔记", "--name", "existing", "--body", "x"})
	if err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Errorf("文件名冲突应报错: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("冲突时不应调用 hugo new: %v", calls)
	}
}

func TestCmdBlogNewHugoFailure(t *testing.T) {
	newBlogEnv(t)
	orig := runHugoNew
	runHugoNew = func(hugoBin, siteDir, rel string) error {
		return &testErr{"hugo new 失败（mock: hugo 不在 PATH）"}
	}
	t.Cleanup(func() { runHugoNew = orig })

	err := cmdBlogNew([]string{"--title", "x", "--slug", "fail-post", "--categories", "笔记", "--name", "fail_post", "--body", "x"})
	if err == nil || !strings.Contains(err.Error(), "hugo new 失败") {
		t.Errorf("hugo new 失败应透传: %v", err)
	}
}

func TestCmdBlogNewNoFenceArchetype(t *testing.T) {
	_, _, postDir := newBlogEnv(t)
	orig := runHugoNew
	runHugoNew = func(hugoBin, siteDir, rel string) error {
		target := filepath.Join(siteDir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(target), 0o755)
		return os.WriteFile(target, []byte("hello\n"), 0o644)
	}
	t.Cleanup(func() { runHugoNew = orig })

	if err := cmdBlogNew([]string{"--title", "x", "--slug", "plain-post", "--categories", "笔记", "--name", "plain_post", "--body", "# 正文\n"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(postDir, "plain_post.md"))
	if string(data) != "hello\n\n# 正文\n" {
		t.Errorf("无围栏 archetype 应直接拼正文: %q", string(data))
	}
}

func TestCmdBlogNewNameRequired(t *testing.T) {
	newBlogEnv(t)
	err := cmdBlogNew([]string{"--title", "x", "--slug", "x-post", "--categories", "笔记", "--body", "x"})
	if err == nil || !strings.Contains(err.Error(), "--name 必填") {
		t.Errorf("缺 --name 应报错: %v", err)
	}
}
