package main

import "fmt"
import "os"
import "encoding/binary"

type super_block struct {
	Ninodes uint16			// # of usable inodes on the minor device
	Nzones uint16		// total device size, including bit maps, etc.
	Imap_blocks uint16		// # of blocks used by inode bit map
	Zmap_blocks uint16		// # of blocks used by zone bit map
	Firstdatazone uint16	// number of first data zone
	Log_zone_size uint16	// log2 of blocks/zone
	Max_size uint32		// maximum file size on this device
	Magic uint16			// magic number to recognize super-blocks
	State uint16			// filesystem state
	Zones uint32			// device size in blocks (v2)
	Unused [4]uint32
}

// Read the superblock from the second 1024k block of the file
func read_superblock(file *os.File) (*super_block) {
	sup := new(super_block)
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
