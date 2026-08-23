package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDeclarationShrinkCleansOrphanLinks 声明收缩（如 --paths 变更）后，
// 不再声明的旧链接应被清理，保留的链接不受影响。
func TestDeclarationShrinkCleansOrphanLinks(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "shrinkproj", []string{"wiki", "issues"})
	os.Remove(filepath.Join(proj, "AGENTS.md")) // 集中式声明，项目无块

	if _, err := ensureRegistered(wiki, proj, &wikiSyncDecl{Paths: []string{"wiki", "issues"}}); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(wiki, "projects", "shrinkproj", "issues")
	if _, err := os.Lstat(orphan); err != nil {
		t.Fatalf("应有 issues 链接: %v", err)
	}

	// 收缩声明为仅 wiki
	if _, err := ensureRegistered(wiki, proj, &wikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("孤儿链接应被清理: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(wiki, "projects", "shrinkproj", "wiki")); err != nil {
		t.Error("保留声明的 wiki 链接不应被误删")
	}
}

// --- wiki init（声明集中化：项目仓库零足迹） ---

func TestInitFlowAndMetadataUpdate(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "flowproj", []string{"wiki"})
	// 声明不再写入项目 AGENTS.md：接入后项目目录应无任何新文件
	os.Remove(filepath.Join(proj, "AGENTS.md"))

	// 无知识目录且未注册 → 自动发现失败应报错
	empty := filepath.Join(t.TempDir(), "flowproj")
	os.MkdirAll(empty, 0o755)
	if res, err := initByHelper(wiki, empty, "", "", ""); err == nil {
		t.Fatalf("无可发现目录应报错, got %+v", res)
	}
	if _, err := os.Stat(filepath.Join(empty, "AGENTS.md")); err == nil {
		t.Fatal("init 不应在项目仓库创建 AGENTS.md")
	}

	// 自动发现：wiki/ 存在则直接接入
	if _, err := initByHelper(wiki, proj, "", "", ""); err != nil {
		t.Fatal(err)
	}
	e := findTestEntry(t, wiki, "flowproj")
	if len(e.Paths) != 1 || e.Paths[0] != "wiki" {
		t.Fatalf("自动发现应接入 wiki: %+v", e)
	}
	if e.Intro != "" {
		t.Errorf("intro 应为空待补充, got %q", e.Intro)
	}

	// 后期任意时间只补 intro/summary：paths 省略保留
	if _, err := initByHelper(wiki, proj, "", "一句话介绍", "几句话摘要"); err != nil {
		t.Fatal(err)
	}
	e = findTestEntry(t, wiki, "flowproj")
	if e.Intro != "一句话介绍" || e.Summary != "几句话摘要" {
		t.Errorf("元数据未更新: %+v", e)
	}
	if len(e.Paths) != 1 || e.Paths[0] != "wiki" {
		t.Errorf("paths 应保留: %v", e.Paths)
	}
	if _, err := os.Stat(filepath.Join(proj, "AGENTS.md")); err == nil {
		t.Fatal("元数据补充也不应在项目仓库创建文件")
	}
}

// initByHelper 复刻 cmdInit 的核心路径（不经过 flag 解析）。
func initByHelper(root, proj, paths, intro, summary string) (*EnsureResult, error) {
	reg, err := loadRegistry(root)
	if err != nil {
		return nil, err
	}
	old, had := findEntryByRoot(reg, proj)
	decl := &wikiSyncDecl{}
	switch {
	case paths != "":
		decl.Paths = splitCSV(paths)
	case had:
		decl.Paths = old.Paths
	default:
		decl.Paths = discoverKnowledgeDirs(proj, knowledgeDirs())
		if len(decl.Paths) == 0 {
			return nil, os.ErrInvalid
		}
	}
	decl.Intro = pick(intro, old.Intro)
	decl.Summary = pick(summary, old.Summary)
	return ensureRegistered(root, proj, decl)
}

// --- wiki check（Stop hook 自动同步：注册表优先，AGENTS.md 块仅 opt-in 回退） ---

func TestCheckAutoSyncByRegistry(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "autoproj", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "README.md"), []byte("# 索引\n"), 0o644)
	os.Remove(filepath.Join(proj, "AGENTS.md")) // 集中式：项目里没有声明块

	// 未注册且无声明块 → check 静默无副作用
	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	reg, _ := loadRegistry(wiki)
	if _, ok := findEntryByRoot(reg, proj); ok {
		t.Fatal("未声明时不应注册")
	}

	// 注册后 → check 按注册表自动维护
	if _, err := ensureRegistered(wiki, proj, &wikiSyncDecl{Paths: []string{"wiki"}, Intro: "自动同步介绍"}); err != nil {
		t.Fatal(err)
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

func TestCheckFallbackToAgentsBlock(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "blockproj", []string{"wiki"}) // 自带 AGENTS.md 声明块（opt-in）
	// 未注册但有声明块 → check 仍应自动接入
	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	e := findTestEntry(t, wiki, "blockproj")
	if len(e.Paths) != 1 || e.Paths[0] != "wiki" {
		t.Errorf("声明块回退未生效: %+v", e)
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
	t.Setenv("WIKI_KNOWLEDGE_DIRS", "")

	// 无默认：未配置时报错并给出指引（开源工具不含个人路径）
	if _, err := requireBlogRepo(); err == nil || !strings.Contains(err.Error(), "wiki config set blogRepo") {
		t.Errorf("未配置 blogRepo 应报错并给指引: %v", err)
	}
	// knowledgeDirs 默认 wiki,issues
	if got := strings.Join(knowledgeDirs(), ","); got != "wiki,issues" {
		t.Errorf("默认 knowledgeDirs = %q", got)
	}
	// config.json
	cfg := &wikiConfig{BlogRepo: `X:\blog`, KnowledgeDirs: []string{"wiki", "notes"}}
	if err := cfg.save(wiki); err != nil {
		t.Fatal(err)
	}
	if got := blogRepo(); got != `X:\blog` {
		t.Errorf("config blogRepo = %q", got)
	}
	if got := blogPostsDir(); got != filepath.Join(`X:\blog`, postsRelDir) {
		t.Errorf("config blogPosts = %q", got)
	}
	if got := strings.Join(knowledgeDirs(), ","); got != "wiki,notes" {
		t.Errorf("config knowledgeDirs = %q", got)
	}
	// env 覆盖 config
	t.Setenv("WIKI_BLOG_REPO", `Y:\blog`)
	if got := blogRepo(); got != `Y:\blog` {
		t.Errorf("env blogRepo = %q", got)
	}
	t.Setenv("WIKI_KNOWLEDGE_DIRS", "wiki,zhishi")
	if got := strings.Join(knowledgeDirs(), ","); got != "wiki,zhishi" {
		t.Errorf("env knowledgeDirs = %q", got)
	}
}
