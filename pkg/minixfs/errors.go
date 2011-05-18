package minixfs

import (
	"os"
)

// The following string constants are taken from the Minix 3.1.0 source,
// specifically from lib/ansi/errlist.c.

var (
	EBUSY   = os.NewError("Resource busy")
	EINVAL  = os.NewError("Invalid argument")
	EISDIR  = os.NewError("Is a directory")
	ENFILE  = os.NewError("File table overflow")
	ENOENT  = os.NewError("No such file or directory")
	ENOTDIR = os.NewError("Not a directory")
)
