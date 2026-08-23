package view

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/daidaiJ/my-wiki-repo/internal/registry"
)

func TestViews(t *testing.T) {
	// view 依赖全局 wiki 根（resolveSpec/ProjectsRoot），指向临时根
	wiki := t.TempDir()
	t.Setenv("WIKI_ROOT", wiki)
	proj := filepath.Join(t.TempDir(), "viewproj")
	wikiDir := filepath.Join(proj, "wiki")
	os.MkdirAll(wikiDir, 0o755)
	os.WriteFile(filepath.Join(wikiDir, "note.md"), []byte("grep 目标词 alpha\n"), 0o644)
	os.WriteFile(filepath.Join(wikiDir, "other.md"), []byte("没有关键词\n"), 0o644)
	if _, err := registry.EnsureRegistered(wiki, proj, &registry.WikiSyncDecl{Paths: []string{"wiki"}}); err != nil {
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

	// grep：能搜到目标词（走链接视图），恰好 1 处命中
	re := regexp.MustCompile("目标词")
	var hits int
	if err := walkFiles(registry.ProjectsRoot(), nil, func(path string) error {
		n, _ := grepFile(path, re, registry.ProjectsRoot())
		hits += n
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Errorf("应恰好 1 处命中，实际 %d", hits)
	}
}
