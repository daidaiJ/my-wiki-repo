package registry

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
	"github.com/daidaiJ/my-wiki-repo/internal/config"
)

const (
	migratingSuffix = ".__migrating__"
	migratedSuffix  = ".__migrated__"
	gitignoreBanner = "# wiki knowledge dirs (managed by wiki CLI)"
)

// isLink 判断路径是否为符号链接或 Windows 目录 junction（不跟随）。
func isLink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return true
	}
	return isWindowsReparsePoint(path)
}

func isRealDir(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || isLink(path) {
		return false
	}
	return fi.IsDir()
}

func dirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) == 0
}

func sameResolved(a, b string) bool {
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		ra = a
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		rb = b
	}
	absA, err := filepath.Abs(ra)
	if err != nil {
		return false
	}
	absB, err := filepath.Abs(rb)
	if err != nil {
		return false
	}
	return cli.SamePath(absA, absB)
}

// invertedHealthy：知识库侧是真目录，项目侧是指向它的窗口链接。
func invertedHealthy(store, projPath string) bool {
	return isRealDir(store) && isLink(projPath) && sameResolved(projPath, store)
}

// legacyForward：旧模型——知识库侧是指向项目真目录的正向链接。
func legacyForward(store, projPath string) bool {
	return isLink(store) && isRealDir(projPath) && sameResolved(store, projPath)
}

// ensureInverted 把一条知识路径收敛到方案 C：先把正文迁进知识库，再把项目侧替换成窗口链接。
// provision 为 true 时，两侧都不存在则新建空知识库目录并建立窗口链接（供 init/prepare hook 使用）。
// 已健康则幂等跳过。返回是否发生了迁移或链接修复。
func ensureInverted(store, projPath string, provision bool) (bool, error) {
	if invertedHealthy(store, projPath) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
		return false, err
	}

	storeExists := cli.Lexists(store)
	projExists := cli.Lexists(projPath)
	storeLink := storeExists && isLink(store)
	projLink := projExists && isLink(projPath)
	storeReal := storeExists && isRealDir(store)
	projReal := projExists && isRealDir(projPath)

	switch {
	case storeReal && (!projExists || projLink):
		if projLink {
			if err := os.Remove(projPath); err != nil {
				return false, fmt.Errorf("移除损坏的窗口链接 %s 失败: %w", projPath, err)
			}
		}
		if err := createLink(store, projPath); err != nil {
			return false, err
		}
		return true, nil

	case projReal && (!storeExists || storeLink || (storeReal && dirEmpty(store))):
		if err := migrateInvert(store, projPath); err != nil {
			return false, err
		}
		return true, nil

	case storeReal && projReal:
		switch {
		case dirEmpty(projPath):
			if err := os.Remove(projPath); err != nil {
				return false, err
			}
			if err := createLink(store, projPath); err != nil {
				return false, err
			}
			return true, nil
		case dirEmpty(store):
			if err := migrateInvert(store, projPath); err != nil {
				return false, err
			}
			return true, nil
		default:
			return false, fmt.Errorf("项目路径 %s 与知识库 %s 都是真实目录且均有内容，请先合并后再同步", projPath, store)
		}

	case projLink && !storeExists:
		return false, fmt.Errorf("窗口链接 %s 的目标知识库不存在", projPath)

	case storeLink && !projExists:
		return false, fmt.Errorf("正向链接 %s 的目标 %s 已不存在", store, projPath)

	case !storeExists && !projExists:
		if !provision {
			return false, fmt.Errorf("接入路径不存在: %s", projPath)
		}
		if err := os.MkdirAll(store, 0o755); err != nil {
			return false, err
		}
		if err := createLink(store, projPath); err != nil {
			return false, err
		}
		return true, nil

	default:
		return false, fmt.Errorf("无法识别 %s / %s 的目录布局（可能是损坏的链接）", projPath, store)
	}
}

