package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// wikiConfig 是 wiki 根目录下的 config.json。开源可移植：fork 者在此配置自己的环境。
// 优先级：环境变量 > config.json > 内置默认值。
type wikiConfig struct {
	BlogRepo      string   `json:"blogRepo,omitempty"`      // Hugo 博客 git 仓库根（必配，无默认）
	BlogPosts     string   `json:"blogPosts,omitempty"`     // 文章目录（缺省 <blogRepo>\pandawo\content\post）
	KnowledgeDirs []string `json:"knowledgeDirs,omitempty"` // 知识目录类型名（缺省 wiki、issues）
}

// knowledgeDirs 返回生效的知识目录类型名列表。
func knowledgeDirs() []string {
	if v := os.Getenv("WIKI_KNOWLEDGE_DIRS"); v != "" {
		return splitCSV(v)
	}
	if cfg := loadWikiConfig(wikiRoot()); len(cfg.KnowledgeDirs) > 0 {
		return cfg.KnowledgeDirs
	}
	return []string{"wiki", "issues"}
}

func configPath(root string) string { return filepath.Join(root, "config.json") }

func loadWikiConfig(root string) *wikiConfig {
	cfg := &wikiConfig{}
	data, err := os.ReadFile(configPath(root))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, cfg)
	return cfg
}

func (c *wikiConfig) save(root string) error {
	return os.WriteFile(configPath(root), mustJSONIndent(c), 0o644)
}

func mustJSONIndent(v any) []byte {
	data, _ := json.MarshalIndent(v, "", "  ")
	return data
}

// cmdConfig 无参打印生效配置；`set <key> <value>` 写入 config.json。
func cmdConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	root := wikiRoot()
	switch fs.NArg() {
	case 0:
		fmt.Printf("wikiRoot:       %s\nknowledgeDirs:  %s\nblogRepo:       %s\nblogPosts:      %s\nrecord:         %s\n",
			root, strings.Join(knowledgeDirs(), ", "), blogRepo(), blogPostsDir(), recordPath(root))
		return nil
	case 3:
		if fs.Arg(0) != "set" {
			return errors.New("用法: wiki config set <blogRepo|blogPosts|knowledgeDirs> <值>")
		}
		key, val := fs.Arg(1), fs.Arg(2)
		cfg := loadWikiConfig(root)
		switch key {
		case "blogRepo":
			abs, err := filepath.Abs(val)
			if err != nil {
				return err
			}
			cfg.BlogRepo = abs
		case "blogPosts":
			abs, err := filepath.Abs(val)
			if err != nil {
				return err
			}
			cfg.BlogPosts = abs
		case "knowledgeDirs":
			dirs := splitCSV(val)
			if len(dirs) == 0 {
				return errors.New("knowledgeDirs 至少一个目录名（逗号分隔，如 wiki,issues）")
			}
			cfg.KnowledgeDirs = dirs
		default:
			return fmt.Errorf("未知配置项 %q（可用: blogRepo, blogPosts, knowledgeDirs）", key)
		}
		if err := cfg.save(root); err != nil {
			return err
		}
		fmt.Printf("已设置 %s（写入 %s）\n", key, configPath(root))
		return nil
	default:
		return errors.New("用法: wiki config [查看] 或 wiki config set <blogRepo|blogPosts|knowledgeDirs> <值>")
	}
}
