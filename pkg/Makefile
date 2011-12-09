all: bcache bitmap common device dinode finode icache testutils utils
	make -C minixfs/fs

bcache: common
	make -C minixfs/bcache

bitmap: common
	make -C minixfs/bitmap

common:
	make -C minixfs/common

device: common
	make -C minixfs/device

dinode: common utils
	make -C minixfs/dinode

finode: common utils
	make -C minixfs/finode

icache: common
	make -C minixfs/icache

testutils: common device
	make -C minixfs/testutils

utils: common
	make -C minixfs/utils

clean:
	make clean -C minixfs/bcache
	make clean -C minixfs/bitmap
	make clean -C minixfs/common
	make clean -C minixfs/device
	make clean -C minixfs/dinode
	make clean -C minixfs/finode
	make clean -C minixfs/fs
	make clean -C minixfs/icache
	make clean -C minixfs/testutils
	make clean -C minixfs/utils

