package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- 拷贝模式核心：ensureCopyMode / mergeCopy ---

// copyProj 在项目侧知识目录里造初始内容（a.md、b.md、sub/c.md）。
func copyProj(t *testing.T, proj string) {
	t.Helper()
	wikiDir := filepath.Join(proj, "wiki")
	if err := os.MkdirAll(filepath.Join(wikiDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(wikiDir, "a.md"):        "v1\n",
		filepath.Join(wikiDir, "b.md"):        "bee\n",
		filepath.Join(wikiDir, "sub", "c.md"): "sea\n",
	}
	for p, content := range files {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEnsureCopyModeInitial(t *testing.T) {
	proj := filepath.Join(t.TempDir(), "proj")
	copyProj(t, proj)
	store := filepath.Join(t.TempDir(), "kb", "proj", "wiki")
	projWiki := filepath.Join(proj, "wiki")

	changed, err := ensureCopyMode(store, projWiki, true)
	if err != nil || !changed {
		t.Fatalf("首次拷贝: changed=%v err=%v", changed, err)
	}
	// 知识库有全量拷贝，项目侧原封不动（仍为真目录）
	got, err := os.ReadFile(filepath.Join(store, "a.md"))
	if err != nil || string(got) != "v1\n" {
		t.Errorf("知识库拷贝缺失: %s %v", got, err)
	}
	if !isRealDir(filepath.Join(proj, "wiki")) {
		t.Error("拷贝模式项目侧应保持真目录")
	}
	// 幂等：重跑无变化
	changed, err = ensureCopyMode(store, projWiki, true)
	if err != nil || changed {
		t.Errorf("重跑应无变化: changed=%v err=%v", changed, err)
	}
}

func TestMergeCopyIncremental(t *testing.T) {
	proj := filepath.Join(t.TempDir(), "proj")
	copyProj(t, proj)
	store := filepath.Join(t.TempDir(), "kb", "proj", "wiki")
	projWiki := filepath.Join(proj, "wiki")
	if _, err := ensureCopyMode(store, projWiki, true); err != nil {
		t.Fatal(err)
	}

	untouched := filepath.Join(store, "sub", "c.md")
	fi, err := os.Stat(untouched)
	if err != nil {
		t.Fatal(err)
	}
	untouchedMtime := fi.ModTime()

	// 项目侧：改 a.md、删 b.md、加 d.md；sub/c.md 不动
	if err := os.WriteFile(filepath.Join(proj, "wiki", "a.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(proj, "wiki", "b.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "wiki", "d.md"), []byte("dee\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 保证修改时间严格晚于首次拷贝（跨文件系统时间精度差异兜底）
	future := time.Now().Add(2 * time.Hour)
	os.Chtimes(filepath.Join(proj, "wiki", "a.md"), future, future)
	os.Chtimes(filepath.Join(proj, "wiki", "d.md"), future, future)

	changed, err := ensureCopyMode(store, projWiki, true)
	if err != nil || !changed {
		t.Fatalf("增量合并: changed=%v err=%v", changed, err)
	}
	if got, _ := os.ReadFile(filepath.Join(store, "a.md")); string(got) != "v2\n" {
		t.Errorf("修改未同步: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(store, "d.md")); string(got) != "dee\n" {
		t.Errorf("新增未同步: %q", got)
	}
	// 合并单向永不删文件：项目侧删掉的 b.md 在知识库保留
	if got, err := os.ReadFile(filepath.Join(store, "b.md")); err != nil || string(got) != "bee\n" {
		t.Errorf("项目侧删除不应传播到知识库: %q %v", got, err)
	}
	// 未变更文件不重写（mtime 不变 → 下次也不会重拷）
	fi, err = os.Stat(untouched)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().Equal(untouchedMtime) {
		t.Error("未变更文件被重写了")
	}
}

func TestMergeCopyPreservesKBEdits(t *testing.T) {
	proj := filepath.Join(t.TempDir(), "proj")
	copyProj(t, proj)
	store := filepath.Join(t.TempDir(), "kb", "proj", "wiki")
	projWiki := filepath.Join(proj, "wiki")
	if _, err := ensureCopyMode(store, projWiki, true); err != nil {
		t.Fatal(err)
	}

	// Obsidian 校对：把知识库侧 a.md 改晚（mtime 晚于项目侧）
	storeA := filepath.Join(store, "a.md")
	if err := os.WriteFile(storeA, []byte("obsidian 校对版\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Hour)
	os.Chtimes(storeA, later, later)

	if _, err := ensureCopyMode(store, projWiki, true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(storeA)
	if string(got) != "obsidian 校对版\n" {
		t.Errorf("知识库侧校对修改被覆盖: %q", got)
	}
}

func TestEnsureCopyModeRejectsLinkLayout(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "lproj", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "n.md"), []byte("x\n"), 0o644)
	store := filepath.Join(wiki, ProjectsRootName, "lproj", "wiki")

	// 先按链接模式建立布局
	if _, err := ensureInverted(store, filepath.Join(proj, "wiki"), true); err != nil {
		t.Fatal(err)
	}
	// 拷贝模式遇到两侧任一链接都拒绝动手（防错误覆盖搬迁）
	if _, err := ensureCopyMode(store, filepath.Join(proj, "wiki"), true); err == nil {
		t.Error("项目侧为链接时应拒绝")
	}
}

func TestEnsureCopyModeProvisionMissingProj(t *testing.T) {
	proj := filepath.Join(t.TempDir(), "ghost", "wiki")
	store := filepath.Join(t.TempDir(), "kb", "ghost", "wiki")

	// 项目侧还没出现（prepare hook 预置）：建空知识库目录
	changed, err := ensureCopyMode(store, proj, true)
	if err != nil || !changed {
		t.Fatalf("预置: changed=%v err=%v", changed, err)
	}
	if !isRealDir(store) || !dirEmpty(store) {
		t.Error("应预置空知识库目录")
	}
	// 非 provision（健康检查）时报错不动作
	if _, err := ensureCopyMode(store, proj, false); err == nil {
		t.Error("项目侧缺失且非 provision 应报错")
	}
}

// --- 注册表与命令层集成 ---

func TestEnsureRegisteredCopyMode(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "cmproj", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "n.md"), []byte("x\n"), 0o644)

	res, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}, Intro: "cm", Mode: ModeCopy})
	if err != nil {
		t.Fatal(err)
	}
	if res.Synced != 1 || res.Migrated != 0 || res.LinksRepaired != 0 {
		t.Errorf("EnsureResult 计数不符: %+v", res)
	}
	if !isRealDir(filepath.Join(proj, "wiki")) {
		t.Error("项目侧应保持真目录")
	}
	if !isRealDir(filepath.Join(wiki, ProjectsRootName, "cmproj", "wiki")) {
		t.Error("知识库侧应为真目录拷贝")
	}
	// 拷贝模式不写项目 .gitignore
	if _, err := os.Stat(filepath.Join(proj, ".gitignore")); !os.IsNotExist(err) {
		t.Error("拷贝模式不应维护项目 .gitignore")
	}
	// mode 持久化并回读
	e := findTestEntry(t, wiki, "cmproj")
	if e.Mode != ModeCopy {
		t.Errorf("注册表 mode 未持久化: %+v", e)
	}
	if problems := syncProject(wiki, *e, false); len(problems) != 0 {
		t.Errorf("copy 条目应健康: %v", problems)
	}
}

func TestSyncProjectCopyModeProblems(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "cp", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "n.md"), []byte("x\n"), 0o644)
	entry := ProjectEntry{Name: "cp", Root: proj, Paths: []string{"wiki"}, Mode: ModeCopy}

	// 知识库拷贝缺失 → 待修复；--fix 首次拷贝
	if problems := syncProject(wiki, entry, false); len(problems) == 0 {
		t.Fatal("知识库拷贝缺失应报问题")
	}
	if problems := syncProject(wiki, entry, true); len(problems) != 0 {
		t.Fatalf("--fix 后仍有问题: %v", problems)
	}
	// 项目侧被换成链接（残留链接模式布局）→ 异常不覆盖
	os.RemoveAll(filepath.Join(wiki, ProjectsRootName, "cp", "wiki"))
	if _, err := ensureInverted(filepath.Join(wiki, ProjectsRootName, "cp", "wiki"), filepath.Join(proj, "wiki"), true); err != nil {
		t.Fatal(err)
	}
	problems := syncProject(wiki, entry, true)
	if len(problems) == 0 {
		t.Fatal("项目侧为链接应报异常")
	}
	if !isLink(filepath.Join(proj, "wiki")) {
		t.Error("异常布局不应被拷贝模式擅自覆盖")
	}
}

