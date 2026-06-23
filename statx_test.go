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
