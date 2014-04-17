package fs

import (
	. "minixfs/common"
)

type Process struct {
	pid     int         // the numeric id of this process
	umask   uint16      // file creation mask
	rootdir *Inode      // root directory of the process
	workdir *Inode      // working directory of the process
	files   []*filp     // list of file descriptors
	fs      *FileSystem // the file system for this process
}

func (proc *Process) Mount(dev BlockDevice, path string) error {
	proc.fs.in <- req_FS_Mount{proc, dev, path}
	result := (<-proc.fs.out).(res_FS_Mount)
	return result.Arg0
}
func (proc *Process) Unmount(path string) error {
	proc.fs.in <- req_FS_Unmount{proc, path}
	result := (<-proc.fs.out).(res_FS_Unmount)
	return result.Arg0
}
func (proc *Process) Sync() {
	proc.fs.in <- req_FS_Sync{}
	<-proc.fs.out
	return
}
func (proc *Process) Shutdown() {
	proc.fs.in <- req_FS_Shutdown{}
	<-proc.fs.out
	return
}
func (proc *Process) Fork() (*Process, error) {
	proc.fs.in <- req_FS_Fork{proc}
	result := (<-proc.fs.out).(res_FS_Fork)
	return result.Arg0, result.Arg1
}
func (proc *Process) Exit() {
	proc.fs.in <- req_FS_Exit{proc}
	<-proc.fs.out
	return
}
func (proc *Process) Open(path string, flags int, mode uint16) (Fd, error) {
	proc.fs.in <- req_FS_OpenCreat{proc, path, flags, mode}
	result := (<-proc.fs.out).(res_FS_OpenCreat)
	return result.Arg0, result.Arg1
}
func (proc *Process) Creat(path string, flags int, mode uint16) (Fd, error) {
	proc.fs.in <- req_FS_OpenCreat{proc, path, flags | O_CREAT, mode}
	result := (<-proc.fs.out).(res_FS_OpenCreat)
	return result.Arg0, result.Arg1
}
func (proc *Process) Close(fd Fd) error {
	proc.fs.in <- req_FS_Close{proc, fd}
	result := (<-proc.fs.out).(res_FS_Close)
	return result.Arg0
}
func (proc *Process) Stat(path string) (*StatInfo, error) {
	proc.fs.in <- req_FS_Stat{proc, path}
	result := (<-proc.fs.out).(res_FS_Stat)
	return result.Arg0, result.Arg1
}
func (proc *Process) Chmod(path string, mode uint16) error {
	proc.fs.in <- req_FS_Chmod{proc, path, mode}
	result := (<-proc.fs.out).(res_FS_Chmod)
	return result.Arg0
}
func (proc *Process) Link(oldpath, newpath string) error {
	proc.fs.in <- req_FS_Link{proc, oldpath, newpath}
	result := (<-proc.fs.out).(res_FS_Link)
	return result.Arg0
}
func (proc *Process) Unlink(path string) error {
	proc.fs.in <- req_FS_Unlink{proc, path}
	result := (<-proc.fs.out).(res_FS_Unlink)
	return result.Arg0
}
func (proc *Process) Mkdir(path string, mode uint16) error {
	proc.fs.in <- req_FS_Mkdir{proc, path, mode}
	result := (<-proc.fs.out).(res_FS_Mkdir)
	return result.Arg0
}
func (proc *Process) Rmdir(path string) error {
	proc.fs.in <- req_FS_Rmdir{proc, path}
	result := (<-proc.fs.out).(res_FS_Rmdir)
	return result.Arg0
}
func (proc *Process) Chdir(path string) error {
	proc.fs.in <- req_FS_Chdir{proc, path}
	result := (<-proc.fs.out).(res_FS_Chdir)
	return result.Arg0
}
