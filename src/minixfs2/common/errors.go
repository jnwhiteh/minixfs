package common

import "errors"

// The following string constants are taken from the Minix 3.1.0 source,
// specifically from lib/ansi/errlist.c.

var (
	EBADF     = errors.New("Bad file number")
	EBUSY     = errors.New("Resource busy")
	EEXIST    = errors.New("File exists")
	EFBIG     = errors.New("File too large")
	EINVAL    = errors.New("Invalid argument")
	EISDIR    = errors.New("Is a directory")
	EMFILE    = errors.New("Too many open files")
	EMLINK    = errors.New("Too many links")
	ENFILE    = errors.New("File table overflow")
	ENOENT    = errors.New("No such file or directory")
	ENOSPC    = errors.New("No space left on device")
	ENOTDIR   = errors.New("Not a directory")
	ENOTEMPTY = errors.New("Directory not empty")
	EXDEV     = errors.New("Cross-device link")
)
