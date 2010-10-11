package minixfs

import "fmt"
import "os"
import "encoding/binary"

// ino_t		uint32
// zone1_t 		uint16
// zone_t		uint32
// off_t		int32

type Super_block struct {
	Ninodes           uint32 // # of usable inodes on the minor device
	Nzones            uint16 // total device size, including bit maps, etc.
	Imap_blocks       uint16 // # of blocks used by inode bit map
	Zmap_blocks       uint16 // # of blocks used by zone bit map
	Firstdatazone_old uint16 // number of first data zone
	Log_zone_size     uint16 // log2 of blocks/zone
	Pad               uint16 // try to avoid compiler-dependent padding
	Max_size          int32  // maximum file size on this device
	Zones             uint32 // number of zones (replaces s_nzones in V2+)
	Magic             uint16 // magic number to recognize super-blocks

	// The following fields are only present in V3 and above, which is
	// the standard version of Minix that we are implementing
	Pad2         uint16 // try to avoid compiler-dependent padding
	Block_size   uint16 // block size in bytes
	Disk_version byte   // filesystem format sub-version
}

// Read the superblock from the second 1024k block of the file
func Read_superblock(file *os.File) *Super_block {
	sup := new(Super_block)
	_, err := file.Seek(1024, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to seek to superblock: %s\n", err.String())
		os.Exit(-1)
	}
	err = binary.Read(file, binary.LittleEndian, sup)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read binary blob into superblock: %s\n", err.String())
		os.Exit(-1)
	}
	return sup
}
