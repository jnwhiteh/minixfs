all: bcache common device icache super testutils

common:
	make -C minixfs/common

device: common
	make -C minixfs/device

bcache: common
	make -C minixfs/bcache

super: common
	make -C minixfs/super

testutils: common device
	make -C minixfs/testutils

icache: common
	make -C minixfs/icache

finode: common
	make -C minixfs/finode

clean:
	make clean -C minixfs/bcache
	make clean -C minixfs/common
	make clean -C minixfs/device
	make clean -C minixfs/finode
	make clean -C minixfs/icache
	make clean -C minixfs/super
	make clean -C minixfs/testutils

