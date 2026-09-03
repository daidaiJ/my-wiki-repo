//go:build !windows

package registry

func isWindowsReparsePoint(string) bool { return false }
