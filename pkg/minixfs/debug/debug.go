package debug

import (
	. "../../minixfs/common/_obj/minixfs/common"
	"bytes"
	"fmt"
	"log"
)

func PrintBlock(bp *CacheBlock, devinfo DeviceInfo) {
	switch bp.Block.(type) {
	case DirectoryBlock:
		// Print the directory block entries
		buf := bytes.NewBuffer(nil)
		bdata := bp.Block.(DirectoryBlock)
		for i, dirent := range bdata {
			if dirent.Name[0] != 0 && dirent.Inum != 0 {
				fmt.Fprintf(buf, "Entry %8d: \"%s\" at inode %8d\n", i, dirent, dirent.Inum)
			}
		}
		log.Printf("Block data follows:\n%s\n", buf.String())
	case InodeBlock:
		// Print which inodes these are, so need to convert from block number
		// to inode number.
		block_offset := devinfo.MapOffset
		inum := ((bp.Blockno - block_offset) * (devinfo.Blocksize / V2_INODE_SIZE)) + 1
		buf := bytes.NewBuffer(nil)
		bdata := bp.Block.(InodeBlock)
		fmt.Fprintf(buf, "%8s %-16s %8s %8s %s\n", "INODE #", "MODE", "NLINKS", "SIZE", "ZONES")
		for i, inode := range bdata {
			if inode.Mode != 0 && inode.Nlinks != 0 {
				fmt.Fprintf(buf, "%8d %16b %8d %8d %v\n", inum+i, inode.Mode, inode.Nlinks, inode.Size, inode.Zone)
			}
		}
		log.Printf("Block data follows:\n%s\n", buf.String())
	}
}
