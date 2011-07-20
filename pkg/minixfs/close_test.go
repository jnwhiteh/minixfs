package minixfs

import (
	"testing"
)

func Test_Close_Syscall(test *testing.T) {
	fs, proc := OpenMinix3(test)
	file, err := proc.Open("/sample/europarl-en.txt", O_RDONLY, 066)
	if err != nil {
		test.Errorf("Failed to open sample file")
	}

	filp := file.filp
	fd := file.fd

	err = file.Close()
	if err != nil {
		test.Errorf("Failed when closing file: %s", err)
	}

	if filp.Count() != 0 {
		test.Errorf("Filp entry show count > 0: %d", filp.Count())
	}
	if file.proc != nil {
		test.Errorf("File.proc is non-nil")
	}
	if proc._filp[fd] != nil {
		test.Errorf("Filp[%d] is non-nil", fd)
	}
	if proc._files[fd] != nil {
		test.Errorf("Files[%d] is non-nil", fd)
	}

	// Intentionally close it again to trigger the error
	err = file.Close()
	if err != EBADF {
		test.Errorf("Expected %s, got %s", EBADF, err)
	}

	proc.Exit()
	if err := fs.Close(); err != nil {
		test.Errorf("Failed when closing filesystem: %s", err)
	}
}

func Test_Exit_Syscall(test *testing.T) {
	fs, proc := OpenMinix3(test)
	// Open several files, in this case the same file
	files := make([]*File, 0, 0)
	fds := make([]int, 0, 0)

	for i := 0; i < 5; i++ {
		file, err := proc.Open("/sample/europarl-en.txt", O_RDONLY, 066)
		if err != nil {
			test.Errorf("Failed to open sample file")
		}
		files = append(files, file)
		fds = append(fds, file.fd)
	}

	err := fs.Close()
	if err != EBUSY {
		test.Errorf("Expected %s, got %s", EBUSY, err)
	}

	proc.Exit()

	for idx, file := range files {
		fd := fds[idx]
		if proc._filp[fd] != nil {
			test.Errorf("Filp[%d] is non-nil", fd)
		}
		if proc._files[fd] != nil {
			test.Errorf("Files[%s] is non-nil", fd)
		}
		if file.fd != NO_FILE {
			test.Errorf("Fd mismatch for file %d, expected %d, got %d", idx, NO_FILE, file.fd)
		}
	}

	if err := fs.Close(); err != nil {
		test.Errorf("Failed when closing filesystem: %s", err)
	}
}
