minixfs:
	make -C pkg/minixfs && make -C pkg/minixfs install

mkfs: minixfs
	make -C cmd/mkfs

example: minixfs
	make -C cmd/example

example.run: minixfs example
	cmd/example/example

all: minixfs mkfs example

clean:
	make -C pkg/minixfs clean
	make -C cmd/mkfs clean
	make -C cmd/example clean

test:
	make -C pkg/minixfs test

.PHONY: all clean mkfs example minixfs
