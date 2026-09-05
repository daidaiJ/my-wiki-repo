package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigPrecedence(t *testing.T) {
	wiki := t.TempDir()
	t.Setenv("WIKI_ROOT", wiki)
	t.Setenv("WIKI_BLOG_REPO", "") // 确认未被外层污染
	t.Setenv("WIKI_KNOWLEDGE_DIRS", "")

	// 无默认：未配置时报错并给出指引（开源工具不含个人路径）
	if _, err := RequireBlogRepo(); err == nil || !strings.Contains(err.Error(), "wiki config set blogRepo") {
		t.Errorf("未配置 blogRepo 应报错并给指引: %v", err)
	}
	// knowledgeDirs 默认 wiki,issues
	if got := strings.Join(KnowledgeDirs(), ","); got != "wiki,issues" {
		t.Errorf("默认 knowledgeDirs = %q", got)
	}
	// config.json
	cfg := &wikiConfig{BlogRepo: `X:\blog`, KnowledgeDirs: []string{"wiki", "notes"}}
	if err := cfg.save(wiki); err != nil {
		t.Fatal(err)
	}
	if got := BlogRepo(); got != `X:\blog` {
		t.Errorf("config blogRepo = %q", got)
	}
	if got := BlogPostsDir(); got != filepath.Join(`X:\blog`, postsRelDir) {
		t.Errorf("config blogPosts = %q", got)
	}
	if got := strings.Join(KnowledgeDirs(), ","); got != "wiki,notes" {
		t.Errorf("config knowledgeDirs = %q", got)
	}
	// env 覆盖 config
	t.Setenv("WIKI_BLOG_REPO", `Y:\blog`)
	if got := BlogRepo(); got != `Y:\blog` {
		t.Errorf("env blogRepo = %q", got)
	}
	t.Setenv("WIKI_KNOWLEDGE_DIRS", "wiki,zhishi")
	if got := strings.Join(KnowledgeDirs(), ","); got != "wiki,zhishi" {
		t.Errorf("env knowledgeDirs = %q", got)
	}
}

func TestProjectGitignoreConfig(t *testing.T) {
	wiki := t.TempDir()
	t.Setenv("WIKI_ROOT", wiki)
	t.Setenv("WIKI_PROJECT_GITIGNORE", "")

	if !ProjectGitignore(wiki) {
		t.Error("缺省应为 true")
	}
	off := false
	cfg := &wikiConfig{ProjectGitignore: &off}
	if err := cfg.save(wiki); err != nil {
		t.Fatal(err)
	}
	if ProjectGitignore(wiki) {
		t.Error("config false 应生效")
	}
	t.Setenv("WIKI_PROJECT_GITIGNORE", "true")
	if !ProjectGitignore(wiki) {
		t.Error("环境变量应覆盖 config")
	}
}

func TestHugoConfig(t *testing.T) {
	wiki := t.TempDir()
	t.Setenv("WIKI_ROOT", wiki)
	t.Setenv("WIKI_HUGO_BIN", "")
	t.Setenv("WIKI_HUGO_SITE", "")

	// 默认：PATH 上的 hugo，站点相对 blogRepo 的 pandawo
	if got := HugoBin(); got != "hugo" {
		t.Errorf("默认 hugoBin = %q", got)
	}
	// config.json
	cfg := &wikiConfig{BlogRepo: `X:\blog`, HugoBin: `C:\hugo\hugo.exe`, HugoSite: "mysite"}
	if err := cfg.save(wiki); err != nil {
		t.Fatal(err)
	}
	if got := HugoBin(); got != `C:\hugo\hugo.exe` {
		t.Errorf("config hugoBin = %q", got)
	}
	if got := HugoSiteDir(); got != filepath.Join(`X:\blog`, "mysite") {
		t.Errorf("config hugoSite = %q", got)
	}
	// 绝对路径站点不拼接（平台无关：用临时目录构造绝对路径）
	absSite := filepath.Join(t.TempDir(), "site")
	cfg.HugoSite = absSite
	if err := cfg.save(wiki); err != nil {
		t.Fatal(err)
	}
	if got := HugoSiteDir(); got != absSite {
		t.Errorf("绝对 hugoSite = %q", got)
	}
	// env 覆盖 config
	t.Setenv("WIKI_HUGO_BIN", "hugo-test")
	t.Setenv("WIKI_HUGO_SITE", absSite)
	if got := HugoBin(); got != "hugo-test" {
		t.Errorf("env hugoBin = %q", got)
	}
	if got := HugoSiteDir(); got != absSite {
		t.Errorf("env hugoSite = %q", got)
	}
}

