package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
)

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

// initByHelper 复刻 CmdInit 的核心路径（不经过 flag 解析）。
func initByHelper(root, proj, paths, intro, summary string) (*EnsureResult, error) {
	reg, err := LoadRegistry(root)
	if err != nil {
		return nil, err
	}
	old, had := findEntryByRoot(reg, proj)
	decl := &WikiSyncDecl{}
	switch {
	case paths != "":
		decl.Paths = cli.SplitCSV(paths)
	case had:
		decl.Paths = old.Paths
	default:
		decl.Paths = discoverKnowledgeDirs(proj, []string{"wiki", "issues"})
		if len(decl.Paths) == 0 {
			return nil, os.ErrInvalid
		}
	}
	decl.Intro = cli.Pick(intro, old.Intro)
	decl.Summary = cli.Pick(summary, old.Summary)
	return EnsureRegistered(root, proj, decl)
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
	reg, _ := LoadRegistry(wiki)
	if _, ok := findEntryByRoot(reg, proj); ok {
		t.Fatal("未声明时不应注册")
	}

	// 注册后 → check 按注册表自动维护
	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}, Intro: "自动同步介绍"}); err != nil {
		t.Fatal(err)
	}
	// 破坏链接 → check 自动修复
	link := filepath.Join(wiki, ProjectsRootName, "autoproj", "wiki")
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

func TestDiscoverKnowledgeDirs(t *testing.T) {
	dir := newTestProject(t, "d", []string{"wiki", "docs/research"})
	os.MkdirAll(filepath.Join(dir, "issues"), 0o755)
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	// 只发现配置列表内的类型名，按配置顺序
	got := discoverKnowledgeDirs(dir, []string{"issues", "wiki", "zhishi"})
	if len(got) != 2 || got[0] != "issues" || got[1] != "wiki" {
		t.Errorf("discoverKnowledgeDirs = %v", got)
	}
}
