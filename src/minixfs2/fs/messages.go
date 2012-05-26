package fs

import (
	. "minixfs2/common"
)

type req_FS_Mount struct {
	proc *Process
	dev  BlockDevice
	path string
}
type res_FS_Mount struct {
	Arg0 error
}
type req_FS_Unmount struct {
	proc *Process
	path string
}
type res_FS_Unmount struct {
	Arg0 error
}
type req_FS_Sync struct {
}
type res_FS_Sync struct{}
type req_FS_Shutdown struct {
}
type res_FS_Shutdown struct {
	Arg0 error
}
type req_FS_Fork struct {
	proc *Process
}
type res_FS_Fork struct {
	Arg0 *Process
	Arg1 error
}
type req_FS_Exit struct {
	proc *Process
}
type res_FS_Exit struct{}
type req_FS_Open struct {
	proc  *Process
	path  string
	flags int
	mode  uint16
}
type res_FS_Open struct {
	Arg0 *Fd
	Arg1 error
}
type req_FS_Creat struct {
	proc  *Process
	path  string
	flags int
	mode  uint16
}
type res_FS_Creat struct {
	Arg0 *Fd
	Arg1 error
}
type req_FS_Close struct {
	proc *Process
	fs   *Fd
}
type res_FS_Close struct {
	Arg0 error
}
type req_FS_Stat struct {
	proc *Process
	path string
}
type res_FS_Stat struct {
	Arg0 *StatInfo
	Arg1 error
}
type req_FS_Chmod struct {
	proc *Process
	path string
	mode uint16
}
type res_FS_Chmod struct {
	Arg0 error
}
type req_FS_Link struct {
	proc             *Process
	oldpath, newpath string
}
type res_FS_Link struct {
	Arg0 error
}
type req_FS_Unlink struct {
	proc *Process
	path string
}
type res_FS_Unlink struct {
	Arg0 error
}
type req_FS_Mkdir struct {
	proc *Process
	path string
}
type res_FS_Mkdir struct {
	Arg0 error
}
type req_FS_Rmdir struct {
	proc *Process
	path string
}
type res_FS_Rmdir struct {
	Arg0 error
}
type req_FS_Chdir struct {
	proc *Process
	path string
}
type res_FS_Chdir struct {
	Arg0 error
}

// Interface types and implementations
type reqFS interface {
	is_reqFS()
}
type resFS interface {
	is_resFS()
}

func (r req_FS_Mount) is_reqFS()    {}
func (r res_FS_Mount) is_resFS()    {}
func (r req_FS_Unmount) is_reqFS()  {}
func (r res_FS_Unmount) is_resFS()  {}
func (r req_FS_Sync) is_reqFS()     {}
func (r res_FS_Sync) is_resFS()     {}
func (r req_FS_Shutdown) is_reqFS() {}
func (r res_FS_Shutdown) is_resFS() {}
func (r req_FS_Fork) is_reqFS()     {}
func (r res_FS_Fork) is_resFS()     {}
func (r req_FS_Exit) is_reqFS()     {}
func (r res_FS_Exit) is_resFS()     {}
func (r req_FS_Open) is_reqFS()     {}
func (r res_FS_Open) is_resFS()     {}
func (r req_FS_Creat) is_reqFS()    {}
func (r res_FS_Creat) is_resFS()    {}
func (r req_FS_Close) is_reqFS()    {}
func (r res_FS_Close) is_resFS()    {}
func (r req_FS_Stat) is_reqFS()     {}
func (r res_FS_Stat) is_resFS()     {}
func (r req_FS_Chmod) is_reqFS()    {}
func (r res_FS_Chmod) is_resFS()    {}
func (r req_FS_Link) is_reqFS()     {}
func (r res_FS_Link) is_resFS()     {}
func (r req_FS_Unlink) is_reqFS()   {}
func (r res_FS_Unlink) is_resFS()   {}
func (r req_FS_Mkdir) is_reqFS()    {}
func (r res_FS_Mkdir) is_resFS()    {}
func (r req_FS_Rmdir) is_reqFS()    {}
func (r res_FS_Rmdir) is_resFS()    {}
func (r req_FS_Chdir) is_reqFS()    {}
func (r res_FS_Chdir) is_resFS()    {}

// Type check request/response types
var _ reqFS = req_FS_Mount{}
var _ resFS = res_FS_Mount{}
var _ reqFS = req_FS_Unmount{}
var _ resFS = res_FS_Unmount{}
var _ reqFS = req_FS_Sync{}
var _ resFS = res_FS_Sync{}
var _ reqFS = req_FS_Shutdown{}
var _ resFS = res_FS_Shutdown{}
var _ reqFS = req_FS_Fork{}
var _ resFS = res_FS_Fork{}
var _ reqFS = req_FS_Exit{}
var _ resFS = res_FS_Exit{}
var _ reqFS = req_FS_Open{}
var _ resFS = res_FS_Open{}
var _ reqFS = req_FS_Creat{}
var _ resFS = res_FS_Creat{}
var _ reqFS = req_FS_Close{}
var _ resFS = res_FS_Close{}
var _ reqFS = req_FS_Stat{}
var _ resFS = res_FS_Stat{}
var _ reqFS = req_FS_Chmod{}
var _ resFS = res_FS_Chmod{}
var _ reqFS = req_FS_Link{}
var _ resFS = res_FS_Link{}
var _ reqFS = req_FS_Unlink{}
var _ resFS = res_FS_Unlink{}
var _ reqFS = req_FS_Mkdir{}
var _ resFS = res_FS_Mkdir{}
var _ reqFS = req_FS_Rmdir{}
var _ resFS = res_FS_Rmdir{}
var _ reqFS = req_FS_Chdir{}
var _ resFS = res_FS_Chdir{}
