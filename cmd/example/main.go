package main

import "jnwhiteh/minixfs"
import "fmt"

func main() {
	fs, err := minixfs.OpenFileSystemFile("hello.img")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Magic of filesystem is: %d\n", fs.GetMagic())

	root, err := fs.GetInode(minixfs.ROOT_INODE_NUM)
	inode := root

	fmt.Printf("Mode: %d\n", inode.Mode)
	fmt.Printf("Nlinks: %d\n", inode.Nlinks)
	fmt.Printf("Size: %d\n", inode.Size)
	fmt.Printf("Uid: %d\n", inode.Uid)
	fmt.Printf("Gid: %d\n", inode.Gid)
	fmt.Printf("Atime: %d\n", inode.Atime)
	fmt.Printf("Mtime: %d\n", inode.Atime)
	fmt.Printf("Ctime: %d\n", inode.Atime)
	for idx, zoneNum := range inode.Zone {
		if zoneNum != 0 {
			fmt.Printf("Zone[%d]: %d\n", idx, inode.Zone[0])
		}
	}

	// Get the directory block from the zone specified
	dirent_per_block := fs.GetBlockSize() / 64
	dir_block := make([]minixfs.Directory, dirent_per_block)
	fs.GetBlock(uint(inode.Zone[0]), dir_block)
	for _, dirent := range dir_block {
		if dirent.Inum > 0 {
			fmt.Printf("Inode: %d\tName: %s\n", dirent.Inum, dirent.Name)
		}
	}
}
