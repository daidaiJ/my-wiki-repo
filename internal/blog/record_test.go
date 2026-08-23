package blog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daidaiJ/my-wiki-repo/internal/config"
)

func TestBlogRecordReconcile(t *testing.T) {
	wiki := t.TempDir()
	t.Setenv("WIKI_ROOT", wiki) // recordPath 经 config.WikiRoot
	postDir := newTestPostDir(t, map[string]string{
		"a.md": "---\ntitle: \"A\"\nslug: a\ncategories: [\"笔记\"]\ntags: [\"golang\"]\n---\nx\n",
	})

	// 首次：扫描引导
	rec, err := reconcileRecord(wiki, postDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Posts) != 1 || rec.Posts[0].Title != "A" {
		t.Fatalf("引导结果: %+v", rec.Posts)
	}

	// 增量：新文件只解析新增的；删除的移除
	os.WriteFile(filepath.Join(postDir, "b.md"), []byte("---\ntitle: \"B\"\nslug: b\ncategories: [\"AI\"]\n---\ny\n"), 0o644)
	os.Remove(filepath.Join(postDir, "a.md"))
	rec, err = reconcileRecord(wiki, postDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Posts) != 1 || rec.Posts[0].File != "b.md" || rec.Posts[0].Title != "B" {
		t.Fatalf("对账结果: %+v", rec.Posts)
	}

	// upsert：publish 成功路径
	rec.upsert(BlogRecEntry{File: "b.md", Title: "B2", Slug: "b", Categories: []string{"AI"}})
	if rec.Posts[0].Title != "B2" || len(rec.Posts) != 1 {
		t.Errorf("upsert 结果: %+v", rec.Posts)
	}

	// 聚合
	os.WriteFile(filepath.Join(postDir, "c.md"), []byte("---\ntitle: \"C\"\nslug: c\ncategories: [\"AI\"]\n---\nz\n"), 0o644)
	rec, _ = reconcileRecord(wiki, postDir)
	cats := aggregate(rec, func(e BlogRecEntry) []string { return e.Categories })
	if cats["AI"] != 2 {
		t.Errorf("categories 聚合: %+v", cats)
	}
}

// TestParseRealPosts 在真实博客目录存在时，用线上文章验证解析与记录引导。
func TestParseRealPosts(t *testing.T) {
	dir := config.BlogPostsDir()
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Skipf("真实博客目录不存在（未配置 blogRepo），跳过: %v", err)
	}
	wiki := t.TempDir() // 记录写到临时根，不污染真实 wiki 根
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
