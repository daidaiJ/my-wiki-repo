package registry

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateThenInvertAndWriteThrough(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "true")
	proj := newTestProject(t, "invproj", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "a.md"), []byte("from-project\n"), 0o644)

	res, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Migrated != 1 {
		t.Errorf("Migrated = %d, want 1", res.Migrated)
	}
	store := filepath.Join(wiki, ProjectsRootName, "invproj", "wiki")
	window := filepath.Join(proj, "wiki")
	if !invertedHealthy(store, window) {
		t.Fatal("应为先迁移后替换的反向布局")
	}
	if !isRealDir(store) || isLink(store) {
		t.Fatal("知识库侧必须是真目录")
	}
	if res.GitignoreAction != "created" {
		t.Errorf("gitignore action = %q, want created", res.GitignoreAction)
	}

	// 透过窗口写入，正文落在知识库
	if err := os.WriteFile(filepath.Join(window, "b.md"), []byte("via-window\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(store, "b.md"))
	if err != nil || string(got) != "via-window\n" {
		t.Errorf("窗口写入未落到知识库: %s %v", got, err)
	}

	// 幂等
	res2, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Migrated != 0 || res2.LinksRepaired != 0 {
		t.Errorf("健康布局不应再迁移/修复: %+v", res2)
	}
}

func TestLegacyForwardMigrates(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "false")
	proj := newTestProject(t, "legacy", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "old.md"), []byte("legacy-body\n"), 0o644)
	store := filepath.Join(wiki, ProjectsRootName, "legacy", "wiki")
	if err := createLink(filepath.Join(proj, "wiki"), store); err != nil {
		t.Fatal(err)
	}
	if !legacyForward(store, filepath.Join(proj, "wiki")) {
		t.Fatal("前置条件：应为旧正向链接")
	}

	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}
	if !invertedHealthy(store, filepath.Join(proj, "wiki")) {
		t.Fatal("旧正向链接应被迁移并反转")
	}
	got, _ := os.ReadFile(filepath.Join(store, "old.md"))
	if string(got) != "legacy-body\n" {
		t.Errorf("迁移丢失正文: %q", got)
	}
	if _, err := os.Stat(filepath.Join(proj, ".gitignore")); err == nil {
		t.Fatal("projectGitignore=false 时不应写 .gitignore")
	}
}

func TestGitignoreAppend(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "true")
	proj := newTestProject(t, "gi", []string{"wiki", "issues"})
	os.WriteFile(filepath.Join(proj, ".gitignore"), []byte("bin/\n"), 0o644)
	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki", "issues"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(proj, ".gitignore"))
	s := string(got)
	if !strings.HasPrefix(s, "bin/\n") {
		t.Error("既有 gitignore 内容应保留")
	}
	if !strings.Contains(s, "/wiki") || !strings.Contains(s, "/issues") {
		t.Errorf("应追加知识目录条目:\n%s", s)
	}
	res, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki", "issues"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.GitignoreAction != "" {
		t.Errorf("条目已在时不应再写盘, action=%s", res.GitignoreAction)
	}
}

func TestUnlinkKeepsStoreUnlessPurge(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "false")
	proj := newTestProject(t, "keepme", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "keep.md"), []byte("keep\n"), 0o644)
	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}
	storeFile := filepath.Join(wiki, ProjectsRootName, "keepme", "wiki", "keep.md")

	if err := CmdUnlink([]string{"keepme"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(storeFile); err != nil {
		t.Fatalf("unlink 默认应保留知识正文: %v", err)
	}
	if isLink(filepath.Join(proj, "wiki")) {
		t.Error("窗口链接应被摘除")
	}

	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}
	if err := CmdUnlink([]string{"keepme", "--purge"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wiki, ProjectsRootName, "keepme")); !os.IsNotExist(err) {
		t.Errorf("--purge 应删除知识库目录, err=%v", err)
	}
}

func TestBundleCloneAndZip(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "false")
	proj := newTestProject(t, "bundled", []string{"wiki"})
	os.MkdirAll(filepath.Join(proj, "wiki", "sub"), 0o755)
	os.WriteFile(filepath.Join(proj, "wiki", "sub", "n.md"), []byte("bundle-me\n"), 0o644)
	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(wiki, "my-bundle")
	if err := CmdBundle([]string{"--dir", out, "--archive", "zip", "--name", "notes"}); err != nil {
		t.Fatal(err)
	}
	cloned := filepath.Join(out, "bundled", "wiki", "sub", "n.md")
	got, err := os.ReadFile(cloned)
	if err != nil || string(got) != "bundle-me\n" {
		t.Errorf("目录结构克隆失败: %s %v", got, err)
	}
	zipPath := filepath.Join(wiki, "notes.zip")
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	found := false
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "sub/n.md") || strings.HasSuffix(f.Name, `sub\n.md`) {
			found = true
		}
	}
	if !found {
		t.Errorf("zip 中未找到克隆文件, files=%v", zipNames(zr))
	}
}

