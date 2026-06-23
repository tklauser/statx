// Copyright 2026 Tobias Klauser. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestFileTypeString(t *testing.T) {
	tests := []struct {
		name     string
		mode     uint16
		mask     uint32
		wantType string
		wantChar byte
	}{
		{
			name:     "regular file",
			mode:     uint16(unix.S_IFREG | 0644),
			mask:     unix.STATX_TYPE,
			wantType: "regular file",
			wantChar: '-',
		},
		{
			name:     "directory",
			mode:     uint16(unix.S_IFDIR | 0755),
			mask:     unix.STATX_TYPE,
			wantType: "directory",
			wantChar: 'd',
		},
		{
			name:     "symbolic link",
			mode:     uint16(unix.S_IFLNK | 0777),
			mask:     unix.STATX_TYPE,
			wantType: "symbolic link",
			wantChar: 'l',
		},
		{
			name:     "type unavailable",
			mode:     0,
			mask:     0,
			wantType: "no type",
			wantChar: '?',
		},
		{
			name:     "unknown type",
			mode:     0,
			mask:     unix.STATX_TYPE,
			wantType: "unknown type (0)",
			wantChar: '?',
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotChar := fileTypeString(tt.mode, tt.mask)
			if gotType != tt.wantType || gotChar != tt.wantChar {
				t.Fatalf("fileTypeString(%#o, %#x) = %q, %q; want %q, %q",
					tt.mode, tt.mask, gotType, gotChar, tt.wantType, tt.wantChar)
			}
		})
	}
}

func TestModeString(t *testing.T) {
	tests := []struct {
		name string
		mode uint16
		ft   byte
		want string
	}{
		{
			name: "regular file",
			mode: uint16(unix.S_IFREG | 0644),
			ft:   '-',
			want: "0644/-rw-r--r--",
		},
		{
			name: "directory",
			mode: uint16(unix.S_IFDIR | 0755),
			ft:   'd',
			want: "0755/drwxr-xr-x",
		},
		{
			name: "no permissions",
			mode: 0,
			ft:   '?',
			want: "0000/?---------",
		},
		{
			name: "special bits",
			mode: uint16(04755),
			ft:   '-',
			want: "4755/-rwxr-xr-x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modeString(tt.mode, tt.ft); got != tt.want {
				t.Fatalf("modeString(%#o, %q) = %q, want %q", tt.mode, tt.ft, got, tt.want)
			}
		})
	}
}

func TestFormatStatxTimestamp(t *testing.T) {
	oldLocal := time.Local
	time.Local = time.UTC
	defer func() {
		time.Local = oldLocal
	}()

	ts := unix.StatxTimestamp{
		Sec:  1782205445,
		Nsec: 123456789,
	}

	got := formatStatxTimestamp(ts)
	want := "2026-06-23 09:04:05.123456789 +0000"
	if got != want {
		t.Fatalf("formatStatxTimestamp(%+v) = %q, want %q", ts, got, want)
	}
}

func TestAttributeString(t *testing.T) {
	allAttributesMask := uint64(
		unix.STATX_ATTR_COMPRESSED |
			unix.STATX_ATTR_IMMUTABLE |
			unix.STATX_ATTR_APPEND |
			unix.STATX_ATTR_NODUMP |
			unix.STATX_ATTR_ENCRYPTED |
			unix.STATX_ATTR_AUTOMOUNT |
			unix.STATX_ATTR_MOUNT_ROOT |
			unix.STATX_ATTR_VERITY |
			unix.STATX_ATTR_DAX |
			unix.STATX_ATTR_WRITE_ATOMIC)

	tests := []struct {
		name           string
		attributes     uint64
		attributesMask uint64
		want           string
	}{
		{
			name:           "unsupported",
			attributes:     0,
			attributesMask: 0,
			want:           "..........",
		},
		{
			name:           "supported but unset",
			attributes:     0,
			attributesMask: allAttributesMask,
			want:           "----------",
		},
		{
			name:           "all set",
			attributes:     allAttributesMask,
			attributesMask: allAttributesMask,
			want:           "ciadeAmvDx",
		},
		{
			name:           "mixed support and values",
			attributes:     unix.STATX_ATTR_IMMUTABLE,
			attributesMask: unix.STATX_ATTR_COMPRESSED | unix.STATX_ATTR_IMMUTABLE | unix.STATX_ATTR_APPEND,
			want:           "-i-.......",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attributesString(tt.attributes, tt.attributesMask); got != tt.want {
				t.Fatalf("attributesString(%#x, %#x) = %q, want %q",
					tt.attributes, tt.attributesMask, got, tt.want)
			}
		})
	}
}

