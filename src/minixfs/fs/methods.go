package fs

import (
	. "minixfs/common"
)

func (s *FileSystem) Mount(proc *Process, dev BlockDevice, path string) error {
	s.in <- req_FS_Mount{proc, dev, path}
	result := (<-s.out).(res_FS_Mount)
	return result.Arg0
}
func (s *FileSystem) Unmount(proc *Process, path string) error {
	s.in <- req_FS_Unmount{proc, path}
	result := (<-s.out).(res_FS_Unmount)
	return result.Arg0
}
func (s *FileSystem) Sync() {
	s.in <- req_FS_Sync{}
	<-s.out
	return
}
func (s *FileSystem) Shutdown() error {
	s.in <- req_FS_Shutdown{}
	result := (<-s.out).(res_FS_Shutdown)
	return result.Arg0
}
func (s *FileSystem) Fork(proc *Process) (*Process, error) {
	s.in <- req_FS_Fork{proc}
	result := (<-s.out).(res_FS_Fork)
	return result.Arg0, result.Arg1
}
func (s *FileSystem) Exit(proc *Process) {
	s.in <- req_FS_Exit{proc}
	<-s.out
	return
}
func (s *FileSystem) Open(proc *Process, path string, flags int, mode uint16) (Fd, error) {
	s.in <- req_FS_OpenCreat{proc, path, flags, mode}
	result := (<-s.out).(res_FS_OpenCreat)
	return result.Arg0, result.Arg1
}
func (s *FileSystem) Creat(proc *Process, path string, flags int, mode uint16) (Fd, error) {
	s.in <- req_FS_OpenCreat{proc, path, flags, mode}
	result := (<-s.out).(res_FS_OpenCreat)
	return result.Arg0, result.Arg1
}
func (s *FileSystem) Close(proc *Process, fd Fd) error {
	proc.fs.in <- req_FS_Close{proc, fd}
	result := (<-proc.fs.out).(res_FS_Close)
	return result.Arg0
}
func (s *FileSystem) Stat(proc *Process, path string) (*StatInfo, error) {
	s.in <- req_FS_Stat{proc, path}
	result := (<-s.out).(res_FS_Stat)
	return result.Arg0, result.Arg1
}
func (s *FileSystem) Chmod(proc *Process, path string, mode uint16) error {
	s.in <- req_FS_Chmod{proc, path, mode}
	result := (<-s.out).(res_FS_Chmod)
	return result.Arg0
}
func (s *FileSystem) Link(proc *Process, oldpath, newpath string) error {
	s.in <- req_FS_Link{proc, oldpath, newpath}
	result := (<-s.out).(res_FS_Link)
	return result.Arg0
}
func (s *FileSystem) Unlink(proc *Process, path string) error {
	s.in <- req_FS_Unlink{proc, path}
	result := (<-s.out).(res_FS_Unlink)
	return result.Arg0
}
func (s *FileSystem) Mkdir(proc *Process, path string, mode uint16) error {
	s.in <- req_FS_Mkdir{proc, path, mode}
	result := (<-s.out).(res_FS_Mkdir)
	return result.Arg0
}
func (s *FileSystem) Rmdir(proc *Process, path string) error {
	s.in <- req_FS_Rmdir{proc, path}
	result := (<-s.out).(res_FS_Rmdir)
	return result.Arg0
}
func (s *FileSystem) Chdir(proc *Process, path string) error {
	s.in <- req_FS_Chdir{proc, path}
	result := (<-s.out).(res_FS_Chdir)
	return result.Arg0
}
