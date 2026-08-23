package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// postsRelDir 是文章目录相对仓库根的默认位置（Hugo 站点在 pandawo 子目录）。
const postsRelDir = `pandawo\content\post`

// blogRepo 博客 git 仓库根：开源工具无内置默认值，须通过 config.json 或环境变量配置。
func blogRepo() string {
	if v := os.Getenv("WIKI_BLOG_REPO"); v != "" {
		return v
	}
	if cfg := loadWikiConfig(wikiRoot()); cfg.BlogRepo != "" {
		return cfg.BlogRepo
	}
	return ""
}

// requireBlogRepo 供博客命令统一校验并给出配置指引。
func requireBlogRepo() (string, error) {
	repo := blogRepo()
	if repo == "" {
		return "", errors.New("未配置博客仓库：执行 wiki config set blogRepo <你的 Hugo 仓库绝对路径>（或设 WIKI_BLOG_REPO 环境变量）")
	}
	return repo, nil
}

func blogPostsDir() string {
	if v := os.Getenv("WIKI_BLOG_POSTS"); v != "" {
		return v
	}
	if cfg := loadWikiConfig(wikiRoot()); cfg.BlogPosts != "" {
		return cfg.BlogPosts
	}
	return filepath.Join(blogRepo(), postsRelDir)
}

// --- front matter 解析 ---

// parseFrontMatter 提取 `---` 围栏内的 YAML 段并解析关心的字段。
// 容忍既有文章的写法差异：`tags :`（冒号前有空格）、`categories: ["a"]` 流式列表。
func parseFrontMatter(content string) (*FrontMatter, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, errors.New("不是以 --- 开头的 front matter")
	}
	rest := normalized[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, errors.New("找不到 front matter 结束分隔符 ---")
	}
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(rest[:end]), &raw); err != nil {
		return nil, fmt.Errorf("front matter YAML 解析失败: %w", err)
	}
	get := func(k string) string {
		if v, ok := raw[k].(string); ok {
			return v
		}
		return ""
	}
	return &FrontMatter{
		Title:      get("title"),
		Slug:       get("slug"),
		Categories: strList(raw["categories"]),
		Tags:       strList(raw["tags"]),
	}, nil
}

// strList 把 YAML 值规整为字符串列表：兼容 []any、[]string、单个字符串、空值。
func strList(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(t))
		for _, s := range t {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{strings.TrimSpace(t)}
	default:
		return nil
	}
}

// --- 文章收集（blog new 的 apply 时查重用） ---

