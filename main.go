package main

import "fmt"
import "os"

// Notes:
// 18:47 exch: with unsafe it gets a lil messy, but doable: var t T; slice := (*(*[1<<31 - 1]byte)(unsafe.Pointer(&t)))[0:sizeOfStruct]
// 18:47 ronnyy has left IRC (Remote host closed the connection)
// 18:47 jnwhiteh: exch: thanks, I'll make a note of that
// 18:47 exch: note that this will not actually allocate 1<<31-1 bytes. it just a pointer. which makes it extra nice
// 18:48 exch: incidentally 1<<31-1 is the largest possible size an array/slice can have

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
	write_superblock(file, zone_t(zones), ino_t(nrinodes), block_size, inodes_per_block)
}

func bitmapsize(nr_bits bit_t, block_size uint) (int) {
	bits_per_block := (int(block_size) / 2) * (2 / CHAR_BIT)
	var nr_blocks int = int(nr_bits) / bits_per_block
	// This is hardcoded, is actually usizeof(bitchunk_t) instead of 2
	if bit_t(nr_blocks * bits_per_block) < nr_bits {
		nr_blocks = nr_blocks + 1
	}
	return nr_blocks
}

func write_superblock(file *os.File, zones zone_t, inodes ino_t, block_size, inodes_per_block uint) {
	var sup = new(super_block)
	sup.s_ninodes = inodes
	sup.s_nzones = 0
	sup.s_zones = zones

	BIGGERBLOCKS := "Please try a larger block size for an FS of this size.\n"
	nb1 := bitmapsize(bit_t(1 + inodes), block_size)
	sup.s_imap_blocks = short(nb1)
	if int(sup.s_imap_blocks) != nb1 {
		fmt.Fprintf(os.Stderr, "too many inode bitmap blocks. %s\n", BIGGERBLOCKS)
		os.Exit(-1)
	}
	nb2 := bitmapsize(bit_t(zones), block_size)
	if nb2 != int(sup.s_zmap_blocks) {
		fmt.Fprintf(os.Stderr, "too many block bitmap blocks. %s\n", BIGGERBLOCKS)
		os.Exit(-1)
	}

	// These are declared globally in the main tool
	inode_offset := START_BLOCK + sup.s_imap_blocks + sup.s_zmap_blocks
	inodeblks := (ulong(inodes) + ulong(inodes_per_block - 1)) / ulong(inodes_per_block)
	initblks := long(inode_offset) + long(inodeblks)

	nb3 := (initblks + (1 << 0) - 1) >> 0
	sup.s_firstdatazone_old = zone1_t(nb3)
	if nb3 >= long(zones) {
		fmt.Fprintf(os.Stderr, "bit maps too large")
		os.Exit(-1)
	}
	if nb3 != long(sup.s_firstdatazone_old) {
		// The field is too small to store the value. Fortunately, the value
		// can be computed from other fields. We set the on-disk field to zero
		// to indicate that it must not be used. Eventually, we can always set
		// the on-disk field to zero and stop using it.
		sup.s_firstdatazone_old = 0
	}
	sup.s_firstdatazone = zone_t(nb3)
	sup.s_log_zone_size = 0
	v2sq := (block_size / 4) * (block_size / 4)
	zo := V2_NR_DZONES + (block_size / 4) + v2sq
	sup.s_magic = SUPER_V2
	sup.s_max_size = off_t(zo * block_size)

	//zone_size := 1 << 0	// number of blocks per zone

	_, err := file.Seek(STATIC_BLOCK_SIZE, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "write_superblock() couldn't seek\n")
		os.Exit(-1)
	}

	buf := []byte(sup)

	zero := make([]byte, STATIC_BLOCK_SIZE, STATIC_BLOCK_SIZE)

	nwr, err := file.Write(buf)
	if nwr != len(buf) || err != nil {
		fmt.Fprintf(os.Stderr, "write_superblock() couldn't write: %d, %s\n", nwr, err.String())
		os.Exit(-1)
	}
	if len(buf) < STATIC_BLOCK_SIZE {
		left := STATIC_BLOCK_SIZE - len(buf)
		nwr, err = file.Write(zero[0:left])
		if nwr != left || err != nil {
			fmt.Fprintf(os.Stderr, "error filling in rest of buffer: %d, %s\n", nwr, err.String())
		}
	}
}

func put_block(file *os.File, offset int64, data []byte) {
	n, err := file.WriteAt(data, offset)
	if n != len(data) || err != nil {
		panic(fmt.Sprintf("Failed when writing block: %d and %s", n, err.String()))
	}
}