func zipNames(zr *zip.ReadCloser) []string {
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

func TestBothRealDirsIdenticalRecovers(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "false")
	proj := newTestProject(t, "dup", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "a.md"), []byte("same\n"), 0o644)
	store := filepath.Join(wiki, ProjectsRootName, "dup", "wiki")
	os.MkdirAll(store, 0o755)
	os.WriteFile(filepath.Join(store, "a.md"), []byte("same\n"), 0o644)

	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}
	if !invertedHealthy(store, filepath.Join(proj, "wiki")) {
		t.Fatal("两侧同内容时应恢复为窗口，而不是报错拒绝")
	}
	got, _ := os.ReadFile(filepath.Join(store, "a.md"))
	if string(got) != "same\n" {
		t.Errorf("知识库正文被改写: %q", got)
	}
}

func TestBothRealDirsMergesProjectExtra(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "false")
	proj := newTestProject(t, "extra", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "old.md"), []byte("keep\n"), 0o644)
	os.WriteFile(filepath.Join(proj, "wiki", "new.md"), []byte("from-proj\n"), 0o644)
	store := filepath.Join(wiki, ProjectsRootName, "extra", "wiki")
	os.MkdirAll(store, 0o755)
	os.WriteFile(filepath.Join(store, "old.md"), []byte("keep\n"), 0o644)
	os.WriteFile(filepath.Join(store, "obsidian.md"), []byte("from-store\n"), 0o644)

	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}
	if !invertedHealthy(store, filepath.Join(proj, "wiki")) {
		t.Fatal("应合并后换成窗口")
	}
	for _, tc := range []struct{ name, want string }{
		{"old.md", "keep\n"},
		{"new.md", "from-proj\n"},
		{"obsidian.md", "from-store\n"},
	} {
		got, err := os.ReadFile(filepath.Join(store, tc.name))
		if err != nil || string(got) != tc.want {
			t.Errorf("%s = %q %v, want %q", tc.name, got, err, tc.want)
		}
	}
}

func TestBothRealDirsConflictKeepsStore(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "false")
	proj := newTestProject(t, "conf", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "a.md"), []byte("project-side\n"), 0o644)
	store := filepath.Join(wiki, ProjectsRootName, "conf", "wiki")
	os.MkdirAll(store, 0o755)
	os.WriteFile(filepath.Join(store, "a.md"), []byte("store-side\n"), 0o644)

	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(store, "a.md"))
	if string(got) != "store-side\n" {
		t.Fatalf("冲突时不得覆盖知识库: %q", got)
	}
	side, err := os.ReadFile(filepath.Join(store, "a.md"+conflictSuffix))
	if err != nil || string(side) != "project-side\n" {
		t.Fatalf("项目侧冲突副本应另存: %q %v", side, err)
	}
	if !invertedHealthy(store, filepath.Join(proj, "wiki")) {
		t.Fatal("冲突另存后仍应换成窗口")
	}
}

func TestRelocateDirFallsBackWhenRenameFails(t *testing.T) {
	t.Cleanup(func() { renameDir = os.Rename })
	renameDir = func(string, string) error { return fmt.Errorf("simulated lock") }

	src := filepath.Join(t.TempDir(), "src")
	os.MkdirAll(filepath.Join(src, "sub"), 0o755)
	os.WriteFile(filepath.Join(src, "a.md"), []byte("a\n"), 0o644)
	os.WriteFile(filepath.Join(src, "sub", "b.md"), []byte("b\n"), 0o644)
	dst := filepath.Join(t.TempDir(), "dst")

	if err := relocateDir(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) {
		t.Fatalf("源目录应已搬走: %v", err)
	}
	for _, rel := range []string{"a.md", filepath.Join("sub", "b.md")} {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Fatal(err)
		}
		want := "a\n"
		if strings.HasSuffix(rel, "b.md") {
			want = "b\n"
		}
		if string(got) != want {
			t.Errorf("%s = %q", rel, got)
		}
	}
}

func TestMigrateInvertFallbackWhenProjectRenameFails(t *testing.T) {
	t.Cleanup(func() { renameDir = os.Rename })
	renameDir = func(old, new string) error {
		if strings.Contains(old, ProjectsRootName) {
			return os.Rename(old, new)
		}
		if filepath.Base(old) == "wiki" {
			return fmt.Errorf("simulated lock")
		}
		return os.Rename(old, new)
	}

	wiki := newTestWiki(t)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "false")
	proj := newTestProject(t, "locked", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "note.md"), []byte("safe\n"), 0o644)

	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(wiki, ProjectsRootName, "locked", "wiki")
	if !invertedHealthy(store, filepath.Join(proj, "wiki")) {
		t.Fatal("项目目录改名失败时应走逐项搬走，最终仍能换成窗口")
	}
	got, _ := os.ReadFile(filepath.Join(store, "note.md"))
	if string(got) != "safe\n" {
		t.Errorf("迁移丢失正文: %q", got)
	}
}

func TestRelocateDirRefusesExistingDest(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "dst")
	os.MkdirAll(src, 0o755)
	os.MkdirAll(dst, 0o755)
	os.WriteFile(filepath.Join(src, "a.md"), []byte("src\n"), 0o644)
	os.WriteFile(filepath.Join(dst, "a.md"), []byte("dst\n"), 0o644)
	if err := relocateDir(src, dst); err == nil {
		t.Fatal("目标已存在时应拒绝覆盖")
	}
	got, _ := os.ReadFile(filepath.Join(dst, "a.md"))
	if string(got) != "dst\n" {
		t.Errorf("目标被覆盖: %q", got)
	}
	got, _ = os.ReadFile(filepath.Join(src, "a.md"))
	if string(got) != "src\n" {
		t.Errorf("源被删改: %q", got)
	}
}
