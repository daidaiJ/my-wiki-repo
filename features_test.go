package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// --- wiki init ---

func TestUpsertWikiSyncBlock(t *testing.T) {
	dir := t.TempDir()
	decl := &wikiSyncDecl{Paths: []string{"wiki"}, Intro: "介绍一"}

	// 文件不存在 → 创建
	if err := upsertWikiSyncBlock(dir, decl); err != nil {
		t.Fatal(err)
	}
	got, err := parseWikiSync(dir)
	if err != nil {
		t.Fatalf("创建后应可解析: %v", err)
	}
	if got.Intro != "介绍一" || len(got.Paths) != 1 {
		t.Errorf("decl = %+v", got)
	}

	// 有其他内容 → 原位替换，其余内容不动
	agents := filepath.Join(dir, "AGENTS.md")
	data, _ := os.ReadFile(agents)
	withCtx := append([]byte("# 项目说明\n\n一些既有内容。\n\n"), data...)
	os.WriteFile(agents, withCtx, 0o644)
	decl.Intro = "介绍二"
	decl.Summary = "这是摘要"
	if err := upsertWikiSyncBlock(dir, decl); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(agents)
	if !strings.Contains(string(data2), "一些既有内容") {
		t.Error("替换块时不应动其他内容")
	}
	got, err = parseWikiSync(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Intro != "介绍二" || got.Summary != "这是摘要" {
		t.Errorf("更新后 decl = %+v", got)
	}
	if n := strings.Count(string(data2), "wiki-sync"); n != 1 {
		t.Errorf("应只有一个声明块，实际 %d", n)
	}
}

func TestInitFlowAndMetadataUpdate(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "flowproj", []string{"wiki"})
	// newTestProject 自带声明块，先清掉介绍模拟首次接入
	os.WriteFile(filepath.Join(proj, "AGENTS.md"), []byte("# AGENTS\n"), 0o644)

	// 首次接入必须 --paths
	root := wiki
	if res, err := ensureRegisteredByInit(root, proj, "", "", ""); err == nil {
		t.Fatalf("缺 --paths 应报错, got %+v", res)
	}

	// 接入（intro 留空）
	if _, err := ensureRegisteredByInit(root, proj, "wiki", "", ""); err != nil {
		t.Fatal(err)
	}
	e := findTestEntry(t, root, "flowproj")
	if e.Intro != "" {
		t.Errorf("intro 应为空待补充, got %q", e.Intro)
	}

	// 后期任意时间只补 intro/summary：paths 省略保留
	if _, err := ensureRegisteredByInit(root, proj, "", "一句话介绍", "几句话摘要"); err != nil {
		t.Fatal(err)
	}
	e = findTestEntry(t, root, "flowproj")
	if e.Intro != "一句话介绍" || e.Summary != "几句话摘要" {
		t.Errorf("元数据未同步: %+v", e)
	}
	if len(e.Paths) != 1 || e.Paths[0] != "wiki" {
		t.Errorf("paths 应保留: %v", e.Paths)
	}

	// AGENTS.md 块也应更新（check hook 从这里再同步）
	decl, err := parseWikiSync(proj)
	if err != nil {
		t.Fatal(err)
	}
	if decl.Intro != "一句话介绍" {
		t.Errorf("AGENTS.md 块 intro = %q", decl.Intro)
	}
}

// ensureRegisteredByInit 复刻 cmdInit 的核心路径（不经过 flag 解析）。
func ensureRegisteredByInit(root, proj, paths, intro, summary string) (*EnsureResult, error) {
	old, oldErr := parseWikiSync(proj)
	hadBlock := oldErr == nil
	decl := &wikiSyncDecl{}
	switch {
	case paths != "":
		decl.Paths = splitCSV(paths)
	case hadBlock:
		decl.Paths = old.Paths
	default:
		return nil, os.ErrInvalid
	}
	decl.Intro = old.getIntroOr(intro)
	decl.Summary = old.getSummaryOr(summary)
	if err := validatePaths(proj, decl.Paths); err != nil {
		return nil, err
	}
	if err := upsertWikiSyncBlock(proj, decl); err != nil {
		return nil, err
	}
	return ensureRegistered(root, proj, decl)
}

// --- wiki check（Stop hook 自动同步） ---

func TestCheckAutoSync(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "autoproj", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "README.md"), []byte("# 索引\n"), 0o644)

	// 无声明块（AGENTS.md 覆盖为空）→ check 静默无副作用
	os.WriteFile(filepath.Join(proj, "AGENTS.md"), []byte("# AGENTS\n"), 0o644)
	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	reg, _ := loadRegistry(wiki)
	if _, ok := findEntry(reg, "autoproj"); ok {
		t.Fatal("无声明块时不应注册")
	}

	// 有声明块 → 自动注册
	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	if err := upsertWikiSyncBlock(proj, &wikiSyncDecl{Paths: []string{"wiki"}, Intro: "自动同步介绍"}); err != nil {
		t.Fatal(err)
	}
	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	e := findTestEntry(t, wiki, "autoproj")
	if e.Intro != "自动同步介绍" {
		t.Errorf("check 应把 AGENTS.md 新 intro 同步进注册表: %+v", e)
	}

	// 破坏链接 → check 自动修复
	link := filepath.Join(wiki, "projects", "autoproj", "wiki")
	os.Remove(link)
	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Error("check 应自动重建失效链接")
	}
}

// --- wiki ls / grep / cat ---

func TestViews(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki) // resolveSpec/projectsRoot 依赖全局根
	proj := newTestProject(t, "viewproj", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "note.md"), []byte("grep 目标词 alpha\n"), 0o644)
	os.WriteFile(filepath.Join(proj, "wiki", "other.md"), []byte("没有关键词\n"), 0o644)
	decl, _ := parseWikiSync(proj)
	if _, err := ensureRegistered(wiki, proj, decl); err != nil {
		t.Fatal(err)
	}

	// resolveSpec 拒绝越界
	if _, err := resolveSpec("../etc"); err == nil {
		t.Error("../ 应被拒绝")
	}

	// resolveSpec 正常解析
	p, err := resolveSpec("viewproj/wiki/note.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("解析后的路径应存在: %s: %v", p, err)
	}

	// grep：能搜到目标词，输出可直接喂给 wiki cat 的 spec 路径
	re := regexp.MustCompile("目标词")
	var hits int
	err = walkFiles(projectsRoot(), nil, func(path string) error {
		n, _ := grepFile(path, re, projectsRoot())
		hits += n
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Errorf("应恰好 1 处命中，实际 %d", hits)
	}
}

// --- 发布记录 ---

func TestBlogRecordReconcile(t *testing.T) {
	wiki := newTestWiki(t)
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

// --- config ---

func TestConfigPrecedence(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	t.Setenv("WIKI_BLOG_REPO", "") // 确认未被外层污染

	// 默认
	if got := blogRepo(); got != defaultBlogRepo {
		t.Errorf("默认 blogRepo = %q", got)
	}
	// config.json
	cfg := &wikiConfig{BlogRepo: `X:\blog`}
	if err := cfg.save(wiki); err != nil {
		t.Fatal(err)
	}
	if got := blogRepo(); got != `X:\blog` {
		t.Errorf("config blogRepo = %q", got)
	}
	if got := blogPostsDir(); got != filepath.Join(`X:\blog`, postsRelDir) {
		t.Errorf("config blogPosts = %q", got)
	}
	// env 覆盖 config
	t.Setenv("WIKI_BLOG_REPO", `Y:\blog`)
	if got := blogRepo(); got != `Y:\blog` {
		t.Errorf("env blogRepo = %q", got)
	}
}