type PostInfo struct {
	File  string `json:"file"`
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

func collectPosts(postDir string) ([]PostInfo, error) {
	entries, err := os.ReadDir(postDir)
	if err != nil {
		return nil, fmt.Errorf("读取文章目录 %s 失败: %w", postDir, err)
	}
	var posts []PostInfo
	for _, en := range entries {
		if en.IsDir() || !strings.HasSuffix(en.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(postDir, en.Name()))
		if err != nil {
			return nil, err
		}
		fm, err := parseFrontMatter(string(data))
		if err != nil {
			// 单篇坏 front matter 不阻塞整体列表，标题退化为文件名
			posts = append(posts, PostInfo{File: en.Name(), Title: en.Name()})
			continue
		}
		posts = append(posts, PostInfo{File: en.Name(), Title: fm.Title, Slug: fm.Slug})
	}
	sort.Slice(posts, func(i, j int) bool { return posts[i].File < posts[j].File })
	return posts, nil
}

// --- 子命令 ---

func cmdBlog(args []string) error {
	if len(args) == 0 {
		return errors.New("用法: wiki blog <list|new|publish>")
	}
	if _, err := requireBlogRepo(); err != nil {
		return err
	}
	switch args[0] {
	case "list":
		return cmdBlogList(args[1:])
	case "new":
		return cmdBlogNew(args[1:])
	case "publish":
		return cmdBlogPublish(args[1:])
	default:
		return fmt.Errorf("未知的 blog 子命令 %q（可用: list/new/publish）", args[0])
	}
}

// cmdBlogList 只列出 categories/tags 两字段（按使用次数降序，供 agent 复用已有类别）。
// 数据来自懒维护的本地发布记录 blog.json，首次调用自动扫描 Hugo 目录引导。
func cmdBlogList(args []string) error {
	fs := flag.NewFlagSet("blog list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "输出 JSON（agent 用）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rec, err := reconcileRecord(wikiRoot(), blogPostsDir())
	if err != nil {
		return err
	}
	cats := aggregate(rec, func(e BlogRecEntry) []string { return e.Categories })
	tags := aggregate(rec, func(e BlogRecEntry) []string { return e.Tags })
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(map[string]any{
			"total":      len(rec.Posts),
			"categories": cats,
			"tags":       tags,
		})
	}
	fmt.Printf("已发布文章: %d 篇（记录 %s）\n\n", len(rec.Posts), recordPath(wikiRoot()))
	fmt.Println("categories（按使用次数降序，创建文章时优先复用已有类别）:")
	for _, c := range sortedByCount(cats) {
		fmt.Printf("  %-16s %d\n", c, cats[c])
	}
	fmt.Println("\ntags（按使用次数降序）:")
	for _, t := range sortedByCount(tags) {
		fmt.Printf("  %-16s %d\n", t, tags[t])
	}
	return nil
}

var (
	slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
)

func cmdBlogNew(args []string) error {
	fs := flag.NewFlagSet("blog new", flag.ContinueOnError)
	title := fs.String("title", "", "文章标题（必填）")
	slug := fs.String("slug", "", "URL slug，kebab-case（必填）")
	categories := fs.String("categories", "", "分类，逗号分隔（必填，优先复用已有分类，先 wiki blog list）")
	tags := fs.String("tags", "", "标签，逗号分隔")
	name := fs.String("name", "", "文件名（缺省取 slug 的 - 转 _）")
	file := fs.String("file", "", "正文来源：文件路径")
	body := fs.String("body", "", "正文来源：字符串")
	stdin := fs.Bool("stdin", false, "正文来源：标准输入")
	dryRun := fs.Bool("dry-run", false, "只打印将写入的路径和全文，不落盘")
	publish := fs.Bool("publish", false, "写完直接提交推送")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch {
	case strings.TrimSpace(*title) == "":
		return errors.New("--title 必填")
	case strings.TrimSpace(*slug) == "":
		return errors.New("--slug 必填")
	case strings.TrimSpace(*categories) == "":
		return errors.New("--categories 必填")
	}
	normSlug := strings.ToLower(strings.TrimSpace(*slug))
	if !slugRe.MatchString(normSlug) {
		return fmt.Errorf("--slug %q 不是合法的 kebab-case（小写字母数字，中划线分隔）", *slug)
	}
	fileName := strings.TrimSpace(*name)
	if fileName == "" {
		fileName = strings.ReplaceAll(normSlug, "-", "_")
	}
	if !nameRe.MatchString(fileName) {
		return fmt.Errorf("--name %q 含非法字符（允许字母数字 _ -）", fileName)
	}
	cats := splitCSV(*categories)
	if len(cats) == 0 {
		return errors.New("--categories 解析后为空")
	}
	tagList := splitCSV(*tags)

	sources := 0
	for _, ok := range []bool{*file != "", *body != "", *stdin} {
		if ok {
			sources++
		}
	}
	if sources != 1 {
		return errors.New("正文来源必须且只能指定一个：--file / --body / --stdin")
	}
	var bodyText string
	switch {
	case *file != "":
		data, err := os.ReadFile(*file)
		if err != nil {
			return fmt.Errorf("读取正文文件失败: %w", err)
		}
		bodyText = string(data)
	case *stdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		bodyText = string(data)
	default:
		bodyText = *body
	}
	bodyText = strings.ReplaceAll(bodyText, "\r\n", "\n")
	if bodyText != "" && !strings.HasSuffix(bodyText, "\n") {
		bodyText += "\n"
	}

	postDir := blogPostsDir()
	posts, err := collectPosts(postDir)
	if err != nil {
		return err
	}
	for _, p := range posts {
		if p.Slug == normSlug {
			return fmt.Errorf("slug %q 已被 %s 使用，请换一个", normSlug, p.File)
		}
		if p.File == fileName+".md" {
			return fmt.Errorf("文件 %s.md 已存在", fileName)
		}
	}

	meta := NewPostMeta{Title: strings.TrimSpace(*title), Slug: normSlug, Categories: cats, Tags: tagList, Now: nowFn()}
	full := generateFrontMatter(meta) + "\n" + bodyText
	target := filepath.Join(postDir, fileName+".md")

	if *dryRun {
		fmt.Printf("[dry-run] 将写入: %s\n\n%s", target, full)
		return nil
	}
	if err := os.WriteFile(target, []byte(full), 0o644); err != nil {
		return err
	}
	fmt.Printf("已创建 %s\n", target)
	if *publish {
		return blogPublish(fileName)
	}
	fmt.Printf("下一步: wiki blog publish %s\n", fileName)
	return nil
}

func splitCSV(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" && !seen[part] {
			seen[part] = true
			out = append(out, part)
		}
	}
	return out
}

