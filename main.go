package main

import "fmt"
import "os"

type short uint16
type ushort int16
type long int32
type ulong uint32

type mode_t ushort
type uid_t short
type off_t long
type time_t ulong
type zone_t uint32
type block_t uint32

type d2_inode struct {				// V2.x disk inode
	d2_mode mode_t					// file type, protection, etc.
	d2_nlinks uint16				// how many links to this file. HACK!
	d2_uid uid_t					// user id of the file's owner
	d2_gid uint16					// group number. HACK!
	d2_size off_t					// current file size in bytes
	d2_atime time_t					// when was file data last accessed
	d2_mtime time_t					// when was file data last changed
	d2_ctime time_t					// when was inode data last changed
	d2_zone [V2_NR_TZONES]zone_t	// block nums for direct, indirect and double indirect
}

// Disk layout is:
// 
// 	Item				Number of Blocks
//	boot_block			1
// 	super_block			1 (offset 1kb)
//	inode_map			s_imap_blocks
//	zone map			s_zmap_blocks
//	inodes				(s_ninodes + 'inodes per block' - 1)/'inodes per block'
//	unused				whatever is needed to fill the current zone
//	data zones			(s_zones - s_firstdatazone) << s_log_zone_size

// This function will create a minix version 3 filesystem written directly
// to a file, to be used when emulating a block disk device
func main() {
	file, err := os.Open("diskimage.mfs", os.O_WRONLY | os.O_CREAT, 0744)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open disk image file: %s\n", err.String())
		os.Exit(-1)
	}
	defer file.Close()

	// Determine layout of disk image
	var block_size uint
	var inodes_per_block uint

	if block_size == 0 {
		block_size = MAX_BLOCK_SIZE
	}
	if block_size % SECTOR_SIZE != 0 || block_size < MIN_BLOCK_SIZE {
		fmt.Fprint(os.Stderr, "Block size must be a multiple of sector (%d) and at least %d bytes\n", SECTOR_SIZE, MIN_BLOCK_SIZE)
		os.Exit(-1)
	}
	if block_size % V2_INODE_SIZE != 0 {
		fmt.Fprintf(os.Stderr, "block size must be a multiple of inode size (%d bytes)\n", V2_INODE_SIZE)
		os.Exit(-1)
	}

	if inodes_per_block == 0 {
		inodes_per_block = block_size / V2_INODE_SIZE
	}

	zero := make([]byte, block_size, block_size)

	// maxblocks is the final commandline argument, hardcode it here to 1000 blocks.
	var maxblocks uint32 = 1000
	var nrblocks uint32 = maxblocks

	// inodes is a commandline argument -i, hardcode it here to 
	// Determine the proper inode size
	var kb uint32 = (nrblocks * uint32(block_size)) / 1024
	var nrinodes uint32 = kb / 2
	if kb >= 100000 {
		nrinodes = kb / 4
	}

	// round up to fill the inode block
	nrinodes = nrinodes + uint32((inodes_per_block - 1))
	nrinodes = nrinodes / uint32(inodes_per_block) * uint32(inodes_per_block)
	if nrinodes > INODE_MAX {
		nrinodes = INODE_MAX
	}

	fmt.Printf("Size of disk in Kb: %d\n", kb)
	fmt.Printf("Block size: %d\n", block_size)
	fmt.Printf("Inode size: %d\n", V2_INODE_SIZE)
	fmt.Printf("Number of blocks: %d\n", nrblocks)
	fmt.Printf("Number of inodes: %d\n", nrinodes)

	// no zone_shift here, since it's unused
	zones := nrblocks

	put_block(file, 0, zero)	// write the null boot block
	write_superblock(file, zones, nrinodes)
}

func write_superblock(file *os.File, zones uint32, inodes uint32) {
}

func put_block(file *os.File, offset int64, data []byte) {
	n, err := file.WriteAt(data, offset)
	if n != len(data) || err != nil {
		panic(fmt.Sprintf("Failed when writing block: %d and %s", n, err.String()))
	}
}
