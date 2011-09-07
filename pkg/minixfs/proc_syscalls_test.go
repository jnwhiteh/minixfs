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
	_Test_Chdir_Syscall(test)  // move current directory around
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
		file, err := fs.Open(proc, fname, O_CREAT, 0666)
		if err != nil {
			test.Errorf("Could not create file %s: %s", fname, err)
		}
		if file.inode.inum != entry.inode {
			test.Errorf("Inode mismatch, expected %d, got %d", entry.inode, file.inode.inum)
		}
		fs.Close(proc, file)
	}

	// Check to make sure I_Search is set correctly (to 550)
	super := fs.supers[ROOT_DEVICE]
	if super.Search(IMAP) != 550 {
		test.Errorf("I_Search mismatch: expected %d, got %d", 550, super.Search(IMAP))
	}

	fs.Exit(proc)
	if err := fs.Shutdown(); err != nil {
		test.Errorf("Failed when closing filesystem: %s", err)
	}
}

func _Test_Unlink_Syscall(test *testing.T) {
	test.Log("_Test_Unlink_Syscall")
	fs, proc := OpenMinix3(test)
	for _, entry := range fileList {
		fname := entry.filename
		err := fs.Unlink(proc, fname)
		if err != nil {
			test.Fatalf("Could not unlink file %s: %s", fname, err)
		}

		// Ensure the bit in the IMAP was properly de-allocated
		if fs.supers[ROOT_DEVICE].check_bit(IMAP, entry.inode) {
			test.Errorf("Inode bit %d not properly deallocated", entry.inode)
		}
	}

	fs.Exit(proc)
	if err := fs.Shutdown(); err != nil {
		test.Errorf("Failed when closing filesystem: %s", err)
	}
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
		err := fs.Mkdir(proc, dirname, 0666)
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
			if inode.Size() != entry.size {
				test.Errorf("Size mismatch: expected %d, got %d", entry.size, inode.Size())
			}

			fs.put_inode(inode)
		}
	}

	// Check to make sure I_Search is set correctly (to 550)
	super := fs.supers[ROOT_DEVICE]
	if super.Search(IMAP) != 550 {
		test.Errorf("I_Search mismatch: expected %d, got %d", 550, super.Search(IMAP))
	}

	fs.Exit(proc)
	if err := fs.Shutdown(); err != nil {
		test.Errorf("Failed when closing filesystem: %s", err)
	}
}

func _Test_Rmdir_Syscall(test *testing.T) {
	test.Log("_Test_Rmdir_Syscall")
	fs, proc := OpenMinix3(test)

	// Directories must be removed in reverse order
	for i := len(dirList) - 1; i >= 0; i-- {
		dirname := dirList[i].name
		err := fs.Rmdir(proc, dirname)
		if err != nil {
			test.Errorf("Could not rmdir %s: %s", dirname, err)
		}

		// Ensure the bit in the IMAP was properly de-allocated
		if fs.supers[ROOT_DEVICE].check_bit(IMAP, dirList[i].num) {
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

	fs.Exit(proc)
	if err := fs.Shutdown(); err != nil {
		test.Errorf("Failed when closing filesystem: %s", err)
	}
}

func _Test_Chdir_Syscall(test *testing.T) {
	test.Log("_Test_Chdir_Syscall")
	fs, proc := OpenMinix3(test)

	// Ensure the working directory is root
	if proc.workdir.inum != ROOT_INODE {
		test.Errorf("Working directory is not ROOT_INODE: %d", proc.workdir.inum)
	}

	// chdir through each directory in dirList
	for _, entry := range dirList {
		dirname := entry.name
		err := fs.Chdir(proc, dirname)
		if err != nil {
			test.Error("Unexpected error: %s", err)
		}
		if proc.workdir.inum != entry.num {
			test.Errorf("Inode mismatch, expected %d, got %d", entry.num, proc.workdir.inum)
		}
	}

	// take the last directory and split it
	max := len(dirList) - 1
	for i := max; i > 0; i-- {
		err := fs.Chdir(proc, "..")
		if err != nil {
			test.Errorf("Failed changing to ..: %s", err)
		}
		if proc.workdir.inum != dirList[i-1].num {
			test.Errorf("Inode mismatch, expected %d, got %d", dirList[max-i+1].num, proc.workdir.inum)
		}
	}

	// Chdir down to /tmp and then to /
	for i := 0; i < 2; i++ {
		err := fs.Chdir(proc, "..")
		if err != nil {
			test.Errorf("Failed changing to ..: %s", err)
		}
	}

	if proc.workdir.inum != ROOT_INODE {
		test.Errorf("Failed to return to /tmp working directory")
	}

	fs.Exit(proc)
	if err := fs.Shutdown(); err != nil {
		test.Errorf("Failed when closing filesystem: %s", err)
	}
}
