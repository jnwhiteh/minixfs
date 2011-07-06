package minixfs

import (
	"testing"
)

func _Test_Read_Europarl(fs *FileSystem, proc *Process, europarl []byte, test *testing.T) {
	file, err := proc.Open("/sample/europarl-en.txt", O_RDONLY, 0666)
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
}

func TestReadSyscall(test *testing.T) {
	fs, proc := OpenMinix3(test)
	odata := GetEuroparlData(test)

	_Test_Read_Europarl(fs, proc, odata, test)

	fs.Close()
}