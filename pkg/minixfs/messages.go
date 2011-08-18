package minixfs

import (
	"os"
)

//////////////////////////////////////////////////////////////////////////////
// Messages for Device interface
//////////////////////////////////////////////////////////////////////////////

type m_dev_req struct {
	call CallNumber
	buf  interface{}
	pos  int64
}

type m_dev_res struct {
	err os.Error
}

//////////////////////////////////////////////////////////////////////////////
// Messages for BlockCache interface
//////////////////////////////////////////////////////////////////////////////

type m_cache_req interface {
	is_m_cache_req()
}

type m_cache_res interface {
	is_m_cache_res()
}

type m_cache_req_mount struct {
	devno int
	dev   BlockDevice
	super *Superblock
}

type m_cache_req_get struct {
	devno       int
	bnum        int
	btype       BlockType
	only_search int
}

type m_cache_req_put struct {
	cb    *CacheBlock
	btype BlockType
}

type m_cache_req_unmount struct{ dev int }
type m_cache_req_invalidate struct{ dev int }
type m_cache_req_flush struct{ dev int }

type m_cache_res_err struct {
	err os.Error
}

type m_cache_res_get struct {
	cb *CacheBlock
}

type m_cache_res_empty struct{}

func (req m_cache_req_mount) is_m_cache_req()      {}
func (req m_cache_req_get) is_m_cache_req()        {}
func (req m_cache_req_put) is_m_cache_req()        {}
func (req m_cache_req_unmount) is_m_cache_req()    {}
func (req m_cache_req_invalidate) is_m_cache_req() {}
func (req m_cache_req_flush) is_m_cache_req()      {}

func (res m_cache_res_err) is_m_cache_res()   {}
func (res m_cache_res_get) is_m_cache_res()   {}
func (res m_cache_res_empty) is_m_cache_res() {}

// type assertions for devices
var _ m_cache_req = m_cache_req_mount{}
var _ m_cache_req = m_cache_req_get{}
var _ m_cache_req = m_cache_req_put{}
var _ m_cache_req = m_cache_req_unmount{}
var _ m_cache_req = m_cache_req_invalidate{}
var _ m_cache_req = m_cache_req_flush{}

var _ m_cache_res = m_cache_res_err{}
var _ m_cache_res = m_cache_res_get{}
var _ m_cache_res = m_cache_res_empty{}

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
type m_fs_req_close struct {}
type m_fs_req_mount struct {
	dev BlockDevice
	path string
}
type m_fs_req_unmount struct {
	dev BlockDevice
}
type m_fs_req_spawn struct {
	umask uint16
	rootpath string
}

// Response messages
type m_fs_res_err struct {
	err os.Error
}
type m_fs_res_spawn struct {
	proc *Process
	err os.Error
}

// For type-checking
func (m m_fs_req_close) is_m_fs_req() { }
func (m m_fs_req_mount) is_m_fs_req() { }
func (m m_fs_req_unmount) is_m_fs_req() { }
func (m m_fs_req_spawn) is_m_fs_req() { }

func (m m_fs_res_err) is_m_fs_res() { }
func (m m_fs_res_spawn) is_m_fs_res() { }

// Check interface implementation
var _ m_fs_req = m_fs_req_close{}
var _ m_fs_req = m_fs_req_mount{}
var _ m_fs_req = m_fs_req_unmount{}
var _ m_fs_req = m_fs_req_spawn{}

var _ m_fs_res = m_fs_res_err{}
var _ m_fs_res = m_fs_res_spawn{}
