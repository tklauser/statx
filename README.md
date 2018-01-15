# statx

statx reports file status using the Linux
[statx(2)](http://man7.org/linux/man-pages/man2/statx.2.html) syscall.

The output format of statx is implemented as close as possible to the output of
[stat(1)](http://www.gnu.org/software/coreutils/stat) from GNU coreutils.

Installation
============

```
$ go get -u github.com/tklauser/statx
```

Usage
=====

```
Usage of ./statx:
  -A	disable automount
  -L	follow symlinks
  -b	basic stat(2) compatible stats only
```
