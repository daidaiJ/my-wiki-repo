// Package config 管理 wiki 的全部环境解析：数据根目录定位、config.json、
// 知识目录类型名与博客仓库配置。优先级：环境变量 > config.json > 内置默认值。
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
)

// postsRelDir 是文章目录相对博客仓库根的默认位置（平台无关拼接）。
var postsRelDir = filepath.Join("pandawo", "content", "post")

// WikiRoot 解析 wiki 数据根目录（开源可移植，无任何硬编码个人路径）：
//
//  1. WIKI_ROOT 环境变量（显式指定，最高优先）
//  2. wiki 可执行文件所在目录 —— 存在 index.md 标记时认定（clone 本仓库后 go build 的默认形态）
//  3. 当前目录 —— 存在 index.md 标记时认定（可执行文件在 PATH 上、人在 wiki 根里执行的场景）
//  4. 兜底可执行文件所在目录（首次使用时由工具在该目录生成 index.md）
func WikiRoot() string {
	if v := os.Getenv("WIKI_ROOT"); v != "" {
		return v
	}
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
		if dir := ResolveWikiRoot(exeDir); dir != "" {
			return dir
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if dir := ResolveWikiRoot(wd); dir != "" {
			return dir
		}
	}
	if exeDir != "" {
		return exeDir
	}
	wd, _ := os.Getwd()
	return wd
}

// ResolveWikiRoot 判断某目录是否为 wiki 根（含 index.md 注册表标记），是则返回该目录。
func ResolveWikiRoot(dir string) string {
	if fi, err := os.Stat(filepath.Join(dir, "index.md")); err == nil && !fi.IsDir() {
		return dir
	}
	return ""
}

// wikiConfig 是 wiki 根目录下的 config.json。fork 者在此配置自己的环境。
type wikiConfig struct {
	BlogRepo         string   `json:"blogRepo,omitempty"`
	BlogPosts        string   `json:"blogPosts,omitempty"`
	KnowledgeDirs    []string `json:"knowledgeDirs,omitempty"`
	HugoBin          string   `json:"hugoBin,omitempty"`          // hugo 可执行文件（blog new 用，缺省 PATH 上的 hugo）
	HugoSite         string   `json:"hugoSite,omitempty"`         // Hugo 站点目录（相对 blogRepo，缺省 pandawo）
	ProjectGitignore *bool    `json:"projectGitignore,omitempty"` // 是否在项目仓维护知识目录 gitignore；缺省 true
	DefaultMode      string   `json:"defaultMode,omitempty"`      // 新项目接入的存储模式：link（缺省）或 copy
	InjectFile       string   `json:"injectFile,omitempty"`       // wiki inject 的目标指令文件（不同 agent 工具的用户级指令文件路径不同）
}

func ConfigPath(root string) string { return filepath.Join(root, "config.json") }

// LoadWikiConfig 读取根目录的 config.json，缺失或损坏时返回零值配置。
func LoadWikiConfig(root string) *wikiConfig {
	cfg := &wikiConfig{}
	data, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, cfg)
	return cfg
}

func (c *wikiConfig) save(root string) error {
	return os.WriteFile(ConfigPath(root), cli.JSONIndent(c), 0o644)
}

// KnowledgeDirs 返回生效的知识目录类型名列表。
func KnowledgeDirs() []string {
	if v := os.Getenv("WIKI_KNOWLEDGE_DIRS"); v != "" {
		return cli.SplitCSV(v)
	}
	if cfg := LoadWikiConfig(WikiRoot()); len(cfg.KnowledgeDirs) > 0 {
		return cfg.KnowledgeDirs
	}
	return []string{"wiki", "issues"}
}

// BlogRepo 博客 git 仓库根：开源工具无内置默认值，须通过 config.json 或环境变量配置。
func BlogRepo() string {
	if v := os.Getenv("WIKI_BLOG_REPO"); v != "" {
		return v
	}
	if cfg := LoadWikiConfig(WikiRoot()); cfg.BlogRepo != "" {
		return cfg.BlogRepo
	}
	return ""
}

// BlogPostsDir 博客文章目录。
func BlogPostsDir() string {
	if v := os.Getenv("WIKI_BLOG_POSTS"); v != "" {
		return v
	}
	if cfg := LoadWikiConfig(WikiRoot()); cfg.BlogPosts != "" {
		return cfg.BlogPosts
	}
	return filepath.Join(BlogRepo(), postsRelDir)
}

// RequireBlogRepo 供博客命令统一校验并给出配置指引。
func RequireBlogRepo() (string, error) {
	repo := BlogRepo()
	if repo == "" {
		return "", errors.New("未配置博客仓库：执行 wiki config set blogRepo <你的 Hugo 仓库绝对路径>（或设 WIKI_BLOG_REPO 环境变量）")
	}
	return repo, nil
}

// HugoBin 返回 hugo 可执行文件（blog new 按主题 archetype 生成模板用）。
func HugoBin() string {
	if v := os.Getenv("WIKI_HUGO_BIN"); v != "" {
		return v
	}
	if cfg := LoadWikiConfig(WikiRoot()); cfg.HugoBin != "" {
		return cfg.HugoBin
	}
	return "hugo"
}

