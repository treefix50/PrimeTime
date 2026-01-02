//go:build !unix
// +build !unix

package server

import "io/fs"

func fileIdentity(info fs.FileInfo) (string, bool) {
	return "", false
}