func cmdBlogPublish(args []string) error {
	fs := flag.NewFlagSet("blog publish", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("用法: wiki blog publish <文件名（不带 .md）>")
	}
	return blogPublish(fs.Arg(0))
}

func blogPublish(fileName string) error {
	if !nameRe.MatchString(fileName) {
		return fmt.Errorf("文件名 %q 非法", fileName)
	}
	repo := blogRepo()
	postFile := filepath.Join(blogPostsDir(), fileName+".md")
	if _, err := os.Stat(postFile); err != nil {
		return fmt.Errorf("文章不存在: %s", postFile)
	}
	rel, err := filepath.Rel(repo, postFile)
	if err != nil {
		return err
	}

	run := func(step string, gitArgs ...string) (string, bool, error) {
		cmd := exec.Command("git", gitArgs...)
		cmd.Dir = repo
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err := cmd.Run()
		fmt.Printf("[git %s] %s", step, buf.String())
		return buf.String(), err == nil, err
	}

	if out, _, err := run("add", "add", "--", rel); err != nil {
		return fmt.Errorf("git add 失败:\n%s%v", out, err)
	}
	out, ok, err := run("commit", "commit", "-m", "添加文章 "+fileName+".md", "--", rel)
	if err != nil && !strings.Contains(out, "nothing to commit") && !strings.Contains(out, "no changes added") {
		return fmt.Errorf("git commit 失败:\n%s%v", out, err)
	}
	_ = ok
	if out, _, err := run("push", "push"); err != nil {
		return fmt.Errorf(
			"git push 失败（不重试，原始输出透传如下）:\n%s%v\n\n"+
				"GitHub 网络问题请用户手动处理：稍后在 %s 执行 git push 即可，文章已本地提交。",
			out, err, repo)
	}
	fmt.Printf("发布完成：%s 已推送，GitHub Actions 将自动构建部署。\n", fileName+".md")
	// 发布成功 → 更新本地四字段记录（lazy 维护）
	if entry, err := parsePostFile(postFile); err == nil {
		root := wikiRoot()
		rec, lerr := loadBlogRecord(root)
		if lerr == nil {
			rec.upsert(entry)
			if serr := rec.save(root); serr == nil {
				fmt.Printf("发布记录已更新: %s（%d 篇）\n", recordPath(root), len(rec.Posts))
			}
		}
	}
	return nil
}

// nowFn 抽出来便于测试注入固定时间。
var nowFn = time.Now
