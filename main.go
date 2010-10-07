package main

import "fmt"
import "os"

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

func main() {
	// Open the existing disk image and perform some operations
	file, err := os.Open("bootflop.mfs", os.O_RDONLY, 0)
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
	fmt.Printf("Magic: %d\n", sup.Magic)
	fmt.Printf("Block_size: %d\n", sup.Block_size)
	fmt.Printf("Disk_version: %d\n", sup.Disk_version)
}
