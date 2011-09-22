package minixfs

import (
	"os"
)

//////////////////////////////////////////////////////////////////////////////
// Messages for FileSystem
//////////////////////////////////////////////////////////////////////////////

type m_fs_req interface {
	is_m_fs_req()
}

type m_fs_res interface {
	is_m_fs_res()
}

// Request messages
type m_fs_req_shutdown struct{}
type m_fs_req_mount struct {
	dev  RandDevice
	path string
}
type m_fs_req_unmount struct {
	dev RandDevice
}
type m_fs_req_spawn struct {
	pid      int
	umask    uint16
	rootpath string
}
type m_fs_req_exit struct {
	proc *Process
}
type m_fs_req_open struct {
	proc  *Process
	path  string
	flags int
	mode  uint16
}
type m_fs_req_close struct {
	proc *Process
	file *File
}
type m_fs_req_unlink struct {
	proc *Process
	path string
}
type m_fs_req_mkdir struct {
	proc *Process
	path string
	mode uint16
}
type m_fs_req_rmdir struct {
	proc *Process
	path string
}
type m_fs_req_chdir struct {
	proc *Process
	path string
}
type m_fs_req_alloc_zone struct {
	dev    int
	zstart int
}
type m_fs_req_free_zone struct {
	dev  int
	zone int
}

// Response messages
type m_fs_res_err struct {
	err os.Error
}
type m_fs_res_spawn struct {
	proc *Process
	err  os.Error
}
type m_fs_res_open struct {
	file *File
	err  os.Error
}
type m_fs_res_empty struct{}
type m_fs_res_alloc_zone struct {
	zone int
	err  os.Error
}

// For type-checking
func (m m_fs_req_shutdown) is_m_fs_req()   {}
func (m m_fs_req_mount) is_m_fs_req()      {}
func (m m_fs_req_unmount) is_m_fs_req()    {}
func (m m_fs_req_spawn) is_m_fs_req()      {}
func (m m_fs_req_exit) is_m_fs_req()       {}
func (m m_fs_req_open) is_m_fs_req()       {}
func (m m_fs_req_close) is_m_fs_req()      {}
func (m m_fs_req_unlink) is_m_fs_req()     {}
func (m m_fs_req_mkdir) is_m_fs_req()      {}
func (m m_fs_req_rmdir) is_m_fs_req()      {}
func (m m_fs_req_chdir) is_m_fs_req()      {}
func (m m_fs_req_alloc_zone) is_m_fs_req() {}
func (m m_fs_req_free_zone) is_m_fs_req()  {}

func (m m_fs_res_err) is_m_fs_res()        {}
func (m m_fs_res_spawn) is_m_fs_res()      {}
func (m m_fs_res_open) is_m_fs_res()       {}
func (m m_fs_res_empty) is_m_fs_res()      {}
func (m m_fs_res_alloc_zone) is_m_fs_res() {}

// Check interface implementation
var _ m_fs_req = m_fs_req_shutdown{}
var _ m_fs_req = m_fs_req_mount{}
var _ m_fs_req = m_fs_req_unmount{}
var _ m_fs_req = m_fs_req_spawn{}
var _ m_fs_req = m_fs_req_exit{}
var _ m_fs_req = m_fs_req_open{}
var _ m_fs_req = m_fs_req_close{}
var _ m_fs_req = m_fs_req_unlink{}
var _ m_fs_req = m_fs_req_mkdir{}
var _ m_fs_req = m_fs_req_rmdir{}
var _ m_fs_req = m_fs_req_chdir{}
var _ m_fs_req = m_fs_req_alloc_zone{}
var _ m_fs_req = m_fs_req_free_zone{}

var _ m_fs_res = m_fs_res_err{}
var _ m_fs_res = m_fs_res_spawn{}
var _ m_fs_res = m_fs_res_open{}
var _ m_fs_res = m_fs_res_empty{}
var _ m_fs_res = m_fs_res_alloc_zone{}
