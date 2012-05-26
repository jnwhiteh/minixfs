package inode

import (
	. "minixfs2/common"
)

type req_InodeTbl_MountDevice struct {
	devnum int
	info   *DeviceInfo
}
type res_InodeTbl_MountDevice struct{}
type req_InodeTbl_UnmountDevice struct {
	devnum int
}
type res_InodeTbl_UnmountDevice struct {
	Arg0 error
}
type req_InodeTbl_GetInode struct {
	devnum int
	inum   int
}
type res_InodeTbl_GetInode struct {
	Arg0 *Inode
	Arg1 error
}
type req_InodeTbl_DupInode struct {
	inode *Inode
}
type res_InodeTbl_DupInode struct {
	Arg0 *Inode
}
type req_InodeTbl_PutInode struct {
	inode *Inode
}
type res_InodeTbl_PutInode struct{}
type req_InodeTbl_FlushInode struct {
	inode *Inode
}
type res_InodeTbl_FlushInode struct{}
type req_InodeTbl_IsDeviceBusy struct {
	devnum int
}
type res_InodeTbl_IsDeviceBusy struct {
	Arg0 bool
}
type res_InodeTbl_Async struct {
	ch chan resInodeTbl
}

// Interface types and implementations
type reqInodeTbl interface {
	is_reqInodeTbl()
}
type resInodeTbl interface {
	is_resInodeTbl()
}

func (r req_InodeTbl_MountDevice) is_reqInodeTbl()   {}
func (r res_InodeTbl_MountDevice) is_resInodeTbl()   {}
func (r req_InodeTbl_UnmountDevice) is_reqInodeTbl() {}
func (r res_InodeTbl_UnmountDevice) is_resInodeTbl() {}
func (r req_InodeTbl_GetInode) is_reqInodeTbl()      {}
func (r res_InodeTbl_GetInode) is_resInodeTbl()      {}
func (r req_InodeTbl_DupInode) is_reqInodeTbl()      {}
func (r res_InodeTbl_DupInode) is_resInodeTbl()      {}
func (r req_InodeTbl_PutInode) is_reqInodeTbl()      {}
func (r res_InodeTbl_PutInode) is_resInodeTbl()      {}
func (r req_InodeTbl_FlushInode) is_reqInodeTbl()    {}
func (r res_InodeTbl_FlushInode) is_resInodeTbl()    {}
func (r req_InodeTbl_IsDeviceBusy) is_reqInodeTbl()  {}
func (r res_InodeTbl_IsDeviceBusy) is_resInodeTbl()  {}
func (r res_InodeTbl_Async) is_resInodeTbl()         {}

// Type check request/response types
var _ reqInodeTbl = req_InodeTbl_MountDevice{}
var _ resInodeTbl = res_InodeTbl_MountDevice{}
var _ reqInodeTbl = req_InodeTbl_UnmountDevice{}
var _ resInodeTbl = res_InodeTbl_UnmountDevice{}
var _ reqInodeTbl = req_InodeTbl_GetInode{}
var _ resInodeTbl = res_InodeTbl_GetInode{}
var _ reqInodeTbl = req_InodeTbl_DupInode{}
var _ resInodeTbl = res_InodeTbl_DupInode{}
var _ reqInodeTbl = req_InodeTbl_PutInode{}
var _ resInodeTbl = res_InodeTbl_PutInode{}
var _ reqInodeTbl = req_InodeTbl_FlushInode{}
var _ resInodeTbl = res_InodeTbl_FlushInode{}
var _ reqInodeTbl = req_InodeTbl_IsDeviceBusy{}
var _ resInodeTbl = res_InodeTbl_IsDeviceBusy{}
var _ resInodeTbl = res_InodeTbl_Async{}

func (s *server_InodeTbl) MountDevice(devnum int, info *DeviceInfo) {
	s.in <- req_InodeTbl_MountDevice{devnum, info}
	<-s.out
	return
}
func (s *server_InodeTbl) UnmountDevice(devnum int) error {
	s.in <- req_InodeTbl_UnmountDevice{devnum}
	result := (<-s.out).(res_InodeTbl_UnmountDevice)
	return result.Arg0
}
func (s *server_InodeTbl) GetInode(devnum int, inum int) (*Inode, error) {
	s.in <- req_InodeTbl_GetInode{inum, devnum}
	ares := (<-s.out).(res_InodeTbl_Async)
	result := (<-ares.ch).(res_InodeTbl_GetInode)
	return result.Arg0, result.Arg1
}
func (s *server_InodeTbl) DupInode(inode *Inode) *Inode {
	s.in <- req_InodeTbl_DupInode{inode}
	result := (<-s.out).(res_InodeTbl_DupInode)
	return result.Arg0
}
func (s *server_InodeTbl) PutInode(inode *Inode) {
	s.in <- req_InodeTbl_PutInode{inode}
	<-s.out
	return
}
func (s *server_InodeTbl) FlushInode(inode *Inode) {
	s.in <- req_InodeTbl_FlushInode{inode}
	<-s.out
	return
}
func (s *server_InodeTbl) IsDeviceBusy(devnum int) bool {
	s.in <- req_InodeTbl_IsDeviceBusy{devnum}
	result := (<-s.out).(res_InodeTbl_IsDeviceBusy)
	return result.Arg0
}