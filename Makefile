include $(GOROOT)/src/Make.inc

TARG=minixfs
GOFILES=\
	const.go\
	inode.go\
	main.go\
	super.go\
	types.go\

include $(GOROOT)/src/Make.cmd
