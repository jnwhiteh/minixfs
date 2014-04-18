package bcache

import (
	"github.com/jnwhiteh/minixfs/common"
)

type req_BlockCache_MountDevice struct {
	devnum int
	dev    common.BlockDevice
	info   *common.DeviceInfo
}
type res_BlockCache_MountDevice struct {
	Arg0 error
}
type req_BlockCache_UnmountDevice struct {
	devnum int
}
type res_BlockCache_UnmountDevice struct {
	Arg0 error
}
type req_BlockCache_GetBlock struct {
	devnum, bnum int
	btype        common.BlockType
	only_search  int
}
type res_BlockCache_GetBlock struct {
	Arg0 *common.CacheBlock
}
type req_BlockCache_PutBlock struct {
	cb    *common.CacheBlock
	btype common.BlockType
}
type res_BlockCache_PutBlock struct {
	Arg0 error
}
type req_BlockCache_Invalidate struct {
	devnum int
}
type res_BlockCache_Invalidate struct{}
type req_BlockCache_Flush struct {
	devnum int
}
type res_BlockCache_Flush struct{}
type req_BlockCache_Shutdown struct{}
type res_BlockCache_Shutdown struct {
	Arg0 error
}
type res_BlockCache_Async struct {
	ch chan resBlockCache
}

// Interface types and implementations
type reqBlockCache interface {
	is_reqBlockCache()
}
type resBlockCache interface {
	is_resBlockCache()
}

func (r req_BlockCache_MountDevice) is_reqBlockCache()   {}
func (r res_BlockCache_MountDevice) is_resBlockCache()   {}
func (r req_BlockCache_UnmountDevice) is_reqBlockCache() {}
func (r res_BlockCache_UnmountDevice) is_resBlockCache() {}
func (r req_BlockCache_GetBlock) is_reqBlockCache()      {}
func (r res_BlockCache_GetBlock) is_resBlockCache()      {}
func (r req_BlockCache_PutBlock) is_reqBlockCache()      {}
func (r res_BlockCache_PutBlock) is_resBlockCache()      {}
func (r req_BlockCache_Invalidate) is_reqBlockCache()    {}
func (r res_BlockCache_Invalidate) is_resBlockCache()    {}
func (r req_BlockCache_Flush) is_reqBlockCache()         {}
func (r res_BlockCache_Flush) is_resBlockCache()         {}
func (r req_BlockCache_Shutdown) is_reqBlockCache()      {}
func (r res_BlockCache_Shutdown) is_resBlockCache()      {}
func (r res_BlockCache_Async) is_resBlockCache()         {}

// Type check request/response types
var _ reqBlockCache = req_BlockCache_MountDevice{}
var _ resBlockCache = res_BlockCache_MountDevice{}
var _ reqBlockCache = req_BlockCache_UnmountDevice{}
var _ resBlockCache = res_BlockCache_UnmountDevice{}
var _ reqBlockCache = req_BlockCache_GetBlock{}
var _ resBlockCache = res_BlockCache_GetBlock{}
var _ reqBlockCache = req_BlockCache_PutBlock{}
var _ resBlockCache = res_BlockCache_PutBlock{}
var _ reqBlockCache = req_BlockCache_Invalidate{}
var _ resBlockCache = res_BlockCache_Invalidate{}
var _ reqBlockCache = req_BlockCache_Flush{}
var _ resBlockCache = res_BlockCache_Flush{}
var _ reqBlockCache = req_BlockCache_Shutdown{}
var _ resBlockCache = res_BlockCache_Shutdown{}
var _ resBlockCache = res_BlockCache_Async{}

func (c *LRUCache) MountDevice(devnum int, dev common.BlockDevice, info *common.DeviceInfo) error {
	c.in <- req_BlockCache_MountDevice{devnum, dev, info}
	result := (<-c.out).(res_BlockCache_MountDevice)
	return result.Arg0
}
func (c *LRUCache) UnmountDevice(devnum int) error {
	c.in <- req_BlockCache_UnmountDevice{devnum}
	result := (<-c.out).(res_BlockCache_UnmountDevice)
	return result.Arg0
}
func (c *LRUCache) GetBlock(devnum, blocknum int, btype common.BlockType, only_search int) *common.CacheBlock {
	c.in <- req_BlockCache_GetBlock{devnum, blocknum, btype, only_search}
	ares := (<-c.out).(res_BlockCache_Async)
	result := (<-ares.ch).(res_BlockCache_GetBlock)
	// TODO: Triggering this panic is a bit of a pain, can we do better?
	if result.Arg0 == LRU_ALLINUSE {
		panic("all buffers in use")
	}

	return result.Arg0
}
func (c *LRUCache) PutBlock(cb *common.CacheBlock, btype common.BlockType) error {
	c.in <- req_BlockCache_PutBlock{cb, btype}
	result := (<-c.out).(res_BlockCache_PutBlock)
	return result.Arg0
}
func (c *LRUCache) Invalidate(devnum int) {
	c.in <- req_BlockCache_Invalidate{devnum}
	<-c.out
	return
}
func (c *LRUCache) Flush(devnum int) {
	c.in <- req_BlockCache_Flush{devnum}
	<-c.out
	return
}
func (c *LRUCache) Shutdown() error {
	c.in <- req_BlockCache_Shutdown{}
	result := (<-c.out).(res_BlockCache_Shutdown)
	return result.Arg0
}
