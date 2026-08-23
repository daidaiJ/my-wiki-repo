package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestWiki 建一个临时 my-wiki 根目录（含初始 index.md）。
func newTestWiki(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := saveRegistry(root, &Registry{}); err != nil {
		t.Fatal(err)
	}
	return root
}

// newTestProject 建一个带 wiki-sync 声明的假项目，返回其根目录。
func newTestProject(t *testing.T, name string, paths []string) string {
	t.Helper()
	dir := t.TempDir()
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

func findEntry(t *testing.T, root, name string) *ProjectEntry {
	t.Helper()
	reg, err := loadRegistry(root)
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

	entry := ProjectEntry{Name: "demoproj", Root: proj, Intro: "demoproj 的介绍", Paths: []string{"wiki", "docs/research"}}
	taken := map[string]bool{}
	for _, p := range entry.Paths {
		target, _ := filepath.Abs(filepath.Join(proj, p))
		link := linkPathFor(wiki, entry, p, taken)
		if err := createLink(target, link); err != nil {
			t.Fatalf("createLink(%s): %v", link, err)
		}
		// 链接可穿透访问目标内容
		if _, err := os.Stat(filepath.Join(link, ".")); err != nil {
			t.Errorf("链接 %s 不可访问: %v", link, err)
		}
	}

	// 手动补注册表并落盘（register 命令逻辑的等价路径，命令层在冒烟里验证）
	reg := &Registry{Projects: []ProjectEntry{entry}}
	if err := saveRegistry(wiki, reg); err != nil {
		t.Fatal(err)
	}
	got := findEntry(t, wiki, "demoproj")
	if got.Intro != "demoproj 的介绍" || len(got.Paths) != 2 {
		t.Errorf("登记内容不符: %+v", got)
	}

	// sync 应全部健康
	if problems := syncProject(wiki, *got, false); len(problems) != 0 {
		t.Errorf("刚注册就报问题: %v", problems)
	}
}

func TestSyncFixHealsBrokenLink(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "brokenproj", []string{"wiki"})
	entry := ProjectEntry{Name: "brokenproj", Root: proj, Paths: []string{"wiki"}}
	if err := saveRegistry(wiki, &Registry{Projects: []ProjectEntry{entry}}); err != nil {
		t.Fatal(err)
	}

	// 只注册登记、不建链接 → sync 报失效，--fix 修复
	got := findEntry(t, wiki, "brokenproj")
	if problems := syncProject(wiki, *got, false); len(problems) == 0 {
		t.Fatal("链接缺失时 sync 应报问题")
	}
	if problems := syncProject(wiki, *got, true); len(problems) != 0 {
		t.Fatalf("--fix 后仍有问题: %v", problems)
	}
	link := filepath.Join(wiki, "projects", "brokenproj", "wiki")
	if resolved, err := filepath.EvalSymlinks(link); err != nil {
		t.Fatalf("修复后链接不可用: %v", err)
	} else if !samePath(resolved, filepath.Join(proj, "wiki")) {
		t.Errorf("链接指向 %s，期望 %s", resolved, filepath.Join(proj, "wiki"))
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

func TestParseWikiSync(t *testing.T) {
	proj := newTestProject(t, "p", []string{"wiki", "docs/research"})
	decl, err := parseWikiSync(proj)
	if err != nil {
		t.Fatal(err)
	}
	if decl.Intro != "p 的介绍" || len(decl.Paths) != 2 {
		t.Errorf("decl = %+v", decl)
	}
}

func TestParseWikiSyncMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := parseWikiSync(dir); err == nil {
		t.Error("无 AGENTS.md 应报错")
	}
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("没有声明块"), 0o644)
	if _, err := parseWikiSync(dir); err == nil {
		t.Error("无 wiki-sync 块应报错")
	}
}
