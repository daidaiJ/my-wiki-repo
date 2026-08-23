package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// 全局查看命令统一作用于 wiki 的统一视图 <wikiRoot>/projects/，
// 路径规格为 `项目名/链接名/相对路径`（与 grep 输出、链接布局一致）。

func projectsRoot() string { return filepath.Join(wikiRoot(), "projects") }

// resolveSpec 把路径规格解析到 projects 下的绝对路径，拒绝越界。
func resolveSpec(spec string) (string, error) {
	base := projectsRoot()
	p := filepath.Clean(filepath.Join(base, filepath.FromSlash(spec)))
	if !underOrEqual(p, base) {
		return "", fmt.Errorf("路径 %q 越出知识库范围", spec)
	}
	return p, nil
}

func cmdLS(args []string) error {
	if len(args) > 1 {
		return errors.New("用法: wiki ls [项目[/链接/子路径]]")
	}
	root := wikiRoot()
	if len(args) == 0 {
		reg, err := loadRegistry(root)
		if err != nil {
			return err
		}
		if len(reg.Projects) == 0 {
			fmt.Println("知识库为空。agent 执行 wiki init --paths <目录> 接入项目。")
			return nil
		}
		for _, p := range reg.Projects {
			intro := p.Intro
			if intro == "" {
				intro = "（介绍待补充）"
			}
			fmt.Printf("%s/\t%s\t%s\n", p.Name, strings.Join(p.Paths, ","), intro)
		}
		return nil
	}
	dir, err := resolveSpec(args[0])
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("路径不存在: %s（先 wiki ls 看已接入项目）", args[0])
		}
		return err
	}
	prefix := strings.TrimSuffix(filepath.ToSlash(args[0]), "/")
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		if prefix == "" {
			fmt.Println(name)
		} else {
			fmt.Printf("%s/%s\n", prefix, name)
		}
	}
	return nil
}

func cmdCat(args []string) error {
	if len(args) != 1 {
		return errors.New("用法: wiki cat <项目/链接/文件路径>")
	}
	path, err := resolveSpec(args[0])
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(os.Stdout, f)
	return err
}

func cmdTree(args []string) error {
	fs := flag.NewFlagSet("tree", flag.ContinueOnError)
	depth := fs.Int("depth", 3, "最大深度")
	if err := parseWithPositionals(fs, args); err != nil {
		return err
	}
	spec := ""
	if fs.NArg() == 1 {
		spec = fs.Arg(0)
	} else if fs.NArg() > 1 {
		return errors.New("用法: wiki tree [项目] [--depth N]")
	}
	base, err := resolveSpec(spec)
	if err != nil {
		return err
	}
	if spec == "" {
		fmt.Println("projects/")
	} else {
		fmt.Println(strings.TrimSuffix(filepath.ToSlash(spec), "/") + "/")
	}
	walkTree(base, "", *depth, map[string]bool{})
	return nil
}

var skipDirNames = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true, ".idea": true,
}

// walkTree 渲染目录树；visited 防御链接环路（解析后的绝对路径去重）。
func walkTree(dir, indent string, depth int, visited map[string]bool) {
	if depth <= 0 {
		return
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || visited[strings.ToLower(resolved)] {
		return
	}
	visited[strings.ToLower(resolved)] = true
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for i, e := range entries {
		conn, next := "├── ", indent+"│   "
		if i == len(entries)-1 {
			conn, next = "└── ", indent+"    "
		}
		marker := ""
		if e.Type()&os.ModeSymlink != 0 {
			marker = " -> " + symlinkTargetHint(filepath.Join(dir, e.Name()))
		}
		isDir := e.IsDir() || (e.Type()&os.ModeSymlink != 0 && dirExists(filepath.Join(dir, e.Name())))
		fmt.Printf("%s%s%s%s\n", indent, conn, e.Name(), ternary(isDir, "/", "")+marker)
		if isDir && !skipDirNames[e.Name()] {
			walkTree(filepath.Join(dir, e.Name()), next, depth-1, visited)
		}
	}
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func symlinkTargetHint(link string) string {
	res, err := filepath.EvalSymlinks(link)
	if err != nil {
		return "?"
	}
	return res
}

// walkFiles 递归遍历文件；与 filepath.Walk 的区别：跟进指向目录的符号链接/junction
// （统一视图的核心能力），并用 resolved 路径去重防环路。
func walkFiles(dir string, visited map[string]bool, fn func(path string) error) error {
	if visited == nil {
		visited = map[string]bool{}
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		key := strings.ToLower(resolved)
		if visited[key] {
			return nil
		}
		visited[key] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // 单点失败不中断全局搜索
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.Type()&os.ModeSymlink != 0 {
			if dirExists(p) {
				if err := walkFiles(p, visited, fn); err != nil {
					return err
				}
			}
			continue // 文件链接跳过
		}
		if e.IsDir() {
			if skipDirNames[e.Name()] {
				continue
			}
			if err := walkFiles(p, visited, fn); err != nil {
				return err
			}
			continue
		}
		if err := fn(p); err != nil {
			return err
		}
	}
	return nil
}

const (
	grepMaxMatches = 200
	grepMaxFile    = 1 << 20 // 超过 1MB 的文件跳过
)

func cmdGrep(args []string) error {
	fs := flag.NewFlagSet("grep", flag.ContinueOnError)
	fixed := fs.Bool("fixed", false, "按字面量而非正则匹配")
	if err := parseWithPositionals(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 1 || fs.NArg() > 2 {
		return errors.New("用法: wiki grep <模式> [项目[/链接/子路径]]")
	}
	pattern := fs.Arg(0)
	spec := ""
	if fs.NArg() == 2 {
		spec = fs.Arg(1)
	}
	expr := pattern
	if *fixed {
		expr = regexp.QuoteMeta(pattern)
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return fmt.Errorf("正则无效（--fixed 可按字面量匹配）: %w", err)
	}
	base, err := resolveSpec(spec)
	if err != nil {
		return err
	}
	matches := 0
	err = walkFiles(base, nil, func(path string) error {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() > grepMaxFile {
			return nil
		}
		n, herr := grepFile(path, re, base)
		if herr != nil {
			return nil
		}
		matches += n
		if matches >= grepMaxMatches {
			return errStopWalk
		}
		return nil
	})
	if errors.Is(err, errStopWalk) || err == nil {
		if matches >= grepMaxMatches {
			fmt.Printf("…（已达 %d 条上限，收窄路径或模式）\n", grepMaxMatches)
		}
		if matches == 0 {
			fmt.Println("无匹配")
		}
		return nil
	}
	return err
}

var errStopWalk = errors.New("stop walk")

// grepFile 逐行匹配单个文件，输出 `项目/链接/文件:行号: 内容`（可直接喂给 wiki cat）。
func grepFile(path string, re *regexp.Regexp, base string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return 0, err
	}
	spec := filepath.ToSlash(rel)
	br := bufio.NewReader(f)
	// 二进制文件（含 NUL）跳过
	probe, _ := br.Peek(512)
	for _, b := range probe {
		if b == 0 {
			return 0, nil
		}
	}
	n, lineNo := 0, 0
	for {
		line, rerr := br.ReadString('\n')
		if line != "" {
			lineNo++
			if re.MatchString(strings.TrimRight(line, "\r\n")) {
				fmt.Printf("%s:%d: %s\n", spec, lineNo, strings.TrimRight(line, "\r\n"))
				n++
			}
		}
		if rerr != nil {
			break
		}
	}
	return n, nil
}
