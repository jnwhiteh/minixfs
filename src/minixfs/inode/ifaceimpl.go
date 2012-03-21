package inode

import (
	. "minixfs/common"
)

//////////////////////////////////////////////////////////////////////////////
// Interface implementation
//////////////////////////////////////////////////////////////////////////////

func (s *inodeCache) MountDevice(devno int, bitmap Bitmap, info DeviceInfo) () {
	s.in <- req_InodeCache_MountDevice{devno, bitmap, info}
	<-s.out
	return
}
func (s *inodeCache) GetInode(devno, inum int) (Inode, error) {
	s.in <- req_InodeCache_GetInode{devno, inum}
	ares := (<-s.out).(res_InodeCache_Async)
	result := (<-ares.ch).(res_InodeCache_GetInode)
	return result.Arg0, result.Arg1
}
func (s *inodeCache) DupInode(rip Inode) (Inode) {
	s.in <- req_InodeCache_DupInode{rip}
	ares := (<-s.out).(res_InodeCache_Async)
	result := (<-ares.ch).(res_InodeCache_DupInode)
	return result.Arg0
}
func (s *inodeCache) RLockInode(rip InodeId) (Inode) {
	s.in <- req_InodeCache_RLockInode{rip}
	ares := (<-s.out).(res_InodeCache_Async)
	result := (<-ares.ch).(res_InodeCache_RLockInode)
	return result.Arg0
}
func (s *inodeCache) RUnlockInode(rip Inode) (InodeId) {
	s.in <- req_InodeCache_RUnlockInode{rip}
	ares := (<-s.out).(res_InodeCache_Async)
	result := (<-ares.ch).(res_InodeCache_RUnlockInode)
	return result.Arg0
}
func (s *inodeCache) WLockInode(rip Inode) (LockedInode) {
	s.in <- req_InodeCache_WLockInode{rip}
	ares := (<-s.out).(res_InodeCache_Async)
	result := (<-ares.ch).(res_InodeCache_WLockInode)
	return result.Arg0
}
func (s *inodeCache) WUnlockInode(rip LockedInode) (Inode) {
	s.in <- req_InodeCache_WUnlockInode{rip}
	ares := (<-s.out).(res_InodeCache_Async)
	result := (<-ares.ch).(res_InodeCache_WUnlockInode)
	return result.Arg0
}
func (s *inodeCache) PutInode(rip Inode) () {
	s.in <- req_InodeCache_PutInode{rip}
	<-s.out
	return
}
func (s *inodeCache) FlushInode(rip LockedInode) () {
	s.in <- req_InodeCache_FlushInode{rip}
	<-s.out
	return
}
func (s *inodeCache) IsDeviceBusy(devno int) (bool) {
	s.in <- req_InodeCache_IsDeviceBusy{devno}
	result := (<-s.out).(res_InodeCache_IsDeviceBusy)
	return result.Arg0
}
func (s *inodeCache) Close() (error) {
	s.in <- req_InodeCache_Close{}
	result := (<-s.out).(res_InodeCache_Close)
	return result.Arg0
}

type server_InodeCache struct {
	in chan reqInodeCache
	out chan resInodeCache
}
