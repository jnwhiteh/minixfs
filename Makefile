all:
	GOPATH=$(PWD) go build all

minixfs:
	go build minixfs/fs

fsck:
	go install fsck

test:
	go test -v minixfs/bcache minixfs/inode minixfs/fs

clean:
	rm -fr bin pkg

.PHONY: all minixfs fsck test clean
