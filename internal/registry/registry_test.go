package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
)

// newTestWiki 建一个临时 wiki 根目录（含初始 index.md）。
func newTestWiki(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := saveRegistry(root, &Registry{}); err != nil {
		t.Fatal(err)
	}
	return root
}

// newTestProject 建一个带 wiki-sync 声明块的假项目（目录名即项目名），返回其根目录。
func newTestProject(t *testing.T, name string, paths []string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(p)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	decl := `<!-- wiki-sync
{"paths": ["` + strings.Join(paths, `", "`) + `"], "intro": "` + name + ` 的介绍"}
-->`
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(decl), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func findTestEntry(t *testing.T, root, name string) *ProjectEntry {
	t.Helper()
	reg, err := LoadRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	for i := range reg.Projects {
		if reg.Projects[i].Name == name {
			return &reg.Projects[i]
		}
	}
	t.Fatalf("项目 %s 未注册", name)
	return nil
}

func TestRegisterAndUnlink(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "demoproj", []string{"wiki", "docs/research"})
	os.WriteFile(filepath.Join(proj, "wiki", "note.md"), []byte("hello\n"), 0o644)

	res, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki", "docs/research"}, Intro: "demoproj 的介绍"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Migrated == 0 {
		t.Error("首次接入应迁移正文")
	}
	store := filepath.Join(wiki, ProjectsRootName, "demoproj", "wiki")
	if !isRealDir(store) {
		t.Fatalf("知识库侧应为真目录: %s", store)
	}
	if !isLink(filepath.Join(proj, "wiki")) {
		t.Fatal("项目侧 wiki 应为窗口链接")
	}
	got, err := os.ReadFile(filepath.Join(store, "note.md"))
	if err != nil || string(got) != "hello\n" {
		t.Errorf("正文未迁入知识库: %s %v", got, err)
	}
	// 透过窗口链接仍能读
	if _, err := os.Stat(filepath.Join(proj, "wiki", "note.md")); err != nil {
		t.Errorf("窗口链接不可访问: %v", err)
	}

	reg := findTestEntry(t, wiki, "demoproj")
	if reg.Intro != "demoproj 的介绍" || len(reg.Paths) != 2 {
		t.Errorf("登记内容不符: %+v", reg)
	}
	if problems := syncProject(wiki, *reg, false); len(problems) != 0 {
		t.Errorf("刚注册就报问题: %v", problems)
	}
}

func TestSyncFixHealsBrokenLink(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "brokenproj", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "n.md"), []byte("x\n"), 0o644)
	entry := ProjectEntry{Name: "brokenproj", Root: proj, Paths: []string{"wiki"}}
	if err := saveRegistry(wiki, &Registry{Projects: []ProjectEntry{entry}}); err != nil {
		t.Fatal(err)
	}

	got := findTestEntry(t, wiki, "brokenproj")
	if problems := syncProject(wiki, *got, false); len(problems) == 0 {
		t.Fatal("未迁移时应报问题")
	}
	if problems := syncProject(wiki, *got, true); len(problems) != 0 {
		t.Fatalf("--fix 后仍有问题: %v", problems)
	}
	store := filepath.Join(wiki, ProjectsRootName, "brokenproj", "wiki")
	if !invertedHealthy(store, filepath.Join(proj, "wiki")) {
		t.Fatal("--fix 后应为反向窗口布局")
	}
}

func TestSyncDeadProject(t *testing.T) {
	wiki := newTestWiki(t)
	ghost := filepath.Join(t.TempDir(), "gone") // 从未创建
	entry := ProjectEntry{Name: "ghost", Root: ghost, Paths: []string{"wiki"}}
	problems := syncProject(wiki, entry, true)
	if len(problems) == 0 {
		t.Fatal("项目目录不存在应报 dead")
	}
	if !strings.Contains(problems[0], "不存在") {
		t.Errorf("问题描述不含“不存在”: %s", problems[0])
	}
}

func TestLinkNameCollision(t *testing.T) {
	taken := map[string]bool{}
	first := linkName("docs/wiki", taken)
	second := linkName("notes/wiki", taken)
	if first == second {
		t.Fatalf("两个 wiki 路径得到同名链接: %s", first)
	}
	if first != "wiki" {
		t.Errorf("首个链接名应为 wiki，得到 %s", first)
	}
	if second != "notes_wiki" {
		t.Errorf("冲突链接名应为 notes_wiki，得到 %s", second)
	}
}

func TestFlatKnowledgeDirRootPath(t *testing.T) {
	wiki := newTestWiki(t)
	// 项目根本身就是知识目录（声明 "."）
	dir := filepath.Join(t.TempDir(), "flatdocs")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "note.md"), []byte("内容\n"), 0o644)

	res, err := EnsureRegistered(wiki, dir, &WikiSyncDecl{Paths: []string{"."}, Intro: "平铺知识目录"})
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(wiki, ProjectsRootName, "flatdocs", "flatdocs")
	if resolved, err := filepath.EvalSymlinks(link); err != nil {
		t.Fatalf("根路径链接未建立: %v", err)
	} else if !cli.SamePath(resolved, dir) {
		t.Errorf("链接指向 %s，期望 %s", resolved, dir)
	}
	// 根目录的 README 检查同样生效
	if len(res.ReadmeWarnings) != 1 {
		t.Errorf("根目录缺 README 应有告警: %v", res.ReadmeWarnings)
	}
}

// TestDeclarationShrinkCleansOrphanLinks 声明收缩（如 --paths 变更）后，
// 不再声明的旧链接应被清理，保留的链接不受影响。
func TestDeclarationShrinkCleansOrphanLinks(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "shrinkproj", []string{"wiki", "issues"})
	os.Remove(filepath.Join(proj, "AGENTS.md")) // 集中式声明，项目无块

	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki", "issues"}}); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(wiki, ProjectsRootName, "shrinkproj", "issues")
	if !isRealDir(orphan) {
		t.Fatalf("应有 issues 知识目录: %v", orphan)
	}

	// 收缩声明为仅 wiki：项目侧 issues 窗口应摘除，知识库 issues 正文保留
	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}
	if isLink(filepath.Join(proj, "issues")) {
		t.Error("不再声明的项目侧窗口应被摘除")
	}
	if !isRealDir(orphan) {
		t.Errorf("知识库 issues 正文应保留: %v", orphan)
	}
	if _, err := os.Lstat(filepath.Join(wiki, ProjectsRootName, "shrinkproj", "wiki")); err != nil {
		t.Error("保留声明的 wiki 不应被误删")
	}
}

func TestParseWikiSync(t *testing.T) {
	proj := newTestProject(t, "p", []string{"wiki", "docs/research"})
	decl, err := ParseWikiSync(proj)
	if err != nil {
		t.Fatal(err)
	}
	if decl.Intro != "p 的介绍" || len(decl.Paths) != 2 {
		t.Errorf("decl = %+v", decl)
	}
}

func TestParseWikiSyncMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := ParseWikiSync(dir); err == nil {
		t.Error("无 AGENTS.md 应报错")
	}
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("没有声明块"), 0o644)
	if _, err := ParseWikiSync(dir); err == nil {
		t.Error("无 wiki-sync 块应报错")
	}
}
