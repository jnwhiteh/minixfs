package inode

import (
	. "minixfs/common"
)

type req_InodeCache_MountDevice struct {
	devno  int
	bitmap Bitmap
	info   DeviceInfo
}
type res_InodeCache_MountDevice struct{}
type req_InodeCache_GetInode struct {
	devno, inum int
}
type res_InodeCache_GetInode struct {
	Arg0 Inode
	Arg1 error
}
type req_InodeCache_DupInode struct {
	rip Inode
}
type res_InodeCache_DupInode struct {
	Arg0 Inode
}
type req_InodeCache_RLockInode struct {
	rip InodeId
}
type res_InodeCache_RLockInode struct {
	Arg0 Inode
}
type req_InodeCache_RUnlockInode struct {
	rip Inode
}
type res_InodeCache_RUnlockInode struct {
	Arg0 InodeId
}
type req_InodeCache_WLockInode struct {
	rip Inode
}
type res_InodeCache_WLockInode struct {
	Arg0 LockedInode
}
type req_InodeCache_WUnlockInode struct {
	rip LockedInode
}
type res_InodeCache_WUnlockInode struct {
	Arg0 Inode
}
type req_InodeCache_PutInode struct {
	rip Inode
}
type res_InodeCache_PutInode struct{}
type req_InodeCache_FlushInode struct {
	rip LockedInode
}
type res_InodeCache_FlushInode struct{}
type req_InodeCache_IsDeviceBusy struct {
	devno int
}
type res_InodeCache_IsDeviceBusy struct {
	Arg0 bool
}
type req_InodeCache_Close struct {
}
type res_InodeCache_Close struct {
	Arg0 error
}

// Asynchronous response channel
type res_InodeCache_Async struct {
	ch chan resInodeCache
}

// Interface types and implementations
type reqInodeCache interface {
	is_reqInodeCache()
}
type resInodeCache interface {
	is_resInodeCache()
}

func (r req_InodeCache_MountDevice) is_reqInodeCache()  {}
func (r res_InodeCache_MountDevice) is_resInodeCache()  {}
func (r req_InodeCache_GetInode) is_reqInodeCache()     {}
func (r res_InodeCache_GetInode) is_resInodeCache()     {}
func (r req_InodeCache_DupInode) is_reqInodeCache()     {}
func (r res_InodeCache_DupInode) is_resInodeCache()     {}
func (r req_InodeCache_RLockInode) is_reqInodeCache()   {}
func (r res_InodeCache_RLockInode) is_resInodeCache()   {}
func (r req_InodeCache_RUnlockInode) is_reqInodeCache() {}
func (r res_InodeCache_RUnlockInode) is_resInodeCache() {}
func (r req_InodeCache_WLockInode) is_reqInodeCache()   {}
func (r res_InodeCache_WLockInode) is_resInodeCache()   {}
func (r req_InodeCache_WUnlockInode) is_reqInodeCache() {}
func (r res_InodeCache_WUnlockInode) is_resInodeCache() {}
func (r req_InodeCache_PutInode) is_reqInodeCache()     {}
func (r res_InodeCache_PutInode) is_resInodeCache()     {}
func (r req_InodeCache_FlushInode) is_reqInodeCache()   {}
func (r res_InodeCache_FlushInode) is_resInodeCache()   {}
func (r req_InodeCache_IsDeviceBusy) is_reqInodeCache() {}
func (r res_InodeCache_IsDeviceBusy) is_resInodeCache() {}
func (r req_InodeCache_Close) is_reqInodeCache()        {}
func (r res_InodeCache_Close) is_resInodeCache()        {}
func (r res_InodeCache_Async) is_resInodeCache()        {}

// Type check request/response types
var _ reqInodeCache = req_InodeCache_MountDevice{}
var _ resInodeCache = res_InodeCache_MountDevice{}
var _ reqInodeCache = req_InodeCache_GetInode{}
var _ resInodeCache = res_InodeCache_GetInode{}
var _ reqInodeCache = req_InodeCache_DupInode{}
var _ resInodeCache = res_InodeCache_DupInode{}
var _ reqInodeCache = req_InodeCache_RLockInode{}
var _ resInodeCache = res_InodeCache_RLockInode{}
var _ reqInodeCache = req_InodeCache_RUnlockInode{}
var _ resInodeCache = res_InodeCache_RUnlockInode{}
var _ reqInodeCache = req_InodeCache_WLockInode{}
var _ resInodeCache = res_InodeCache_WLockInode{}
var _ reqInodeCache = req_InodeCache_WUnlockInode{}
var _ resInodeCache = res_InodeCache_WUnlockInode{}
var _ reqInodeCache = req_InodeCache_PutInode{}
var _ resInodeCache = res_InodeCache_PutInode{}
var _ reqInodeCache = req_InodeCache_FlushInode{}
var _ resInodeCache = res_InodeCache_FlushInode{}
var _ reqInodeCache = req_InodeCache_IsDeviceBusy{}
var _ resInodeCache = res_InodeCache_IsDeviceBusy{}
var _ reqInodeCache = req_InodeCache_Close{}
var _ resInodeCache = res_InodeCache_Close{}
var _ resInodeCache = res_InodeCache_Async{}
