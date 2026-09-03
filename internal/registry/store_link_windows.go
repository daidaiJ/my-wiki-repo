//go:build windows

package registry

import "syscall"

// isWindowsReparsePoint 识别符号链接与目录 junction（mklink /J）。
// Go 的 os.ModeSymlink 对 junction 不可靠，必须看 FILE_ATTRIBUTE_REPARSE_POINT。
func isWindowsReparsePoint(path string) bool {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attr, err := syscall.GetFileAttributes(p)
	if err != nil {
		return false
	}
	return attr&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
