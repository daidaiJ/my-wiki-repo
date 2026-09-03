package registry

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/daidaiJ/my-wiki-repo/internal/cli"
	"github.com/daidaiJ/my-wiki-repo/internal/config"
)

const bundleMarker = ".wiki-bundle"

// CmdBundle 方案 B：按需把知识库正文克隆成真实目录树，并可额外打 zip/tgz 归档。
// 不走 hook，不改项目侧窗口。
func CmdBundle(args []string) error {
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	outDir := fs.String("dir", "", "克隆输出目录（默认 <wikiRoot>/bundle）")
	archive := fs.String("archive", "", "额外压缩归档：zip 或 tgz")
	name := fs.String("name", "", "归档文件名（不含扩展名，默认 wiki-bundle-<时间戳>）")
	if err := cli.ParseWithPositionals(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("用法: wiki bundle [--dir <目录>] [--archive zip|tgz] [--name <文件名>]")
	}

	root := config.WikiRoot()
	reg, err := LoadRegistry(root)
	if err != nil {
		return err
	}
	if len(reg.Projects) == 0 {
		return errors.New("尚未注册任何项目，没有可打包的知识正文")
	}

	dest := *outDir
	if dest == "" {
		dest = filepath.Join(root, BundleRootName)
	} else {
		dest, err = filepath.Abs(dest)
		if err != nil {
			return err
		}
	}
	if err := prepareBundleDir(dest); err != nil {
		return err
	}

	nFiles := 0
	for _, p := range reg.Projects {
		taken := map[string]bool{}
		for _, rel := range p.Paths {
			store := linkPathFor(root, p, rel, taken)
			src := store
			if resolved, err := filepath.EvalSymlinks(store); err == nil {
				src = resolved
			}
			if !cli.DirExists(src) {
				fmt.Fprintf(os.Stderr, "wiki bundle: 跳过缺失路径 %s\n", store)
				continue
			}
			sub := filepath.Join(dest, p.Name, filepath.Base(store))
			if err := copyTree(src, sub); err != nil {
				return fmt.Errorf("克隆 %s 失败: %w", store, err)
			}
			n, _ := countRegularFiles(sub)
			nFiles += n
		}
	}
	if err := os.WriteFile(filepath.Join(dest, bundleMarker), []byte("wiki bundle\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("已克隆知识目录树到 %s（%d 个文件）\n", dest, nFiles)

	kind := strings.ToLower(strings.TrimSpace(*archive))
	if kind == "" {
		return nil
	}
	base := *name
	if base == "" {
		base = "wiki-bundle-" + time.Now().Format("20060102-150405")
	}
	var archPath string
	switch kind {
	case "zip":
		archPath = filepath.Join(filepath.Dir(dest), base+".zip")
		if err := zipDir(dest, archPath); err != nil {
			return err
		}
	case "tgz", "tar.gz":
		archPath = filepath.Join(filepath.Dir(dest), base+".tar.gz")
		if err := tgzDir(dest, archPath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("未知归档格式 %q（可用 zip 或 tgz）", *archive)
	}
	fmt.Printf("已额外写出归档 %s\n", archPath)
	return nil
}

func prepareBundleDir(dest string) error {
	fi, err := os.Lstat(dest)
	if os.IsNotExist(err) {
		return os.MkdirAll(dest, 0o755)
	}
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("输出路径 %s 不是目录", dest)
	}
	marker := filepath.Join(dest, bundleMarker)
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if _, err := os.Stat(marker); err != nil {
		return fmt.Errorf("目标目录 %s 已存在且不是 wiki bundle 输出（无 %s 标记），拒绝覆盖", dest, bundleMarker)
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.MkdirAll(dest, 0o755)
}

func countRegularFiles(dir string) (int, error) {
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Type()&os.ModeSymlink == 0 {
			n++
		}
		return nil
	})
	return n, err
}

func zipDir(src, zipPath string) error {
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	closeZW := zw.Close()
	closeF := f.Close()
	if err != nil {
		return err
	}
	if closeZW != nil {
		return closeZW
	}
	return closeF
}

func tgzDir(src, tgzPath string) error {
	f, err := os.Create(tgzPath)
	if err != nil {
		return err
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
			return tw.WriteHeader(hdr)
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	closeTW := tw.Close()
	closeGW := gw.Close()
	closeF := f.Close()
	if err != nil {
		return err
	}
	if closeTW != nil {
		return closeTW
	}
	if closeGW != nil {
		return closeGW
	}
	return closeF
}
