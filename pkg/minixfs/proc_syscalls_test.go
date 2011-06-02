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
}

var fileList = []string{
	"/tmp/foo0.txt",
	"/sample/europarl-br.txt",
	"/var/run/myapp.pid",
	"/etc/myapp.conf",
}

func _Test_Creat_Syscall(test *testing.T) {
	fs, proc := OpenMinix3(test)
	for _, fname := range fileList {
		file, err := proc.Open(fname, O_CREAT, 0666)
		if err != nil {
			test.Fatalf("Could not create file %s: %s", fname, err)
		}
		file.Close()
	}

	fs.Close()
}
