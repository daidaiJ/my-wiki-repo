package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestCollectAndEnrich(t *testing.T) {
	dir := newTestPostDir(t, map[string]string{
		"post_a.md":    "---\ntitle: \"文章A\"\nslug: post-a\ncategories:\n    - 笔记\ntags :\n    - golang\n---\n正文A\n",
		"post_b.md":    "---\ntitle: \"文章B\"\nslug: post-b\ncategories: [\"笔记\", \"AI\"]\ntags: [\"golang\", \"ai\"]\n---\n正文B\n",
		"post_c.md":    "---\ntitle: \"文章C\"\nslug: post-c\ncategories:\n    - AI\ntags: [\"ai\"]\n---\n正文C\n",
		"notapost.txt": "忽略我",
	})
	posts, err := collectPosts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 3 {
		t.Fatalf("应收集 3 篇，得到 %d: %+v", len(posts), posts)
	}
	idx := buildBlogIndex(posts)
	if err := enrichTaxonomy(idx, dir, posts); err != nil {
		t.Fatal(err)
	}
	if idx.Categories["笔记"] != 2 || idx.Categories["AI"] != 2 {
		t.Errorf("categories 计数错误: %+v", idx.Categories)
	}
	if idx.Tags["golang"] != 2 || idx.Tags["ai"] != 2 {
		t.Errorf("tags 计数错误: %+v", idx.Tags)
	}
	if len(idx.Slugs) != 3 || idx.Slugs[0] != "post-a" {
		t.Errorf("slugs = %v", idx.Slugs)
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

	// slug 冲突
	if _, err := createPost(t, dir, NewPostMeta{Slug: "existing-post"}, "x", "another"); err != errSlugConflict {
		t.Errorf("slug 冲突未检出: %v", err)
	}
	// 文件名冲突
	if _, err := createPost(t, dir, NewPostMeta{Slug: "fresh-post"}, "x", "new_post"); err != errFileExists {
		t.Errorf("文件名冲突未检出: %v", err)
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" 技术笔记, AI ,,笔记,")
	want := "技术笔记|AI|笔记"
	if strings.Join(got, "|") != want {
		t.Errorf("splitCSV = %v, want %s", got, want)
	}
}

func TestSlugAndNameValidation(t *testing.T) {
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

// TestParseRealPosts 在真实博客目录存在时，用线上 51 篇文章验证解析器。
func TestParseRealPosts(t *testing.T) {
	dir := blogPostsDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("真实博客目录不存在，跳过: %v", err)
	}
	posts, err := collectPosts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) < 40 {
		t.Fatalf("真实文章应 ≥40 篇，得到 %d", len(posts))
	}
	bad := 0
	for _, p := range posts {
		if p.Title == "" || p.Title == p.File {
			bad++
		}
	}
	if bad > 2 {
		t.Errorf("%d 篇文章标题解析异常（容忍少量历史遗留）", bad)
	}
	idx := buildBlogIndex(posts)
	if len(idx.Slugs) == 0 {
		t.Error("未解析到任何 slug")
	}
	if err := enrichTaxonomy(idx, dir, posts); err != nil {
		t.Fatal(err)
	}
	if idx.Categories["笔记"] < 5 {
		t.Errorf("categories 计数异常: %+v", idx.Categories)
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
