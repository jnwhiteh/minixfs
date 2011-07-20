package minixfs

import (
	"sync"
	"testing"
	"time"
)

// Test to ensure that a function call completed within 'delay' nanonseconds,
// after the mutex 'm' has been acquired. You can test the 'read' side of a
// RWMutex by calling m.RLocker() and passing that in as 'm'.
func testMutex(name string, m sync.Locker, delay int64, test *testing.T, fn func()) bool {
	done := make(chan bool)

	if m != nil {
		m.Lock()
	}

	// Run the function
	go func() {
		fn()
		done <- true
	}()

	timeout := time.After(delay)

	var result bool

	select {
	case <-timeout:
		result = false
	case <-done:
		go func() {
			<-timeout
			close(timeout)
		}()
		result = true
	}

	if m != nil {
		m.Unlock()
	}
	return result
}

// Test the system-call level locks to ensure they function properly. The
// following system calls should be able to happen in any order:
//
// FileSystem.NewProcess
// Process.Chdir
// Process.Mkdir
// Process.Open
// Process.Rmdir
// Process.Unlink
// File.Close
// File.Read
// File.Seek
// File.Write
//
// These must acquire the write lock
//
// FileSystem.Mount
// FileSystem.Unmount
// FileSystem.Close
func Test_Syscall_Locks(test *testing.T) {
	filename := "/tmp/locktest.txt"
	fs, proc := OpenMinix3(test)
	file, err := proc.Open(filename, O_CREAT, 0666)
	if err != nil {
		test.Errorf("Failed when opening %s: %s", filename, err)
	}

	type Test struct {
		name     string      // the name of the test
		mutex    sync.Locker // the mutex to test (use RLocker to get read side)
		expected bool        // should the call complete?
		fn       func()      // the function to execute
	}

	tests := []Test{
		Test{"FileSystem.NewProcess", fs.m.device.RLocker(), true,
			func() {
				fs.NewProcess(1, 0, "/")
			},
		},
		Test{"Process.Mkdir", fs.m.device.RLocker(), true,
			func() {
				proc.Mkdir("/path/does/not/exist", 066)
			},
		},
		Test{"Process.Chdir", fs.m.device.RLocker(), true,
			func() {
				proc.Chdir("/path/does/not/exist")
			},
		},
		Test{"Process.Rmdir", fs.m.device.RLocker(), true,
			func() {
				proc.Rmdir("/path/does/not/exist")
			},
		},
		Test{"Process.Open", fs.m.device.RLocker(), true,
			func() {
				proc.Open("/path/does/not/exist", O_RDONLY, 0666)
			},
		},
		Test{"Process.Unlink", fs.m.device.RLocker(), true,
			func() {
				proc.Unlink("/path/does/not/exist")
			},
		},
		Test{"File.Read", fs.m.device.RLocker(), true,
			func() {
				buf := make([]byte, 0, 0)
				file.Read(buf)
			},
		},
		Test{"File.Write", fs.m.device.RLocker(), true,
			func() {
				buf := []byte("Hello World!")
				file.Write(buf)
			},
		},
		Test{"File.Seek", fs.m.device.RLocker(), true,
			func() {
				file.Seek(0, 0)
			},
		},
		Test{"File.Close", fs.m.device.RLocker(), true,
			func() {
				file.Close()
			},
		},
		Test{"FileSystem.Mount", fs.m.device.RLocker(), false,
			func() {
				fs.Mount(nil, "/path/does/not/exist")
			},
		},
		Test{"FileSystem.Unmount", fs.m.device.RLocker(), false,
			func() {
				fs.Unmount(nil)
			},
		},
		Test{"Cleanup phase", nil, true,
			func() {
				// Unlink the file so the filesystem doesn't change
				err := proc.Unlink(filename)
				if err != nil {
					test.Errorf("Failed when unlinking %s: %s", filename, err)
				}
			},
		},
		Test{"FileSystem.Close", fs.m.device.RLocker(), false,
			func() {
				proc.Exit()
				if err := fs.Close(); err != nil {
					test.Errorf("Failed when closing filesystem: %s", err)
				}
			},
		},
	}

	var delay int64 = 1e9 // 5 seconds
	for _, data := range tests {
		if testMutex(data.name, data.mutex, delay, test, data.fn) != data.expected {
			if data.expected {
				test.Errorf("Lock test failed for '%s', call did not complete within %d seconds", data.name, delay/1e9)
			} else {
				test.Errorf("Lock test failed for '%s', call was not expected to complete", data.name)
			}
		}
	}

	// After running tests, file should be closed, unlinked and fs closed
}