// migrateInvert 先把项目侧真目录完整拷进知识库，校验后再把项目侧替换成窗口链接。
func migrateInvert(store, projPath string) error {
	if !isRealDir(projPath) {
		return fmt.Errorf("迁移源不是真实目录: %s", projPath)
	}
	if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
		return err
	}
	staging := store + migratingSuffix
	_ = os.RemoveAll(staging)
	if err := copyTree(projPath, staging); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("迁移拷贝失败: %w", err)
	}

	if cli.Lexists(store) {
		switch {
		case isLink(store):
			if err := os.Remove(store); err != nil {
				_ = os.RemoveAll(staging)
				return fmt.Errorf("移除旧正向链接 %s 失败: %w", store, err)
			}
		case isRealDir(store) && dirEmpty(store):
			if err := os.Remove(store); err != nil {
				_ = os.RemoveAll(staging)
				return err
			}
		default:
			_ = os.RemoveAll(staging)
			return fmt.Errorf("知识库路径 %s 已是真实目录且非空，拒绝覆盖", store)
		}
	}
	if err := os.Rename(staging, store); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("迁入知识库 %s 失败: %w", store, err)
	}

	backup := projPath + migratedSuffix
	_ = os.RemoveAll(backup)
	if err := os.Rename(projPath, backup); err != nil {
		return fmt.Errorf("挪开项目目录 %s 失败（知识库已迁入 %s）: %w", projPath, store, err)
	}
	if err := createLink(store, projPath); err != nil {
		if !cli.Lexists(projPath) && cli.Lexists(backup) {
			_ = os.Rename(backup, projPath)
		}
		return fmt.Errorf("建立窗口链接失败: %w", err)
	}
	if !invertedHealthy(store, projPath) {
		return fmt.Errorf("迁移后校验失败: %s 未正确指向 %s", projPath, store)
	}
	if err := os.RemoveAll(backup); err != nil {
		fmt.Fprintf(os.Stderr, "wiki: 迁移备份 %s 未能删除，请确认知识库完好后手动删除\n", backup)
	}
	return nil
}

func ensureForwardLink(store, target string) (bool, error) {
	if isLink(store) {
		if resolved, err := filepath.EvalSymlinks(store); err == nil && cli.SamePath(resolved, target) {
			return false, nil
		}
	}
	if err := createLink(target, store); err != nil {
		return false, err
	}
	return true, nil
}

// copyTree 把 src 目录树拷到 dst（真实文件）。不跟随符号链接/junction，跳过 .git 与迁移残留。
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if name == ".git" || strings.HasSuffix(name, migratingSuffix) || strings.HasSuffix(name, migratedSuffix) {
			if d.IsDir() || isLink(path) {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if isLink(path) {
			if d.IsDir() || (d.Type()&os.ModeSymlink != 0 && cli.DirExists(path)) {
				return fs.SkipDir
			}
			return nil
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		return copyFile(path, out)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	perm := info.Mode().Perm()
	if perm == 0 {
		perm = 0o644
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func gitignoreEntry(rel string) string {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || rel == "." {
		return ""
	}
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}
	return rel
}

func gitignoreHas(content, entry string) bool {
	entry = strings.TrimSuffix(entry, "/")
	want := map[string]bool{
		entry: true, entry + "/": true,
		strings.TrimPrefix(entry, "/"):         true,
		strings.TrimPrefix(entry, "/") + "/": true,
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if want[line] || want[strings.TrimSuffix(line, "/")] {
			return true
		}
	}
	return false
}

// ensureProjectGitignore 在项目仓创建或追加知识目录 ignore 条目。
// 返回动作 created / appended / ""（未写盘）。调用方先用 gitignoreEnabled 判断开关。
func ensureProjectGitignore(projectRoot string, relPaths []string) (string, error) {
	var entries []string
	for _, rel := range relPaths {
		e := gitignoreEntry(rel)
		if e != "" {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		return "", nil
	}
	path := filepath.Join(projectRoot, ".gitignore")
	data, err := os.ReadFile(path)
	created := false
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		created = true
		data = nil
	}
	content := string(data)
	var missing []string
	for _, e := range entries {
		if !gitignoreHas(content, e) {
			missing = append(missing, e)
		}
	}
	if len(missing) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString(content)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		b.WriteByte('\n')
	}
	if !strings.Contains(content, gitignoreBanner) {
		if len(content) > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(gitignoreBanner + "\n")
	}
	for _, e := range missing {
		b.WriteString(e + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	if created {
		return "created", nil
	}
	return "appended", nil
}

func gitignoreEnabled(root string) bool {
	return config.ProjectGitignore(root)
}
