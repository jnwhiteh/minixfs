package minixfs

import (
	"testing"
)

// Test the various 'Proc' level system calls. Since we want the filesystem to
// ideally be the same when we're done testing as when we started, we will
// order the tests appropriately.
//
// Creat
// Open
// Unlink
// Mkdir
// Rmdir
// Chdir
//
func Test_Proc_Syscalls(test *testing.T) {
	_Test_Creat_Syscall(test) // create several new files
	_Test_Unlink_Syscall(test) // remove the newly created files
}

type fileEntry struct {
	filename string
	inode uint
}

var fileList = []fileEntry{
	{"/tmp/foo0.txt", 546},
	{"/sample/europarl-br.txt", 547},
	{"/var/run/myapp.pid", 549},
	{"/etc/myapp.conf", 550},
}

func _Test_Creat_Syscall(test *testing.T) {
	fs, proc := OpenMinix3(test)
	for _, entry := range fileList {
		fname := entry.filename
		file, err := proc.Open(fname, O_CREAT, 0666)
		if err != nil {
			test.Fatalf("Could not create file %s: %s", fname, err)
		}
		if file.inode.inum != entry.inode {
			test.Errorf("Inode mismatch, expected %d, got %d", entry.inode, file.inode.inum)
		}
		file.Close()
	}

	fs.Close()
}

func _Test_Unlink_Syscall(test *testing.T) {
	fs, proc := OpenMinix3(test)
	for _, entry := range fileList {
		fname := entry.filename
		err := proc.Unlink(fname)
		if err != nil {
			test.Fatalf("Could not unlink file %s: %s", fname, err)
		}
	}

	fs.Close()
}
