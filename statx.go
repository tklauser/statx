// Copyright 2018 Tobias Klauser. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// statx - Report file status using the Linux statx(2) syscall
//
// The output format of statx is implemented as close as possible to the output
// of stat(1) from GNU coreutils.

// +build linux

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"time"

	"golang.org/x/sys/unix"
)

var (
	noAutomount = flag.Bool("A", false, "disable automount")
	basic       = flag.Bool("b", false, "basic stat(2) compatible stats only")
	follow      = flag.Bool("L", false, "follow symlinks")
	// TODO(tk): add flags for further AT_STATX_* flags and STATX_* mask
)

func statxTimestampToTime(sts unix.StatxTimestamp) time.Time {
	return time.Unix(sts.Sec, int64(sts.Nsec))
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
		var statx unix.Statx_t
		if err := unix.Statx(unix.AT_FDCWD, arg, flags, mask, &statx); err != nil {
			if err == unix.ENOSYS {
				log.Fatalf("The statx syscall is not supported. At least Linux kernel 4.11 is needed\n")
			}
			log.Fatalf("cannot statx '%s': %v", arg, err)
		}
		fmt.Printf("  File: '%s'\n", arg)

		fmt.Print(" ")
		if statx.Mask&unix.STATX_SIZE != 0 {
			fmt.Printf(" Size: %-15d", statx.Size)
		}
		if statx.Mask&unix.STATX_BLOCKS != 0 {
			fmt.Printf(" Blocks: %-10d", statx.Blocks)
		}
		fmt.Printf(" IO Block: %-6d", statx.Blksize)
		ft := '?'
		if statx.Mask&unix.STATX_TYPE != 0 {
			switch statx.Mode & unix.S_IFMT {
			case unix.S_IFIFO:
				fmt.Print(" FIFO")
				ft = 'p'
			case unix.S_IFCHR:
				fmt.Print(" character special file")
				ft = 'c'
			case unix.S_IFDIR:
				fmt.Print(" directory")
				ft = 'd'
			case unix.S_IFBLK:
				fmt.Print(" block special file")
				ft = 'b'
			case unix.S_IFREG:
				fmt.Print(" regular file")
				ft = '-'
			case unix.S_IFLNK:
				fmt.Print(" symbolic link")
				ft = 'l'
			case unix.S_IFSOCK:
				fmt.Print(" socket")
				ft = 's'
			default:
				fmt.Printf(" unknown type (%o)", statx.Mode&unix.S_IFMT)
			}
		} else {
			fmt.Printf(" no type")
		}
		fmt.Println()

		dev := unix.Mkdev(statx.Dev_major, statx.Dev_minor)
		fmt.Printf("Device: %-15s", fmt.Sprintf("%xh/%dd", dev, dev))
		if statx.Mask&unix.STATX_INO != 0 {
			fmt.Printf(" Inode: %-11d", statx.Ino)
		}
		if statx.Mask&unix.STATX_NLINK != 0 {
			fmt.Printf(" Links: %-5d", statx.Nlink)
		}
		if statx.Mask&unix.STATX_TYPE != 0 {
			switch statx.Mode & unix.S_IFMT {
			case unix.S_IFBLK:
				fallthrough
			case unix.S_IFCHR:
				fmt.Printf(" Device type: %d,%d", statx.Rdev_major, statx.Rdev_minor)
				break
			}
		}
		fmt.Println()

		if statx.Mask&unix.STATX_MODE != 0 {
			u := []byte{'-', '-', '-'}
			if statx.Mode&unix.S_IRUSR != 0 {
				u[0] = 'r'
			}
			if statx.Mode&unix.S_IWUSR != 0 {
				u[1] = 'w'
			}
			if statx.Mode&unix.S_IXUSR != 0 {
				u[2] = 'x'
			}
			g := []byte{'-', '-', '-'}
			if statx.Mode&unix.S_IRGRP != 0 {
				g[0] = 'r'
			}
			if statx.Mode&unix.S_IWGRP != 0 {
				g[1] = 'w'
			}
			if statx.Mode&unix.S_IXGRP != 0 {
				g[2] = 'x'
			}
			o := []byte{'-', '-', '-'}
			if statx.Mode&unix.S_IROTH != 0 {
				o[0] = 'r'
			}
			if statx.Mode&unix.S_IWOTH != 0 {
				o[1] = 'w'
			}
			if statx.Mode&unix.S_IXOTH != 0 {
				o[2] = 'x'
			}
			fmt.Printf("Access: (%04o/%c%s%s%s)  ", statx.Mode&07777, ft, u, g, o)
		}
		if statx.Mask&unix.STATX_UID != 0 {
			user, err := user.LookupId(fmt.Sprint(statx.Uid))
			if err == nil {
				fmt.Printf("Uid: (%5d/%8s)   ", statx.Uid, user.Username)
			} else {
				fmt.Printf("Uid: %5d   ", statx.Uid)
			}
		}
		if statx.Mask&unix.STATX_GID != 0 {
			group, err := user.LookupGroupId(fmt.Sprint(statx.Gid))
			if err == nil {
				fmt.Printf("Gid: (%5d/%8s)", statx.Gid, group.Name)
			} else {
				fmt.Printf("Gid: %5d", statx.Gid)
			}
		}
		fmt.Println()

		if statx.Mask&unix.STATX_ATIME != 0 {
			fmt.Println("Access:", statxTimestampToTime(statx.Atime))
		}
		if statx.Mask&unix.STATX_MTIME != 0 {
			fmt.Println("Modify:", statxTimestampToTime(statx.Mtime))
		}
		if statx.Mask&unix.STATX_CTIME != 0 {
			fmt.Println("Change:", statxTimestampToTime(statx.Ctime))
		}
		if statx.Mask&unix.STATX_BTIME != 0 {
			fmt.Println(" Birth:", statxTimestampToTime(statx.Btime))
		}

		if statx.Attributes_mask != 0 {
			fmt.Printf(" Attrs: %016x (", statx.Attributes)
			attrs := []struct {
				attr string
				mask uint64
			}{
				{"c", unix.STATX_ATTR_COMPRESSED}, // file is compressed by the fs
				{"i", unix.STATX_ATTR_IMMUTABLE},  // file is marked immutable
				{"a", unix.STATX_ATTR_APPEND},     // file is append-only
				{"d", unix.STATX_ATTR_NODUMP},     // file is not to be dumped
				{"e", unix.STATX_ATTR_ENCRYPTED},  // file requires key to decrypt in fs
			}
			for _, a := range attrs {
				if statx.Attributes_mask&a.mask == 0 {
					fmt.Print(".") // not supported
				} else if statx.Attributes&a.mask != 0 {
					fmt.Print(a.attr)
				} else {
					fmt.Print("-") // not set
				}
			}
			fmt.Println(")")
		}
	}
}
