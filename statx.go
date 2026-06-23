// Copyright 2018 Tobias Klauser. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// statx - Report file status using the Linux statx(2) syscall
//
// The output format of statx is implemented as close as possible to the output
// of stat(1) from GNU coreutils.

//go:build linux

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

var (
	noAutomount = flag.Bool("A", false, "disable automount")
	basic       = flag.Bool("b", false, "basic stat(2) compatible stats only")
	follow      = flag.Bool("L", false, "follow symlinks")
	// TODO(tk): add flags for further AT_STATX_* flags and STATX_* mask
)

func fileTypeString(mode uint16, mask uint32) (string, byte) {
	if mask&unix.STATX_TYPE == 0 {
		return "no type", '?'
	}

	switch mode & unix.S_IFMT {
	case unix.S_IFIFO:
		return "FIFO", 'p'
	case unix.S_IFCHR:
		return "character special file", 'c'
	case unix.S_IFDIR:
		return "directory", 'd'
	case unix.S_IFBLK:
		return "block special file", 'b'
	case unix.S_IFREG:
		return "regular file", '-'
	case unix.S_IFLNK:
		return "symbolic link", 'l'
	case unix.S_IFSOCK:
		return "socket", 's'
	default:
		return fmt.Sprintf("unknown type (%o)", mode&unix.S_IFMT), '?'
	}
}

func modeString(mode uint16, ft byte) string {
	u := []byte{'-', '-', '-'}
	if mode&unix.S_IRUSR != 0 {
		u[0] = 'r'
	}
	if mode&unix.S_IWUSR != 0 {
		u[1] = 'w'
	}
	if mode&unix.S_IXUSR != 0 {
		u[2] = 'x'
	}
	g := []byte{'-', '-', '-'}
	if mode&unix.S_IRGRP != 0 {
		g[0] = 'r'
	}
	if mode&unix.S_IWGRP != 0 {
		g[1] = 'w'
	}
	if mode&unix.S_IXGRP != 0 {
		g[2] = 'x'
	}
	o := []byte{'-', '-', '-'}
	if mode&unix.S_IROTH != 0 {
		o[0] = 'r'
	}
	if mode&unix.S_IWOTH != 0 {
		o[1] = 'w'
	}
	if mode&unix.S_IXOTH != 0 {
		o[2] = 'x'
	}
	return fmt.Sprintf("%04o/%c%s%s%s", mode&07777, ft, u, g, o)
}

func formatStatxTimestamp(sts unix.StatxTimestamp) string {
	return time.Unix(sts.Sec, int64(sts.Nsec)).Format("2006-01-02 15:04:05.000000000 -0700")
}

func attributesString(attributes uint64, attributesMask uint64) string {
	attrs := []struct {
		attr byte
		mask uint64
	}{
		{'c', unix.STATX_ATTR_COMPRESSED},   // file is compressed by the fs
		{'i', unix.STATX_ATTR_IMMUTABLE},    // file is marked immutable
		{'a', unix.STATX_ATTR_APPEND},       // file is append-only
		{'d', unix.STATX_ATTR_NODUMP},       // file is not to be dumped
		{'e', unix.STATX_ATTR_ENCRYPTED},    // file requires key to decrypt in fs
		{'A', unix.STATX_ATTR_AUTOMOUNT},    // dir: Automount trigger
		{'m', unix.STATX_ATTR_MOUNT_ROOT},   // root of a mount
		{'v', unix.STATX_ATTR_VERITY},       // verity protected file
		{'D', unix.STATX_ATTR_DAX},          // file is currenlunix.STATX_ATTR_DAXy in DAX state
		{'x', unix.STATX_ATTR_WRITE_ATOMIC}, // file supports atomic write operations
	}
	var sb strings.Builder
	for _, a := range attrs {
		if attributesMask&a.mask == 0 {
			sb.WriteByte('.') // not supported
		} else if attributes&a.mask != 0 {
			sb.WriteByte(a.attr)
		} else {
			sb.WriteByte('-') // not set
		}
	}
	return sb.String()
}

