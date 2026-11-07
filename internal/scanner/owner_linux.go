//go:build linux

package scanner

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func getOSFileOwner(fi os.FileInfo) string {
	if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
		uid := strconv.FormatUint(uint64(stat.Uid), 10)
		if u, err := user.LookupId(uid); err == nil && u.Username != "" {
			return u.Username
		}
	}
	return "www-data"
}

