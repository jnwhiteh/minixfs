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
type m_cache_req_reserve struct {
	dev  int
	bnum int
}
type m_cache_req_claim struct {
	dev   int
	bnum  int
	btype BlockType
}
type m_cache_req_invalidate struct{ dev int }
type m_cache_req_flush struct{ dev int }
type m_cache_req_close struct{}

type m_cache_res_err struct {
	err os.Error
}
type m_cache_res_block struct {
	cb *CacheBlock
}
type m_cache_res_async_block struct {
	ch chan m_cache_res_block
}
type m_cache_res_reserve struct {
	avail bool
}
type m_cache_res_empty struct{}

func (req m_cache_req_mount) is_m_cache_req()      {}
func (req m_cache_req_get) is_m_cache_req()        {}
func (req m_cache_req_put) is_m_cache_req()        {}
func (req m_cache_req_unmount) is_m_cache_req()    {}
func (req m_cache_req_reserve) is_m_cache_req()    {}
func (req m_cache_req_claim) is_m_cache_req()      {}
func (req m_cache_req_invalidate) is_m_cache_req() {}
func (req m_cache_req_flush) is_m_cache_req()      {}
func (req m_cache_req_close) is_m_cache_req()      {}

func (res m_cache_res_err) is_m_cache_res()     {}
func (res m_cache_res_block) is_m_cache_res()   {}
func (res m_cache_res_async_block) is_m_cache_res()   {}
func (res m_cache_res_reserve) is_m_cache_res() {}
func (res m_cache_res_empty) is_m_cache_res()   {}

// type assertions for devices
var _ m_cache_req = m_cache_req_mount{}
var _ m_cache_req = m_cache_req_get{}
var _ m_cache_req = m_cache_req_put{}
var _ m_cache_req = m_cache_req_unmount{}
var _ m_cache_req = m_cache_req_reserve{}
var _ m_cache_req = m_cache_req_claim{}
var _ m_cache_req = m_cache_req_invalidate{}
var _ m_cache_req = m_cache_req_flush{}
var _ m_cache_req = m_cache_req_close{}

var _ m_cache_res = m_cache_res_err{}
var _ m_cache_res = m_cache_res_block{}
var _ m_cache_res = m_cache_res_async_block{}
var _ m_cache_res = m_cache_res_reserve{}
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
type m_fs_req_shutdown struct{}
type m_fs_req_mount struct {
	dev  BlockDevice
	path string
}
type m_fs_req_unmount struct {
	dev BlockDevice
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

//////////////////////////////////////////////////////////////////////////////
// Messages for Finode
//////////////////////////////////////////////////////////////////////////////

type m_finode_req interface {
	is_m_finode_req()
}

type m_finode_res interface {
	is_m_finode_res()
}

// Request types
type m_finode_req_read struct {
	buf []byte
	pos int
}

type m_finode_req_write struct {
	buf []byte
	pos int
}

type m_finode_req_close struct{}

// Response types
type m_finode_res_io struct {
	n   int
	err os.Error
}

type m_finode_res_asyncio struct {
	callback <-chan m_finode_res_io
}

type m_finode_res_empty struct{}

// For type-checking
func (m m_finode_req_read) is_m_finode_req()  {}
func (m m_finode_req_write) is_m_finode_req() {}
func (m m_finode_req_close) is_m_finode_req() {}

func (m m_finode_res_io) is_m_finode_res()      {}
func (m m_finode_res_asyncio) is_m_finode_res() {}
func (m m_finode_res_empty) is_m_finode_res()   {}

// Check interface implementation
var _ m_finode_req = m_finode_req_read{}
var _ m_finode_req = m_finode_req_write{}
var _ m_finode_req = m_finode_req_close{}

var _ m_finode_res = m_finode_res_io{}
var _ m_finode_res = m_finode_res_asyncio{}
var _ m_finode_res = m_finode_res_empty{}

//////////////////////////////////////////////////////////////////////////////
// Messages for Superblock
//////////////////////////////////////////////////////////////////////////////

type m_super_req interface {
	is_m_super_req()
}

type m_super_res interface {
	is_m_super_res()
}

// Request types
type m_super_req_search struct {
	bmap uint
}
type m_super_req_setsearch struct {
	bmap uint
	num uint
}
type m_super_req_alloc_bit struct {
	bmap uint
	origin uint
}
type m_super_req_free_bit struct {
	bmap uint
	bnum uint
}
type m_super_req_alloc_zone struct {
	zone int
}
type m_super_req_free_zone struct {
	zone uint
}
type m_super_req_shutdown struct {}

// Response types
type m_super_res_search struct {
	next uint
}
type m_super_res_empty struct {}
type m_super_res_alloc_bit struct {
	bit uint
}
type m_super_res_alloc_zone struct {
	zone int
	err os.Error
}

// For type-checking
func (m m_super_req_search) is_m_super_req() {}
func (m m_super_req_setsearch) is_m_super_req() {}
func (m m_super_req_alloc_bit) is_m_super_req() {}
func (m m_super_req_free_bit) is_m_super_req() {}
func (m m_super_req_alloc_zone) is_m_super_req() {}
func (m m_super_req_free_zone) is_m_super_req() {}
func (m m_super_req_shutdown) is_m_super_req() {}

func (m m_super_res_search) is_m_super_res() {}
func (m m_super_res_empty) is_m_super_res() {}
func (m m_super_res_alloc_bit) is_m_super_res() {}
func (m m_super_res_alloc_zone) is_m_super_res() {}

// Type assertions
var _ m_super_req = m_super_req_search{}
var _ m_super_req = m_super_req_setsearch{}
var _ m_super_req = m_super_req_alloc_bit{}
var _ m_super_req = m_super_req_free_bit{}
var _ m_super_req = m_super_req_alloc_zone{}
var _ m_super_req = m_super_req_free_zone{}
var _ m_super_req = m_super_req_shutdown{}

var _ m_super_res = m_super_res_search{}
var _ m_super_res = m_super_res_empty{}
var _ m_super_res = m_super_res_alloc_bit{}
var _ m_super_res = m_super_res_alloc_zone{}