// ProjectGitignore 是否在接入/同步时维护项目仓 .gitignore（创建或追加知识目录条目）。
// 缺省 true：方案 C 下个人知识正文在知识库，项目仓只留窗口链接，不应被 git add。
// 优先级：环境变量 WIKI_PROJECT_GITIGNORE > config.json > 默认 true。
func ProjectGitignore(root string) bool {
	if v, ok := os.LookupEnv("WIKI_PROJECT_GITIGNORE"); ok && strings.TrimSpace(v) != "" {
		if b, err := cli.ParseBool(v); err == nil {
			return b
		}
	}
	if root == "" {
		root = WikiRoot()
	}
	if cfg := LoadWikiConfig(root); cfg.ProjectGitignore != nil {
		return *cfg.ProjectGitignore
	}
	return true
}

// DefaultMode 返回新项目接入的默认存储模式。
// 优先级：环境变量 WIKI_DEFAULT_MODE > config.json defaultMode > link。
// 合法值由调用方（registry）校验，这里原样返回。
func DefaultMode() string {
	if v := os.Getenv("WIKI_DEFAULT_MODE"); v != "" {
		return v
	}
	if cfg := LoadWikiConfig(WikiRoot()); cfg.DefaultMode != "" {
		return cfg.DefaultMode
	}
	return "link"
}

// InjectFile 返回 wiki inject 的目标指令文件（不同 agent 工具的用户级
// 指令文件路径不同，如 ~/.qwen/QWEN.md、~/.claude/CLAUDE.md）。
// 优先级：环境变量 WIKI_INJECT_FILE > config.json injectFile > ~/.qwen/QWEN.md。
func InjectFile() (string, error) {
	if v := os.Getenv("WIKI_INJECT_FILE"); v != "" {
		return v, nil
	}
	if cfg := LoadWikiConfig(WikiRoot()); cfg.InjectFile != "" {
		return cfg.InjectFile, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".qwen", "QWEN.md"), nil
}

// HugoSiteDir 返回 Hugo 站点目录（hugo new 的执行目录）。
func HugoSiteDir() string {
	if v := os.Getenv("WIKI_HUGO_SITE"); v != "" {
		return v
	}
	site := "pandawo"
	if cfg := LoadWikiConfig(WikiRoot()); cfg.HugoSite != "" {
		site = cfg.HugoSite
	}
	if filepath.IsAbs(site) {
		return site
	}
	return filepath.Join(BlogRepo(), site)
}

// CmdConfig 无参打印生效配置；`set <key> <value>` 写入 config.json。
func CmdConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := WikiRoot()
	switch fs.NArg() {
	case 0:
		injectFile, injectErr := InjectFile()
		if injectErr != nil {
			injectFile = "(无法解析 home 目录)"
		}
		fmt.Printf("wikiRoot:          %s\nknowledgeDirs:     %s\nblogRepo:          %s\nblogPosts:         %s\nhugoBin:           %s\nhugoSite:          %s\nprojectGitignore:  %v\ndefaultMode:       %s\ninjectFile:        %s\n",
			root, strings.Join(KnowledgeDirs(), ", "), BlogRepo(), BlogPostsDir(), HugoBin(), HugoSiteDir(), ProjectGitignore(root), DefaultMode(), injectFile)
		return nil
	case 3:
		if fs.Arg(0) != "set" {
			return errors.New("用法: wiki config set <blogRepo|blogPosts|knowledgeDirs|hugoBin|hugoSite|projectGitignore|defaultMode|injectFile> <值>")
		}
		key, val := fs.Arg(1), fs.Arg(2)
		cfg := LoadWikiConfig(root)
		switch key {
		case "blogRepo", "blogPosts":
			abs, err := filepath.Abs(val)
			if err != nil {
				return err
			}
			if key == "blogRepo" {
				cfg.BlogRepo = abs
			} else {
				cfg.BlogPosts = abs
			}
		case "hugoBin", "hugoSite":
			if key == "hugoBin" {
				cfg.HugoBin = val
			} else {
				cfg.HugoSite = val
			}
		case "knowledgeDirs":
			dirs := cli.SplitCSV(val)
			if len(dirs) == 0 {
				return errors.New("knowledgeDirs 至少一个目录名（逗号分隔，如 wiki,issues）")
			}
			cfg.KnowledgeDirs = dirs
		case "projectGitignore":
			b, err := cli.ParseBool(val)
			if err != nil {
				return err
			}
			cfg.ProjectGitignore = &b
		case "defaultMode":
			if val != "link" && val != "copy" {
				return errors.New("defaultMode 只能是 link（窗口链接）或 copy（项目侧真目录 + 知识库增量拷贝）")
			}
			cfg.DefaultMode = val
		case "injectFile":
			abs, err := filepath.Abs(val)
			if err != nil {
				return err
			}
			cfg.InjectFile = abs
		default:
			return fmt.Errorf("未知配置项 %q（可用: blogRepo, blogPosts, knowledgeDirs, hugoBin, hugoSite, projectGitignore, defaultMode, injectFile）", key)
		}
		if err := cfg.save(root); err != nil {
			return err
		}
		fmt.Printf("已设置 %s（写入 %s）\n", key, ConfigPath(root))
		return nil
	default:
		return errors.New("用法: wiki config [查看] 或 wiki config set <blogRepo|blogPosts|knowledgeDirs|hugoBin|hugoSite|projectGitignore|defaultMode|injectFile> <值>")
	}
}
