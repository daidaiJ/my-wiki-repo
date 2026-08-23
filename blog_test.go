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

// TestParseRealPosts 在真实博客目录存在时，用线上文章验证解析与记录引导。
func TestParseRealPosts(t *testing.T) {
	dir := blogPostsDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("真实博客目录不存在，跳过: %v", err)
	}
	wiki := newTestWiki(t) // 记录写到临时根，不污染真实 wiki 根
	t.Setenv("WIKI_ROOT", wiki)

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

	rec, err := reconcileRecord(wiki, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Posts) != len(posts) {
		t.Errorf("记录引导数 %d != 实际 %d", len(rec.Posts), len(posts))
	}
	cats := aggregate(rec, func(e BlogRecEntry) []string { return e.Categories })
	if cats["笔记"] < 5 {
		t.Errorf("categories 聚合异常: %+v", cats)
	}
	// 二次对账应幂等
	rec2, _ := reconcileRecord(wiki, dir)
	if len(rec2.Posts) != len(rec.Posts) {
		t.Errorf("二次对账不幂等: %d vs %d", len(rec2.Posts), len(rec.Posts))
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
