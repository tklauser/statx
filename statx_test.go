// Copyright 2026 Tobias Klauser. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux

package main

import (
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
