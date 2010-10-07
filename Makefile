include $(GOROOT)/src/Make.inc

TARG=mkfs.minix3
GOFILES=\
	const.go\
	inode.go\
	main.go\
	super.go\

include $(GOROOT)/src/Make.cmd
