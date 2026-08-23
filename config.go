package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// wikiConfig 是 wiki 根目录下的 config.json，让 Hugo 仓库/文章目录可配置。
// 优先级：环境变量 > config.json > 内置默认值。
type wikiConfig struct {
	BlogRepo  string `json:"blogRepo,omitempty"`  // Hugo 博客 git 仓库根
	BlogPosts string `json:"blogPosts,omitempty"` // 文章目录（缺省 <blogRepo>\pandawo\content\post）
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
		cfg := loadWikiConfig(root)
		fmt.Printf("wikiRoot:   %s\nblogRepo:   %s\nblogPosts:  %s\nrecord:     %s\n",
			root, blogRepo(), blogPostsDir(), recordPath(root))
		_ = cfg
		return nil
	case 3:
		if fs.Arg(0) != "set" {
			return errors.New("用法: wiki config [查看] 或 wiki config set <blogRepo|blogPosts> <路径>")
		}
		key, val := fs.Arg(1), fs.Arg(2)
		cfg := loadWikiConfig(root)
		abs, err := filepath.Abs(val)
		if err != nil {
			return err
		}
		switch key {
		case "blogRepo":
			cfg.BlogRepo = abs
		case "blogPosts":
			cfg.BlogPosts = abs
		default:
			return fmt.Errorf("未知配置项 %q（可用: blogRepo, blogPosts）", key)
		}
		if err := cfg.save(root); err != nil {
			return err
		}
		fmt.Printf("已设置 %s = %s（写入 %s）\n", key, abs, configPath(root))
		return nil
	default:
		return errors.New("用法: wiki config [查看] 或 wiki config set <blogRepo|blogPosts> <路径>")
	}
}
