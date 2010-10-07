package main

import "flag"
import "fmt"
import "os"

const (
	MIN_BLOCK_SIZE    = 1024
	MAX_BLOCK_SIZE    = 4096
	STATIC_BLOCK_SIZE = 1024
	SECTOR_SIZE       = 512
)

// This command is used to create a new minix3-v3 filesystem with a root
// directory owned by by the superuser (uid 0)

func main() {
	var inode_count int
	var block_size int
	var block_count int
	var help bool
	var filename string

	// Define commandline flags
	flag.IntVar(&inode_count, "inodecount", 0, "the number of inodes in the filesystem")
	flag.IntVar(&block_size, "blocksize", MAX_BLOCK_SIZE, "the block size (in bytes)")
	flag.IntVar(&block_count, "size", 1000, "the size of the filesystem (in blocks)")
	flag.BoolVar(&help, "help", false, "display the usage for this command")

	// Parse the flags from the commandline
	flag.Parse()

	// Check to ensure a filename is given on the commandline
	if len(os.Args) == 2 {
		filename = os.Args[1]
	} else {
		help = true
	}

	if help {
		fmt.Fprintf(os.Stdout, "Usage: %s <filename>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(-1)
	}

	// Open the existing disk image and perform some operations
	file, err := os.Open(filename, os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open disk image: %s\n", err.String())
		os.Exit(-1)
	}
	sup := read_superblock(file)
	fmt.Printf("Ninodes: %d\n", sup.Ninodes)
	fmt.Printf("Nzones: %d\n", sup.Nzones)
	fmt.Printf("Imap_blocks: %d\n", sup.Imap_blocks)
	fmt.Printf("Zmap_blocks: %d\n", sup.Zmap_blocks)
	fmt.Printf("Firstdatazone: %d\n", sup.Firstdatazone)
	fmt.Printf("Log_zone_size: %d\n", sup.Log_zone_size)
	fmt.Printf("Max_size: %d\n", sup.Max_size)
	fmt.Printf("Zones: %d\n", sup.Zones)
	fmt.Printf("Magic: 0x%x\n", sup.Magic)
	fmt.Printf("Block_size: %d\n", sup.Block_size)
	fmt.Printf("Disk_version: %d\n", sup.Disk_version)
}