func TestPrintStatxMatchesGNUStat(t *testing.T) {
	if _, err := exec.LookPath("stat"); err != nil {
		t.Skip("stat command not found")
	}
	version, err := exec.Command("stat", "--version").Output()
	if err != nil || !strings.Contains(string(version), "GNU coreutils") {
		t.Skip("GNU stat command not found")
	}

	tests := []struct {
		name             string
		ignoreAccessTime bool
		setup            func(t *testing.T) (string, error)
	}{
		{
			name: "regular file",
			setup: func(t *testing.T) (string, error) {
				path := filepath.Join(t.TempDir(), "file")
				return path, os.WriteFile(path, []byte("statx test file\n"), 0644)
			},
		},
		{
			name: "directory",
			setup: func(t *testing.T) (string, error) {
				path := filepath.Join(t.TempDir(), "directory")
				return path, os.Mkdir(path, 0755)
			},
		},
		{
			name:             "symlink",
			ignoreAccessTime: true,
			setup: func(t *testing.T) (string, error) {
				file := filepath.Join(t.TempDir(), "file")
				symlink := filepath.Join(t.TempDir(), "symlink")
				if err := os.Symlink(file, symlink); err != nil {
					return "", err
				}
				if err := os.WriteFile(file, []byte("statx symlink target\n"), 0644); err != nil {
					return "", err
				}
				return symlink, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := tt.setup(t)
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			statCmd := exec.Command("stat", path)
			statCmd.Env = append(os.Environ(), "LC_ALL=C")
			want, err := statCmd.Output()
			if err != nil {
				t.Fatalf("stat %q failed: %v", path, err)
			}

			var buf bytes.Buffer
			if err := printStatx(&buf, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_ALL); err != nil {
				if err == unix.ENOSYS {
					t.Skip("statx syscall not supported")
				} else {
					t.Fatalf("printStatx(%q) failed: %v", path, err)
				}
			}

			gotLines := comparableStatLines(buf.String(), tt.ignoreAccessTime)
			wantLines := comparableStatLines(string(want), tt.ignoreAccessTime)
			if diff := lineDiff(wantLines, gotLines); diff != "" {
				t.Fatalf("printStatx output differs from GNU stat output\n%s", diff)
			}
		})
	}
}

func TestComparableStatLinesNormalizesDeviceLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "old GNU stat device format",
			in:   "Device: 801h/2049d\tInode: 42         Links: 1\n",
			want: []string{"Device: 8,1\tInode: 42         Links: 1"},
		},
		{
			name: "current GNU stat device format",
			in:   "Device: 8,1\tInode: 42         Links: 1\n",
			want: []string{"Device: 8,1\tInode: 42         Links: 1"},
		},
		{
			name: "attrs line omitted",
			in:   "Device: 8,1\tInode: 42         Links: 1\n Attrs: 0000000000000000 (----------)\n",
			want: []string{"Device: 8,1\tInode: 42         Links: 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := comparableStatLines(tt.in, false)
			if diff := lineDiff(tt.want, got); diff != "" {
				t.Fatalf("comparableStatLines mismatch\n%s", diff)
			}
		})
	}
}

func comparableStatLines(s string, ignoreAccessTime bool) []string {
	var lines []string
	for line := range strings.SplitSeq(strings.TrimSuffix(s, "\n"), "\n") {
		if strings.HasPrefix(line, " Attrs:") {
			continue
		}
		if ignoreAccessTime && isAccessTimeLine(line) {
			continue
		}
		lines = append(lines, normalizeDeviceLine(line))
	}
	return lines
}

func normalizeDeviceLine(line string) string {
	const prefix = "Device: "
	if !strings.HasPrefix(line, prefix) {
		return line
	}

	rest := strings.TrimPrefix(line, prefix)
	tokenEnd := strings.IndexFunc(rest, func(r rune) bool {
		return r == ' ' || r == '\t'
	})
	if tokenEnd < 0 {
		return line
	}

	token := rest[:tokenEnd]
	suffix := rest[tokenEnd:]
	if strings.Contains(token, ",") {
		return line
	}

	slash := strings.IndexByte(token, '/')
	if slash < 0 || !strings.HasSuffix(token[:slash], "h") || !strings.HasSuffix(token, "d") {
		return line
	}

	dev, err := strconv.ParseUint(strings.TrimSuffix(token[slash+1:], "d"), 10, 64)
	if err != nil {
		return line
	}
	return fmt.Sprintf("%s%d,%d%s", prefix, unix.Major(dev), unix.Minor(dev), suffix)
}

func isAccessTimeLine(line string) bool {
	return strings.HasPrefix(line, "Access: ") && !strings.HasPrefix(line, "Access: (")
}

func lineDiff(want, got []string) string {
	var b strings.Builder
	for i := 0; i < len(want) || i < len(got); i++ {
		switch {
		case i >= len(want):
			fmt.Fprintf(&b, "+ %s\n", got[i])
		case i >= len(got):
			fmt.Fprintf(&b, "- %s\n", want[i])
		case want[i] != got[i]:
			fmt.Fprintf(&b, "- %s\n+ %s\n", want[i], got[i])
		}
	}
	return b.String()
}
