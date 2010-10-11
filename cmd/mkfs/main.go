package main

import "jnwhiteh/minixfs"
import "bytes"
import "encoding/binary"
import "flag"
import "fmt"
import "os"

const (
	MIN_BLOCK_SIZE    = 1024
	MAX_BLOCK_SIZE    = 4096
	STATIC_BLOCK_SIZE = 1024
	SECTOR_SIZE       = 512
	V2_INODE_SIZE     = minixfs.V2_INODE_SIZE
)

func ferr(f string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, f, s...)
}

func write(file *os.File, data []byte) {
	n, err := file.Write(data)
	if n != len(data) || err != nil {
		ferr("Error writing block to file (%d bytes written): %s\n", n, err)
		panic("blah")
	}
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

	var sup *minixfs.Superblock

	if query {
		sup, err = minixfs.Read_superblock(file)
		if err != nil {
			ferr("Error reading superblock from file '%s': %s\n", filename, err)
			os.Exit(-1)
		}
	} else {
		// allocate the boot block
		boot_block := make([]byte, STATIC_BLOCK_SIZE, STATIC_BLOCK_SIZE)
		supr_block := new(bytes.Buffer)

		// create the superblock data struct
		sup, err = minixfs.NewSuperblock(block_count, inode_count, block_size)
		if err != nil {
			ferr("Error creating new superblock: %s\n", err)
			os.Exit(-1)
		}

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
			ferr("Error writing superblock: %s\n", err)
			os.Exit(-1)
		}

		// Create the rest of the filesystem.
		imap_size := sup.Block_size * sup.Imap_blocks
		imap := make([]byte, imap_size, imap_size)
		write(file, imap)

		zmap_size := sup.Block_size * sup.Imap_blocks
		zmap := make([]byte, zmap_size, zmap_size)
		write(file, zmap)

		// Build up the inode blocks
		inode_size := sup.Ninodes * V2_INODE_SIZE
		if inode_size%uint32(sup.Block_size) != 0 {
			ferr("Inodes do not fill block completely, failing.")
			os.Exit(-1)
		}

		inodes := make([]byte, inode_size, inode_size)
		write(file, inodes)

		// Create the data blocks
		inode_blocks := (sup.Ninodes * V2_INODE_SIZE) / uint32(sup.Block_size)
		data_start_block := 1 + 1 + uint32(sup.Imap_blocks) + uint32(sup.Zmap_blocks) + inode_blocks

		// This operates under the assumption that zones == blocks
		data_block_size := sup.Zones - data_start_block
		fmt.Printf("Data-block_size: %d\n", data_block_size)

		data_block := make([]byte, sup.Block_size, sup.Block_size)
		for i := uint32(0); i < data_block_size; i++ {
			write(file, data_block)
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
