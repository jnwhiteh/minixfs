package minixfs

import (
	"os"
)

// The following string constants are taken from the Minix 3.1.0 source,
// specifically from lib/ansi/errlist.c.

var (
	EBUSY     = os.NewError("Resource busy")
	EEXIST    = os.NewError("File exists")
	EFBIG     = os.NewError("File too large")
	EINVAL    = os.NewError("Invalid argument")
	EISDIR    = os.NewError("Is a directory")
	EMFILE    = os.NewError("Too many open files")
	ENFILE    = os.NewError("File table overflow")
	ENOENT    = os.NewError("No such file or directory")
	ENOSPC    = os.NewError("No space left on device")
	ENOTDIR   = os.NewError("Not a directory")
	ENOTEMPTY = os.NewError("Directory not empty")
)
