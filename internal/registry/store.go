package registry

import (
	"bytes"
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
	conflictSuffix  = ".__from_project__"
	gitignoreBanner = "# wiki knowledge dirs (managed by wiki CLI)"
)

// renameDir 默认为 os.Rename。测试可替换，模拟 Windows 占用导致整目录改名失败。
var renameDir = os.Rename

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
			// 中断恢复：知识库已有正文时绝不覆盖；只把项目侧多出来的文件补进去，
			// 同路径内容不同则另存，再尝试把项目侧换成窗口。
			if err := mergeProjectIntoStore(store, projPath); err != nil {
				return false, fmt.Errorf("合并项目侧到知识库失败（未覆盖已有文件）: %w", err)
			}
			if err := swapProjectToWindow(store, projPath); err != nil {
				return false, err
			}
			return true, nil
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
	if err := verifyStoreCovers(staging, projPath); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("迁移拷贝校验失败（项目侧未动）: %w", err)
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
	if err := renameDir(staging, store); err != nil {
		// 留下 staging，项目侧未动；下次重试会重建 staging
		return fmt.Errorf("迁入知识库 %s 失败（拷贝保留在 %s，项目侧未动）: %w", store, staging, err)
	}
	return swapProjectToWindow(store, projPath)
}

// swapProjectToWindow 在知识库真目录已就位后，把项目侧真目录挪开并换成窗口。
// 整目录 Rename 失败时改逐项搬走（Windows 占用常见）；失败不删知识库、不覆盖备份。
func swapProjectToWindow(store, projPath string) error {
	if invertedHealthy(store, projPath) {
		return nil
	}
	if !isRealDir(store) {
		return fmt.Errorf("知识库 %s 不是真实目录，拒绝把项目侧换成窗口", store)
	}
	if !isRealDir(projPath) {
		if cli.Lexists(projPath) {
			return fmt.Errorf("项目路径 %s 不是真实目录，无法挪开", projPath)
		}
		return createLink(store, projPath)
	}
	backup := uniqueSidePath(projPath, migratedSuffix)
	if err := relocateDir(projPath, backup); err != nil {
		return fmt.Errorf("挪开项目目录 %s 失败（知识库已有正文 %s，未覆盖）: %w", projPath, store, err)
	}
	if err := createLink(store, projPath); err != nil {
		if !cli.Lexists(projPath) {
			if rerr := relocateDir(backup, projPath); rerr != nil {
				return fmt.Errorf("建立窗口链接失败: %v；还原项目目录也失败（正文在 %s 与 %s）: %w", err, store, backup, rerr)
			}
		}
		return fmt.Errorf("建立窗口链接失败: %w", err)
	}
	if !invertedHealthy(store, projPath) {
		return fmt.Errorf("迁移后校验失败: %s 未正确指向 %s", projPath, store)
	}
	if err := verifyStoreCovers(store, backup); err != nil {
		fmt.Fprintf(os.Stderr, "wiki: 知识库未完整覆盖备份 %s（%v），保留备份请勿删除\n", backup, err)
		return nil
	}
	if err := os.RemoveAll(backup); err != nil {
		fmt.Fprintf(os.Stderr, "wiki: 迁移备份 %s 未能删除，知识库已校验完好，可手动删除\n", backup)
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

func skipMigrateName(name string) bool {
	return name == ".git" ||
		strings.Contains(name, migratingSuffix) ||
		strings.Contains(name, migratedSuffix) ||
		strings.Contains(name, conflictSuffix)
}

func uniqueSidePath(path, suffix string) string {
	cand := path + suffix
	for n := 0; cli.Lexists(cand); n++ {
		cand = fmt.Sprintf("%s%s.%d", path, suffix, n+1)
	}
	return cand
}

func sameFileContent(a, b string) (bool, error) {
	fa, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if fa.IsDir() || fb.IsDir() || fa.Size() != fb.Size() {
		return false, nil
	}
	da, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	db, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(da, db), nil
}

func walkRegularFiles(root string, fn func(rel, abs string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if skipMigrateName(name) {
			if d.IsDir() || isLink(path) {
				return fs.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		if isLink(path) {
			if d.IsDir() || (d.Type()&os.ModeSymlink != 0 && cli.DirExists(path)) {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return fn(rel, path)
	})
}

// verifyStoreCovers 确认 dst 含有 src 的每一份常规文件且字节一致（允许 dst 有多余文件）。
func verifyStoreCovers(dst, src string) error {
	return walkRegularFiles(src, func(rel, abs string) error {
		got := filepath.Join(dst, rel)
		same, err := sameFileContent(abs, got)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if !same {
			return fmt.Errorf("%s 与知识库内容不一致或缺失", rel)
		}
		return nil
	})
}

// mergeProjectIntoStore 把项目侧有、知识库没有的文件补进去；同路径内容不同则另存，绝不覆盖知识库。
func mergeProjectIntoStore(store, proj string) error {
	return walkRegularFiles(proj, func(rel, abs string) error {
		dest := filepath.Join(store, rel)
		if !cli.Lexists(dest) {
			return copyFile(abs, dest)
		}
		if isLink(dest) || isRealDir(dest) {
			return stashConflictCopy(store, rel, abs)
		}
		same, err := sameFileContent(abs, dest)
		if err != nil {
			return err
		}
		if same {
			return nil
		}
		return stashConflictCopy(store, rel, abs)
	})
}

func stashConflictCopy(store, rel, src string) error {
	base := filepath.Join(store, rel+conflictSuffix)
	cand := base
	for n := 0; cli.Lexists(cand); n++ {
		if same, err := sameFileContent(src, cand); err == nil && same {
			return nil
		}
		cand = fmt.Sprintf("%s.%d", base, n+1)
	}
	if err := copyFile(src, cand); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wiki: %s 知识库已有不同内容，项目侧副本未覆盖，另存为 %s\n", rel, cand)
	return nil
}

// relocateDir 把 src 挪到 dst（dst 必须尚不存在）。Rename 失败则逐项拷走再删源，避免占用导致整树卡死。
func relocateDir(src, dst string) error {
	if !isRealDir(src) {
		return fmt.Errorf("不是真实目录: %s", src)
	}
	if cli.Lexists(dst) {
		return fmt.Errorf("目标已存在，拒绝覆盖: %s", dst)
	}
	if err := renameDir(src, dst); err == nil {
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	if err := moveDirContents(src, dst); err != nil {
		return err
	}
	if !dirEmpty(src) {
		return fmt.Errorf("目录 %s 仍有残留（可能被占用），已把能搬走的放到 %s", src, dst)
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("清空后删除 %s 失败（可能被占用）: %w", src, err)
	}
	return nil
}

func moveDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	var first error
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		if err := movePath(sp, dp); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func movePath(src, dst string) error {
	if cli.Lexists(dst) {
		if isRealDir(src) && isRealDir(dst) {
			if err := moveDirContents(src, dst); err != nil {
				return err
			}
			if dirEmpty(src) {
				return os.Remove(src)
			}
			return fmt.Errorf("目录 %s 仍有残留，拒绝覆盖目标 %s", src, dst)
		}
		if !isLink(src) && !isLink(dst) && !isRealDir(src) && !isRealDir(dst) {
			same, err := sameFileContent(src, dst)
			if err != nil {
				return err
			}
			if same {
				if err := os.Remove(src); err != nil {
					return fmt.Errorf("目标 %s 已有相同内容，但源 %s 未能删除: %w", dst, src, err)
				}
				return nil
			}
			return fmt.Errorf("目标已存在且内容不同，拒绝覆盖: %s", dst)
		}
		return fmt.Errorf("目标已存在，拒绝覆盖: %s", dst)
	}
	if err := renameDir(src, dst); err == nil {
		return nil
	}
	if isRealDir(src) {
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		if err := moveDirContents(src, dst); err != nil {
			return err
		}
		if !dirEmpty(src) {
			return fmt.Errorf("目录 %s 仍有残留（可能被占用）", src)
		}
		return os.Remove(src)
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	same, err := sameFileContent(src, dst)
	if err != nil || !same {
		_ = os.Remove(dst)
		if err != nil {
			return err
		}
		return fmt.Errorf("拷贝校验失败: %s -> %s", src, dst)
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("已拷到 %s 但源 %s 未能删除（可能被占用）: %w", dst, src, err)
	}
	return nil
}

// copyTree 把 src 目录树拷到 dst（真实文件）。不跟随符号链接/junction，跳过 .git 与迁移残留。
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if skipMigrateName(name) {
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
		strings.TrimPrefix(entry, "/"):       true,
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
