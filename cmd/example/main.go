package main

import "jnwhiteh/minixfs"
import "fmt"

func main() {
	fs, err := minixfs.OpenFileSystemFile("hello.img")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Magic of filesystem is: %d\n", fs.GetMagic())

	root, err := fs.GetInode(1)
	inode := root

	fmt.Printf("Mode: %d\n", inode.Mode)
	fmt.Printf("Nlinks: %d\n", inode.Nlinks)
	fmt.Printf("Uid: %d\n", inode.Uid)
	fmt.Printf("Gid: %d\n", inode.Gid)
	fmt.Printf("Atime: %d\n", inode.Atime)
	fmt.Printf("Mtime: %d\n", inode.Atime)
	fmt.Printf("Ctime: %d\n", inode.Atime)
	fmt.Printf("Zone[0]: %d\n", inode.Zone[0])
	fmt.Printf("Zone[1]: %d\n", inode.Zone[1])
	fmt.Printf("Zone[2]: %d\n", inode.Zone[2])
	fmt.Printf("Zone[3]: %d\n", inode.Zone[3])
	fmt.Printf("Zone[4]: %d\n", inode.Zone[4])
	fmt.Printf("Zone[5]: %d\n", inode.Zone[5])
	fmt.Printf("Zone[6]: %d\n", inode.Zone[6])
	fmt.Printf("Zone[7]: %d\n", inode.Zone[7])
	fmt.Printf("Zone[8]: %d\n", inode.Zone[8])
	fmt.Printf("Zone[9]: %d\n", inode.Zone[9])

	// We know this is the root directory, so the data contains directory
	// entries. Start by reading the first block of these. Need to convert
	// from the zone number to a block number. Luckily, the zone_shift is 
	// 0 so we can just convert directly. 
	dir_block := new(minixfs.DirectoryBlock_16)

	err = fs.GetBlock(20, dir_block)
	if err != nil {
		panic(err)
	}
	for idx := 0; idx < 4; idx++ {
		entry := dir_block.Data[idx]
		fmt.Printf("Directory entry: %d\n", idx)
		fmt.Printf("Inode: %d\tName: %s\n", entry.Inum, entry.Name)
	}
}
