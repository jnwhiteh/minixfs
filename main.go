package main

import "bytes"
import "encoding/binary"
import "flag"
import "fmt"
import "math"
import "os"

const (
	MIN_BLOCK_SIZE    = 1024
	MAX_BLOCK_SIZE    = 4096
	STATIC_BLOCK_SIZE = 1024
	SECTOR_SIZE       = 512
	V2_INODE_SIZE     = 64
	SUPER_V3          = 0x4d5a
	V2_NR_DZONES      = 7
	V2_ZONE_NUM_SIZE  = 4 // the number of bytes in a zone_t (uint32)
	ZONE_SHIFT        = 0 // unused, but leaving in for clarity
	CHAR_BIT          = 8 // number of bits in a char
	START_BLOCK       = 2 // first block of FS (not counting SB)
)

func ferr(f string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, f, s...)
}

// This command is used to create a new minix3-v3 filesystem with a root
// directory owned by by the superuser (uid 0)

func main() {
	var inode_count uint
	var block_size uint
	var block_count uint
	var help bool
	var filename string
	var query bool

	// Define commandline flags
	flag.UintVar(&inode_count, "inodecount", 0, "the number of inodes in the filesystem")
	flag.UintVar(&block_size, "blocksize", MAX_BLOCK_SIZE, "the block size (in bytes)")
	flag.UintVar(&block_count, "size", 1000, "the size of the filesystem (in blocks)")
	flag.BoolVar(&help, "help", false, "display the usage for this command")
	flag.BoolVar(&query, "query", false, "query the image file rather than create")
	flag.StringVar(&filename, "file", "", "the image filename")

	// Parse the flags from the commandline
	flag.Parse()

	// Check to ensure a filename is given on the commandline
	if len(filename) <= 0 {
		ferr("Must specify a filename")
		help = true
	}

	if help {
		ferr("Usage: %s <filename>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(-1)
	}

	// Sanity check arguments to ensure values are all valid
	if block_size%SECTOR_SIZE > 0 || block_size < MIN_BLOCK_SIZE {
		ferr("block size must be a mulitiple of sector (%d) and at least %d bytes\n", SECTOR_SIZE, MIN_BLOCK_SIZE)
		os.Exit(-1)
	}
	if block_size%V2_INODE_SIZE > 0 {
		ferr("block size must be a multiple of inode size (%d bytes)\n", V2_INODE_SIZE)
		os.Exit(-1)
	}
	if block_count < 1 {
		ferr("block count cannot be 0\n")
		os.Exit(-1)
	}

	file, err := os.Open(filename, os.O_RDWR|os.O_CREAT, 0644)
	if err != nil {
		ferr("Error creating image file '%s'\n", filename)
		os.Exit(-1)
	}
	defer file.Close()

	var sup *super_block

	if query {
		sup = read_superblock(file)
	} else {
		// allocate the boot block
		boot_block := make([]byte, STATIC_BLOCK_SIZE, STATIC_BLOCK_SIZE)
		supr_block := new(bytes.Buffer)

		// create the superblock data struct
		sup = create_superblock(block_count, inode_count, block_size)

		// write the bootblock/superblock to the buffer so we can pad it
		n, err := file.Write(boot_block)
		if n != STATIC_BLOCK_SIZE || err != nil {
			ferr("Failed to write boot block: %s\n", err)
			os.Exit(-1)
		}

		err = binary.Write(supr_block, binary.LittleEndian, sup)
		if supr_block.Len() < STATIC_BLOCK_SIZE {
			diff := STATIC_BLOCK_SIZE - supr_block.Len()
			pad := make([]byte, diff, diff)
			supr_block.Write(pad)
		}

		n, err = file.Write(supr_block.Bytes())
		if supr_block.Len() != STATIC_BLOCK_SIZE {
			ferr("Error writing superblock\n")
			os.Exit(-1)
		}
	}

	fmt.Printf("Ninodes: %d\n", sup.Ninodes)
	fmt.Printf("Nzones: %d\n", sup.Nzones)
	fmt.Printf("Imap_blocks: %d\n", sup.Imap_blocks)
	fmt.Printf("Zmap_blocks: %d\n", sup.Zmap_blocks)
	fmt.Printf("Firstdatazone_old: %d\n", sup.Firstdatazone_old)
	fmt.Printf("Log_zone_size: %d\n", sup.Log_zone_size)
	fmt.Printf("Max_size: %d\n", sup.Max_size)
	fmt.Printf("Zones: %d\n", sup.Zones)
	fmt.Printf("Magic: 0x%x\n", sup.Magic)
	fmt.Printf("Block_size: %d\n", sup.Block_size)
	fmt.Printf("Disk_version: %d\n", sup.Disk_version)
}

func bitmapsize(nr_bits uint, block_size uint) uint {
	// In this assignment, 2 == usizeof(bitchunk_t)
	var bchunks uint = block_size / 2
	var bchunk_bits uint = 2 * CHAR_BIT
	bits_per_block := bchunks * bchunk_bits

	var nr_blocks uint = nr_bits / bits_per_block
	if (nr_blocks * bits_per_block) < nr_bits {
		nr_blocks = nr_blocks + 1
	}
	return nr_blocks
}

func create_superblock(blocks, inodes, block_size uint) *super_block {
	sup := new(super_block)

	inodes_per_block := block_size / V2_INODE_SIZE

	// Check to see if inode count is automatic (0) and adjust accordingly
	if inodes == 0 {

		kb := (blocks * block_size) / 1024
		inodes = kb / 2
		if kb >= 100000 {
			inodes = kb / 4
		}

		// round up to fill inode block
		inodes = inodes + inodes_per_block - 1
		inodes = inodes / inodes_per_block * inodes_per_block
	}

	if inodes < 1 {
		ferr("Inode count too small (%d).\n", inodes)
		os.Exit(-1)
	}

	zones := blocks >> ZONE_SHIFT

	sup.Ninodes = uint32(inodes)
	if uint(sup.Ninodes) != inodes {
		ferr("Inode count is too high, need fewer inodes.\n")
		os.Exit(-1)
	}

	sup.Nzones = 0
	sup.Zones = uint32(zones)

	// Perform a check here to see if we need a larger block size
	// for a filesystem of the given size. This is accomplished
	// by checking overflow when assigned to the struct
	nb := bitmapsize(1+inodes, block_size)
	sup.Imap_blocks = uint16(nb)
	if uint(sup.Imap_blocks) != nb {
		ferr("Too many inode bitmap blocks, please try a larger block size.\n")
		os.Exit(-1)
	}

	nb = bitmapsize(zones, block_size)
	sup.Zmap_blocks = uint16(nb)
	if uint(sup.Imap_blocks) != nb {
		ferr("Too many zone bitmap blocks, please try a larger block size.\n")
		os.Exit(-1)
	}

	initblks := inodes + inodes_per_block
	nb = (initblks + (1 << ZONE_SHIFT) - 1) >> ZONE_SHIFT
	if nb >= zones {
		ferr("bitmaps too large\n")
		os.Exit(-1)
	}
	sup.Firstdatazone_old = uint16(nb)
	if uint(sup.Firstdatazone_old) != nb {
		// The field is too small to store the value. Fortunately, the value
		// can be computed from other fields. We set the on-disk field to zero
		// to indicate that it must not be u sed. Eventually we can always set
		// the on-disk field to zero, and stop using it.
		sup.Firstdatazone_old = 0
	}
	sup.Log_zone_size = ZONE_SHIFT

	v2indirect := (block_size / V2_ZONE_NUM_SIZE)
	v2sq := v2indirect * v2indirect
	zo := V2_NR_DZONES + v2indirect + v2sq

	sup.Magic = SUPER_V3
	sup.Block_size = uint16(block_size)
	if uint(sup.Block_size) != block_size {
		ferr("Block size is too large, please choose a smaller one.\n")
		os.Exit(-1)
	}
	if math.MaxUint32/block_size < zo {
		sup.Max_size = math.MaxInt32
	} else {
		sup.Max_size = int32(zo * block_size)
		if uint(sup.Max_size) != (zo * block_size) {
			ferr("Maximum file size is too large. Failing.\n")
			os.Exit(-1)
		}
	}

	return sup
}
