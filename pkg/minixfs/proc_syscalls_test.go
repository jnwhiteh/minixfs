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
	_Test_Creat_Syscall(test)  // create several new files
	_Test_Unlink_Syscall(test) // remove the newly created files
	_Test_Mkdir_Syscall(test)  // create several new directories
	_Test_Rmdir_Syscall(test)  // remove the newly created directories
}

type fileEntry struct {
	filename string
	inode    uint
}

var fileList = []fileEntry{
	{"/tmp/foo0.txt", 546},
	{"/sample/europarl-br.txt", 547},
	{"/var/run/myapp.pid", 549},
	{"/etc/myapp.conf", 550},
}

func _Test_Creat_Syscall(test *testing.T) {
	test.Log("_Test_Creat_Syscall")
	fs, proc := OpenMinix3(test)
	for _, entry := range fileList {
		fname := entry.filename
		file, err := proc.Open(fname, O_CREAT, 0666)
		if err != nil {
			test.Errorf("Could not create file %s: %s", fname, err)
		}
		if file.inode.inum != entry.inode {
			test.Errorf("Inode mismatch, expected %d, got %d", entry.inode, file.inode.inum)
		}
		file.Close()
	}

	// Check to make sure I_Search is set correctly (to 550)
	super := fs.supers[ROOT_DEVICE]
	if super.I_Search != 550 {
		test.Errorf("I_Search mismatch: expected %d, got %d", 550, super.I_Search)
	}

	fs.Close()
}

func _Test_Unlink_Syscall(test *testing.T) {
	test.Log("_Test_Unlink_Syscall")
	fs, proc := OpenMinix3(test)
	for _, entry := range fileList {
		fname := entry.filename
		err := proc.Unlink(fname)
		if err != nil {
			test.Fatalf("Could not unlink file %s: %s", fname, err)
		}

		// Ensure the bit in the IMAP was properly de-allocated
		if fs.check_bit(ROOT_DEVICE, IMAP, entry.inode) {
			test.Errorf("Inode bit %d not properly deallocated", entry.inode)
		}
	}

	fs.Close()
}

type dirEntry struct {
	name string
	num  uint
	size int32
}

var dirList = []dirEntry{
	{"/tmp/alpha", 546, 192},
	{"/tmp/alpha/beta", 547, 192},
	{"/tmp/alpha/beta/gamma", 549, 192},
	{"/tmp/alpha/beta/gamma/delta", 550, 128},
}

func _Test_Mkdir_Syscall(test *testing.T) {
	test.Log("_Test_Mkdir_Syscall")
	fs, proc := OpenMinix3(test)
	for _, entry := range dirList {
		dirname := entry.name
		err := proc.Mkdir(dirname, 0666)
		if err != nil {
			test.Errorf("Could not mkdir %s: %s", dirname, err)
		}
	}

	// Now check to see if they were created corectly
	for _, entry := range dirList {
		dirname := entry.name
		// Run and get that inode
		inode, err := fs.eat_path(proc, dirname)
		if err != nil || inode == nil {
			test.Errorf("Failed to get new inode for %s: %s", dirname, err)
		} else {
			if inode.inum != entry.num {
				test.Errorf("Inode mismatch: expected %d, got %d", entry.num, inode.inum)
			}
			if inode.Size != entry.size {
				test.Errorf("Size mismatch: expected %d, got %d", entry.size, inode.Size)
			}
		}
	}

	// Check to make sure I_Search is set correctly (to 550)
	super := fs.supers[ROOT_DEVICE]
	if super.I_Search != 550 {
		test.Errorf("I_Search mismatch: expected %d, got %d", 550, super.I_Search)
	}

	fs.Close()
}

func _Test_Rmdir_Syscall(test *testing.T) {
	test.Log("_Test_Rmdir_Syscall")
	fs, proc := OpenMinix3(test)

	// Directories must be removed in reverse order
	for i := len(dirList) - 1; i >= 0; i-- {
		dirname := dirList[i].name
		err := proc.Rmdir(dirname)
		if err != nil {
			test.Errorf("Could not rmdir %s: %s", dirname, err)
		}

		// Ensure the bit in the IMAP was properly de-allocated
		if fs.check_bit(ROOT_DEVICE, IMAP, dirList[i].num) {
			test.Errorf("Inode bit %d not properly deallocated", dirList[i].num)
		}
	}

	for _, entry := range dirList {
		dirname := entry.name
		// Run and get that inode
		_, err := fs.eat_path(proc, dirname)
		if err != ENOENT {
			test.Errorf("Error when looking up %s, expected '%s', got '%s'", dirname, ENOENT, err)
		}
	}

	fs.Close()
}
