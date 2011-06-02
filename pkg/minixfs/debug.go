package minixfs

import (
	"bytes"
	"fmt"
	"log"
	"runtime"
)

func here() {
	_, file, line, _ := runtime.Caller(1)
	log.Printf("here: %s:%d", file, line)
}

func _debugPrintBlock(bp *buf, super *Superblock) {
	switch bp.block.(type) {
	case DirectoryBlock:
		buf := bytes.NewBuffer(nil)
		bdata := bp.block.(DirectoryBlock)
		for i, dirent := range bdata {
			name := dirent.Name
			inum := dirent.Inum
			if name[0] != 0 && inum != 0 {
				fmt.Fprintf(buf, "Entry %8d: \"%60s\" at inode %8d\n", i, name, inum)
			}
		}
		log.Printf("Block data follows:\n%s\n", buf.String())
	case InodeBlock:
		// Need to print which inodes these are, so need to convert from block
		// number to inode number.
		block_offset := super.Imap_blocks + super.Zmap_blocks + 2
		inum := ((uint(bp.blocknr) - block_offset) * (super.inodes_per_block)) + 1

		buf := bytes.NewBuffer(nil)
		bdata := bp.block.(InodeBlock)
		fmt.Fprintf(buf, "%8s %-16s %8s %8s %s\n", "INODE #", "MODE", "NLINKS", "SIZE", "ZONES")
		for i, inode := range bdata {
			if inode.Mode != 0 && inode.Nlinks != 0 {
				fmt.Fprintf(buf, "%8d %16b %8d %8d %v\n", inum+uint(i), inode.Mode, inode.Nlinks, inode.Size, inode.Zone)
			}
		}
		log.Printf("Block data follows:\n%s\n", buf.String())
	}
}
