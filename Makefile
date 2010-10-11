minixfs:
	make -C pkg/minixfs && make -C pkg/minixfs install

mkfs: minixfs
	make -C cmd/mkfs

all: minixfs mkfs

clean:
	make -C pkg/minixfs clean
	make -C cmd/mkfs clean


.PHONY: all clean mkfs minixfs
