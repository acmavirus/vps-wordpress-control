//go:build !linux

package scanner

import "os"

func getOSFileOwner(fi os.FileInfo) string {
	return "www-data"
}