func printStatx(out io.Writer, arg string, flags int, mask int) error {
	var statx unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, arg, flags, mask, &statx); err != nil {
		return err
	}

	ftStr, ftChar := fileTypeString(statx.Mode, statx.Mask)

	var extra string
	if ftChar == 'l' {
		link, err := os.Readlink(arg)
		if err == nil {
			extra = fmt.Sprintf(" -> %s", link)
		}
	}
	fmt.Fprintf(out, "  File: %s%s\n ", arg, extra)

	if statx.Mask&unix.STATX_SIZE != 0 {
		fmt.Fprintf(out, " Size: %-10d", statx.Size)
	}
	if statx.Mask&unix.STATX_BLOCKS != 0 {
		fmt.Fprintf(out, "\tBlocks: %-10d", statx.Blocks)
	}
	fmt.Fprintf(out, " IO Block: %-6d %s\n", statx.Blksize, ftStr)

	fmt.Fprintf(out, "Device: %d,%d", statx.Dev_major, statx.Dev_minor)
	if statx.Mask&unix.STATX_INO != 0 {
		fmt.Fprintf(out, "\tInode: %-11d", statx.Ino)
	}
	if statx.Mask&unix.STATX_NLINK != 0 {
		fmt.Fprintf(out, " Links: %d", statx.Nlink)
	}
	if statx.Mask&unix.STATX_TYPE != 0 {
		switch statx.Mode & unix.S_IFMT {
		case unix.S_IFBLK:
			fallthrough
		case unix.S_IFCHR:
			fmt.Fprintf(out, "\tDevice type: %d,%d", statx.Rdev_major, statx.Rdev_minor)
		}
	}
	fmt.Fprintln(out)

	if statx.Mask&unix.STATX_MODE != 0 {
		fmt.Fprintf(out, "Access: (%s)  ", modeString(statx.Mode, ftChar))
	}
	if statx.Mask&unix.STATX_UID != 0 {
		user, err := user.LookupId(fmt.Sprint(statx.Uid))
		if err == nil {
			fmt.Fprintf(out, "Uid: (%5d/%8s)   ", statx.Uid, user.Username)
		} else {
			fmt.Fprintf(out, "Uid: %5d   ", statx.Uid)
		}
	}
	if statx.Mask&unix.STATX_GID != 0 {
		group, err := user.LookupGroupId(fmt.Sprint(statx.Gid))
		if err == nil {
			fmt.Fprintf(out, "Gid: (%5d/%8s)", statx.Gid, group.Name)
		} else {
			fmt.Fprintf(out, "Gid: %5d", statx.Gid)
		}
	}
	fmt.Fprintln(out)

	if statx.Mask&unix.STATX_ATIME != 0 {
		fmt.Fprintln(out, "Access:", formatStatxTimestamp(statx.Atime))
	}
	if statx.Mask&unix.STATX_MTIME != 0 {
		fmt.Fprintln(out, "Modify:", formatStatxTimestamp(statx.Mtime))
	}
	if statx.Mask&unix.STATX_CTIME != 0 {
		fmt.Fprintln(out, "Change:", formatStatxTimestamp(statx.Ctime))
	}
	if statx.Mask&unix.STATX_BTIME != 0 {
		fmt.Fprintln(out, " Birth:", formatStatxTimestamp(statx.Btime))
	}

	if statx.Attributes_mask != 0 {
		fmt.Fprintf(out, " Attrs: %016x (%s)",
			statx.Attributes,
			attributesString(statx.Attributes, statx.Attributes_mask),
		)
	}

	return nil
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	flags := unix.AT_SYMLINK_NOFOLLOW
	mask := unix.STATX_ALL

	if *noAutomount {
		flags |= unix.AT_NO_AUTOMOUNT
	}
	if *basic {
		mask = unix.STATX_BASIC_STATS
	}
	if *follow {
		flags &^= unix.AT_SYMLINK_NOFOLLOW
	}

	for _, arg := range flag.Args() {
		if err := printStatx(os.Stdout, arg, flags, mask); err != nil {
			if err == unix.ENOSYS {
				log.Fatalf("The statx syscall is not supported. At least Linux kernel 4.11 is needed\n")
			}
			log.Fatalf("cannot statx '%s': %v", arg, err)
		}
	}
}
