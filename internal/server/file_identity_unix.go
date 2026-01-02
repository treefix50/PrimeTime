//go:build unix
// +build unix

package server

import (
	"fmt"
	"io/fs"
	"syscall"
)

func fileIdentity(info fs.FileInfo) (string, bool) {
	if info == nil {
		return "", false
	}
	switch stat := info.Sys().(type) {
	case *syscall.Stat_t:
		if stat != nil && stat.Ino != 0 {
			return fmt.Sprintf("ino:%d:%d", stat.Dev, stat.Ino), true
		}
	}
	return "", false
}