func TestCheckHookSyncsCopyMode(t *testing.T) {
	wiki := newTestWiki(t)
	proj := newTestProject(t, "hookcp", []string{"wiki"})
	os.Remove(filepath.Join(proj, "AGENTS.md"))
	if _, err := EnsureRegistered(wiki, proj, &WikiSyncDecl{Paths: []string{"wiki"}, Mode: ModeCopy}); err != nil {
		t.Fatal(err)
	}

	// 项目侧新增文件后 check hook 应增量同步进知识库
	if err := os.WriteFile(filepath.Join(proj, "wiki", "later.md"), []byte("late\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkLogic(wiki, proj); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(wiki, ProjectsRootName, "hookcp", "wiki", "later.md")); err != nil || string(got) != "late\n" {
		t.Errorf("check 未增量同步: %q %v", got, err)
	}
}

// --- wiki init --mode ---

func TestInitModeFlagAndPersistence(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	t.Setenv("WIKI_DEFAULT_MODE", "")
	proj := newTestProject(t, "mdproj", []string{"wiki"})
	os.WriteFile(filepath.Join(proj, "wiki", "n.md"), []byte("x\n"), 0o644)

	// 新项目 --mode copy 接入
	if err := CmdInit([]string{proj, "--mode", "copy"}); err != nil {
		t.Fatal(err)
	}
	e := findTestEntry(t, wiki, "mdproj")
	if e.Mode != ModeCopy {
		t.Fatalf("--mode copy 未生效: %+v", e)
	}
	if !isRealDir(filepath.Join(proj, "wiki")) {
		t.Error("copy 接入不应改动项目侧目录")
	}

	// 重复 init 不带 --mode：保留 copy
	if err := CmdInit([]string{proj}); err != nil {
		t.Fatal(err)
	}
	if e := findTestEntry(t, wiki, "mdproj"); e.Mode != ModeCopy {
		t.Errorf("重复 init 应保留原模式: %+v", e)
	}

	// --mode link 与注册值冲突：告警并保留 copy
	if err := CmdInit([]string{proj, "--mode", "link"}); err != nil {
		t.Fatal(err)
	}
	if e := findTestEntry(t, wiki, "mdproj"); e.Mode != ModeCopy {
		t.Errorf("模式冲突应保留原模式: %+v", e)
	}
}

func TestInitCopyModeRejectsFlatPath(t *testing.T) {
	wiki := newTestWiki(t)
	t.Setenv("WIKI_ROOT", wiki)
	dir := filepath.Join(t.TempDir(), "flat")
	os.MkdirAll(dir, 0o755)

	if err := CmdInit([]string{dir, "--paths", ".", "--mode", "copy"}); err == nil {
		t.Error("拷贝模式平铺接入应报错")
	}
}
