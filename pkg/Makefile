all: bcache common device dinode finode icache super testutils
	make -C minixfs/fs

bcache: common
	make -C minixfs/bcache

common:
	make -C minixfs/common

device: common
	make -C minixfs/device

dinode: common
	make -C minixfs/dinode

finode: common
	make -C minixfs/finode

icache: common
	make -C minixfs/icache

super: common
	make -C minixfs/super

testutils: common device
	make -C minixfs/testutils

clean:
	make clean -C minixfs/bcache
	make clean -C minixfs/common
	make clean -C minixfs/device
	make clean -C minixfs/dinode
	make clean -C minixfs/finode
	make clean -C minixfs/fs
	make clean -C minixfs/icache
	make clean -C minixfs/super
	make clean -C minixfs/testutils

