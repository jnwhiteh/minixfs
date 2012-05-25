package bcache

import (
	. "minixfs2/common"
)

type req_BlockCache_MountDevice struct {
	devnum int
	dev BlockDevice
	info DeviceInfo
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
	btype BlockType
	only_search int
}
type res_BlockCache_GetBlock struct {
	Arg0 *CacheBlock
}
type req_BlockCache_PutBlock struct {
	cb *CacheBlock
	btype BlockType
}
type res_BlockCache_PutBlock struct {
	Arg0 error
}
type req_BlockCache_Invalidate struct {
	device int
}
type res_BlockCache_Invalidate struct {}
type req_BlockCache_Flush struct {
	device int
}
type res_BlockCache_Flush struct {}
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
func (r req_BlockCache_MountDevice) is_reqBlockCache() {}
func (r res_BlockCache_MountDevice) is_resBlockCache() {}
func (r req_BlockCache_UnmountDevice) is_reqBlockCache() {}
func (r res_BlockCache_UnmountDevice) is_resBlockCache() {}
func (r req_BlockCache_GetBlock) is_reqBlockCache() {}
func (r res_BlockCache_GetBlock) is_resBlockCache() {}
func (r req_BlockCache_PutBlock) is_reqBlockCache() {}
func (r res_BlockCache_PutBlock) is_resBlockCache() {}
func (r req_BlockCache_Invalidate) is_reqBlockCache() {}
func (r res_BlockCache_Invalidate) is_resBlockCache() {}
func (r req_BlockCache_Flush) is_reqBlockCache() {}
func (r res_BlockCache_Flush) is_resBlockCache() {}
func (r res_BlockCache_Async) is_resBlockCache() {}

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
var _ resBlockCache = res_BlockCache_Async{}

func (c *LRUCache) MountDevice(devnum int, dev BlockDevice, info DeviceInfo) (error) {
	c.in <- req_BlockCache_MountDevice{devnum, dev, info}
	result := (<-c.out).(res_BlockCache_MountDevice)
	return result.Arg0
}
func (c *LRUCache) UnmountDevice(devnum int) (error) {
	c.in <- req_BlockCache_UnmountDevice{devnum}
	result := (<-c.out).(res_BlockCache_UnmountDevice)
	return result.Arg0
}
func (c *LRUCache) GetBlock(devnum, blocknum int, btype BlockType, only_search int) (*CacheBlock) {
	c.in <- req_BlockCache_GetBlock{devnum, blocknum, btype, only_search}
	ares := (<-c.out).(res_BlockCache_Async)
	result := (<-ares.ch).(res_BlockCache_GetBlock)
	return result.Arg0
}
func (c *LRUCache) PutBlock(cb *CacheBlock, btype BlockType) (error) {
	c.in <- req_BlockCache_PutBlock{cb, btype}
	result := (<-c.out).(res_BlockCache_PutBlock)
	return result.Arg0
}
func (c *LRUCache) Invalidate(device int) () {
	c.in <- req_BlockCache_Invalidate{device}
	<-c.out
	return
}
func (c *LRUCache) Flush(device int) () {
	c.in <- req_BlockCache_Flush{device}
	<-c.out
	return
}

