package minixfs

import (
	"testing"
)

// Create a new file, and write the data from
func _Test_Write_New(fs *FileSystem, proc *Process, europarl []byte, test *testing.T) {
	test.Log("_Test_Write_New")

	numBytes := len(europarl)

	// Open/Create the new file
	file, err := proc.Open("/tmp/europarl-en.txt", O_RDWR|O_CREAT, 0666)
	n, err := file.Write(europarl[:numBytes])
	if n != numBytes {
		test.Errorf("Bytes written mismatch, got %d, expected %d", n, len(europarl))
	}
	if err != nil {
		test.Errorf("Got an error while writing: %s", err)
	}
	if int(file.inode.Size) != n {
		test.Errorf("File size mismatch, got %d, expected %d", file.inode.Size, n)
	}
	file.Close()
}

func _Test_Verify_Write(fs *FileSystem, proc *Process, europarl []byte, test *testing.T) {
	test.Log("_Test_Verify_Write")

	file, err := proc.Open("/tmp/europarl-en.txt", O_RDONLY, 0666)
	if file == nil || err != nil {
		test.Errorf("Failed opening file: %s", err)
	}
	size := int(file.inode.Size)

	file.Seek(0, 0)
	data := make([]byte, size)
	rn, err := file.Read(data)
	if rn != size {
		test.Errorf("Failed when reading back, got %d, expected %d", rn, size)
	}
	for i := 0; i < size; i++ {
		if europarl[i] != data[i] {
			min := i - 25
			if min < 0 {
				min = 0
			}
			max := i + 25
			if max > size {
				max = size
			}
			otxt := europarl[min:max]
			gtxt := data[min:max]
			test.Errorf("Data mismatch at position %d\n::orig::%s\n::got::%s", i, otxt, gtxt)
			break
		}
	}

	// Clean things up
	file.Close()
}

func TestWriteSyscall(test *testing.T) {
	fs, proc := OpenMinix3(test)
	proc.Unlink("/tmp/europarl-en.txt")

	odata := GetEuroparlData(test)

	_Test_Write_New(fs, proc, odata, test)
	_Test_Verify_Write(fs, proc, odata, test)

	proc.Unlink("/tmp/europarl-en.txt")
	fs.Close()
}
