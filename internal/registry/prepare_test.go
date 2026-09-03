package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareCreatesWindowForRegisteredProject(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "true")
	proj := filepath.Join(t.TempDir(), "prepproj")
	os.MkdirAll(proj, 0o755)

	entry := ProjectEntry{Name: "prepproj", Root: proj, Paths: []string{"wiki", "issues"}}
	if err := saveRegistry(wiki, &Registry{Projects: []ProjectEntry{entry}}); err != nil {
		t.Fatal(err)
	}

	if err := prepareLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	storeWiki := filepath.Join(wiki, ProjectsRootName, "prepproj", "wiki")
	windowWiki := filepath.Join(proj, "wiki")
	if !invertedHealthy(storeWiki, windowWiki) {
		t.Fatal("prepare 应为已注册项目建立窗口链接")
	}
	storeIssues := filepath.Join(wiki, ProjectsRootName, "prepproj", "issues")
	windowIssues := filepath.Join(proj, "issues")
	if !invertedHealthy(storeIssues, windowIssues) {
		t.Fatal("prepare 应为所有声明路径建立窗口")
	}
	gi, err := os.ReadFile(filepath.Join(proj, ".gitignore"))
	if err != nil || !strings.Contains(string(gi), "/wiki") || !strings.Contains(string(gi), "/issues") {
		t.Errorf("prepare 应维护 gitignore: %s %v", gi, err)
	}

	// 透过窗口写入
	if err := os.WriteFile(filepath.Join(windowWiki, "note.md"), []byte("via-prepare\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(storeWiki, "note.md"))
	if string(got) != "via-prepare\n" {
		t.Errorf("窗口写入未落到知识库: %q", got)
	}
}

func TestPrepareSilentWhenUnregistered(t *testing.T) {
	wiki := newTestWiki(t)
	proj := filepath.Join(t.TempDir(), "unknown")
	os.MkdirAll(proj, 0o755)
	if err := prepareLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	if isLink(filepath.Join(proj, "wiki")) {
		t.Error("未注册项目 prepare 应静默，不建链接")
	}
}

func TestInitProvisionsEmptyKnowledgeDirs(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "false")
	proj := filepath.Join(t.TempDir(), "newproj")
	os.MkdirAll(proj, 0o755)

	res, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.LinksRepaired == 0 {
		t.Error("init 应新建空知识目录与窗口链接")
	}
	store := filepath.Join(wiki, ProjectsRootName, "newproj", "wiki")
	if !invertedHealthy(store, filepath.Join(proj, "wiki")) {
		t.Fatal("init 应对空路径执行 provision")
	}
}