func TestHookScope(t *testing.T) {
	wiki := t.TempDir()
	t.Setenv("WIKI_ROOT", wiki)
	base := t.TempDir()
	proj := filepath.Join(base, "team", "app")

	// 默认 forbiddenList 模式：未命中禁止名单即生效
	t.Setenv("WIKI_HOOK_MODE", "")
	t.Setenv("WIKI_FORBIDDEN_PATHS", "")
	if ok, reason := HookApplies(wiki, proj); !ok || reason != "" {
		t.Errorf("forbiddenList 模式默认应生效，got ok=%v reason=%q", ok, reason)
	}
	// 祖先命中禁止名单：任意深度子孙全跳过
	t.Setenv("WIKI_FORBIDDEN_PATHS", base)
	if ok, reason := HookApplies(wiki, proj); ok || !strings.Contains(reason, "forbiddenPaths") {
		t.Errorf("禁止名单子孙应跳过，got ok=%v reason=%q", ok, reason)
	}
	if ok, _ := HookApplies(wiki, base); ok {
		t.Error("禁止名单条目自身应跳过")
	}
	other := filepath.Join(t.TempDir(), "other")
	if ok, _ := HookApplies(wiki, other); !ok {
		t.Error("禁止名单外目录应生效")
	}

	// whitelist 模式：仅 includePaths 子孙生效；未配置白名单时全不生效
	t.Setenv("WIKI_HOOK_MODE", "whitelist")
	t.Setenv("WIKI_FORBIDDEN_PATHS", "")
	t.Setenv("WIKI_INCLUDE_PATHS", filepath.Join(base, "team"))
	if ok, _ := HookApplies(wiki, proj); !ok {
		t.Error("whitelist 命中的子孙应生效")
	}
	if ok, reason := HookApplies(wiki, base); ok || reason == "" {
		t.Errorf("whitelist 未命中应跳过且给出原因，got ok=%v reason=%q", ok, reason)
	}
	t.Setenv("WIKI_INCLUDE_PATHS", "")
	if ok, reason := HookApplies(wiki, proj); ok || !strings.Contains(reason, "includePaths") {
		t.Errorf("whitelist 未配置白名单应全不生效，got ok=%v reason=%q", ok, reason)
	}

	// config.json 生效；环境变量覆盖 config.json
	cfg := &wikiConfig{HookMode: "forbiddenList", ForbiddenPaths: []string{proj}}
	if err := cfg.save(wiki); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WIKI_HOOK_MODE", "")
	t.Setenv("WIKI_FORBIDDEN_PATHS", "")
	if ok, _ := HookApplies(wiki, filepath.Join(proj, "sub")); ok {
		t.Error("config.json 禁止名单应生效")
	}
	if HookMode(wiki) != "forbiddenlist" {
		t.Errorf("config hookMode 应为 forbiddenlist，got %q", HookMode(wiki))
	}
	t.Setenv("WIKI_HOOK_MODE", "whitelist")
	if HookMode(wiki) != "whitelist" {
		t.Error("环境变量应覆盖 config hookMode")
	}

	// 相对路径条目按 wiki 根解析
	rel := &wikiConfig{HookMode: "forbiddenList", ForbiddenPaths: []string{"excluded-tree"}}
	if err := rel.save(wiki); err != nil {
		t.Fatal(err)
	}
	if got := ForbiddenPaths(wiki); len(got) != 1 || got[0] != filepath.Join(wiki, "excluded-tree") {
		t.Errorf("相对禁止路径应按 wiki 根解析，got %v", got)
	}
}
func TestResolveWikiRootByMarker(t *testing.T) {
	// 含 index.md 标记的目录被认定为 wiki 根
	marked := t.TempDir()
	os.WriteFile(filepath.Join(marked, "index.md"), []byte("# idx\n"), 0o644)
	if got := ResolveWikiRoot(marked); got != marked {
		t.Errorf("ResolveWikiRoot(%s) = %q, 期望原目录", marked, got)
	}
	// 无标记的普通目录不认定（例如可执行文件被复制到 PATH 目录的场景）
	if got := ResolveWikiRoot(t.TempDir()); got != "" {
		t.Errorf("无 index.md 的目录不应被认定为 wiki 根, got %q", got)
	}
}
