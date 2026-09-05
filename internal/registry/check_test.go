package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
)

func TestCheckAutoInvertsExistingKnowledgeDirs(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "false")
	proj := filepath.Join(t.TempDir(), "lateproj")
	if err := os.MkdirAll(filepath.Join(proj, "wiki"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "wiki", "note.md"), []byte("written-in-session\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	// 正文迁入知识库，原位变成窗口链接
	store := filepath.Join(wiki, ProjectsRootName, "lateproj", "wiki")
	if got, err := os.ReadFile(filepath.Join(store, "note.md")); err != nil || string(got) != "written-in-session\n" {
		t.Fatalf("会话中写入的正文应迁入知识库: %q %v", got, err)
	}
	if !invertedHealthy(store, filepath.Join(proj, "wiki")) {
		t.Fatal("check 应对已有知识目录做反转连接")
	}
	// 自动反转应写注册表，供后续会话复用
	reg, err := LoadRegistry(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findEntryByRoot(reg, proj); !ok {
		t.Error("自动反转应写注册表")
	}
}

func TestCheckNoopWhenNoKnowledgeDirs(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	proj := filepath.Join(t.TempDir(), "empty")
	os.MkdirAll(proj, 0o755)

	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	if cli.Lexists(filepath.Join(proj, "wiki")) || cli.Lexists(filepath.Join(proj, "issues")) {
		t.Error("项目无知识目录时 check 不应创建任何目录")
	}
	reg, err := LoadRegistry(wiki)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Projects) != 0 {
		t.Error("项目无知识目录时 check 不应写注册表")
	}
}

func TestCheckSkipsForbiddenPath(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	base := t.TempDir()
	proj := filepath.Join(base, "skipped")
	os.MkdirAll(filepath.Join(proj, "wiki"), 0o755)

	// 祖先目录在禁止名单 → 任意深度子孙跳过，已有知识目录也不反转
	t.Setenv("WIKI_FORBIDDEN_PATHS", base)
	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	if !isRealDir(filepath.Join(proj, "wiki")) {
		t.Error("命中 forbiddenPaths 的项目应跳过，知识目录保持真目录")
	}
}

func TestCheckWhitelistMode(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	base := t.TempDir()
	inside := filepath.Join(base, "team", "proj")
	outside := filepath.Join(t.TempDir(), "other")
	for _, p := range []string{inside, outside} {
		if err := os.MkdirAll(filepath.Join(p, "wiki"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("WIKI_HOOK_MODE", "whitelist")
	t.Setenv("WIKI_INCLUDE_PATHS", base)

	if err := checkLogic(wiki, inside); err != nil {
		t.Fatal(err)
	}
	if !invertedHealthy(filepath.Join(wiki, ProjectsRootName, "proj", "wiki"), filepath.Join(inside, "wiki")) {
		t.Error("whitelist 命中的子孙目录应自动反转")
	}
	if err := checkLogic(wiki, outside); err != nil {
		t.Fatal(err)
	}
	if !isRealDir(filepath.Join(outside, "wiki")) {
		t.Error("whitelist 未命中的目录应跳过，知识目录保持真目录")
	}
}

func TestCheckSkipsNameCollision(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	a := filepath.Join(t.TempDir(), "dup")
	b := filepath.Join(t.TempDir(), "dup")
	for _, p := range []string{a, b} {
		if err := os.MkdirAll(filepath.Join(p, "wiki"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := saveRegistry(wiki, &Registry{Projects: []ProjectEntry{{Name: "dup", Root: a, Paths: []string{"wiki"}}}}); err != nil {
		t.Fatal(err)
	}

	if err := checkLogic(wiki, b); err != nil {
		t.Fatal(err)
	}
	if !isRealDir(filepath.Join(b, "wiki")) {
		t.Error("同名不同根时应拒绝自动反转，避免覆盖显式 init 的声明")
	}
}
