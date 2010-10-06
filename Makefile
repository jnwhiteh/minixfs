include $(GOROOT)/src/Make.inc

TARG=minixfs
GOFILES=\
	const.go\
	main.go\

include $(GOROOT)/src/Make.cmd
