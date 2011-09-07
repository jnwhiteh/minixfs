package minixfs

import (
	"os"
)

// Remove all the zones from the inode and mark it as dirty
func (fs *fileSystem) truncate(rip *Inode) {
	file_type := rip.Mode() & I_TYPE

	// check to see if the file is special
	if file_type == I_CHAR_SPECIAL || file_type == I_BLOCK_SPECIAL {
		return
	}

	super := fs.supers[rip.dev]
	scale := super.Log_zone_size
	zone_size := super.Block_size << scale
	nr_indirects := super.Block_size / V2_ZONE_NUM_SIZE

	// PIPE:
	// // Pipes can shrink, so adjust size to make sure all zones are removed
	// waspipe := rip.pipe
	// if waspipe {
	// 	rip.Size = PIPE_SIZE(fs.Block_size)
	// }

	// step through the file a zone at a time, finding and freeing the zones
	for position := uint(0); position < uint(rip.Size()); position += zone_size {
		if b := read_map(rip, int(position), fs.cache); b != NO_BLOCK {
			z := b >> scale
			fs.free_zone(rip.dev, uint(z))
		}
	}

	// all the dirty zones have been freed. Now free the indirect zones
	rip.SetDirty(true)
	// PIPE:
	// if waspipe {
	// 	fs.WipeInode(rip)
	// 	return
	// }
	single := V2_NR_DZONES
	fs.free_zone(rip.dev, uint(rip.Zone(single)))
	if z := rip.Zone(single + 1); z != NO_ZONE {
		// free all the single indirect zones pointed to by the double
		b := int(z << scale)
		bp := fs.get_block(rip.dev, b, INDIRECT_BLOCK, NORMAL)
		for i := uint(0); i < nr_indirects; i++ {
			z1 := rd_indir(bp, int(i), fs.cache, rip.Firstdatazone(), rip.Zones())
			fs.free_zone(rip.dev, uint(z1))
		}
		// now free the double indirect zone itself
		fs.put_block(bp, INDIRECT_BLOCK)
		fs.free_zone(rip.dev, uint(z))
	}

	// leave zone numbers for de(1) to recover file after an unlink(2)
}

func (fs *fileSystem) do_unlink(proc *Process, path string) (*Inode, *Inode, string, os.Error) {
	// Get the last directory in the path
	rldirp, rest, err := fs.last_dir(proc, path)
	if rldirp == nil {
		return nil, nil, "", err
	}

	// The last directory exists. Does the file also exist?
	rip, err := fs.advance(proc, rldirp, rest)
	if rip == nil {
		fs.put_inode(rldirp)
		return nil, nil, "", err
	}

	// If error, return inode
	if err != nil {
		fs.put_inode(rldirp)
		fs.put_inode(rip)
		return nil, nil, "", nil
	}

	// Do not remove a mount point
	if rip.inum == ROOT_INODE {
		fs.put_inode(rldirp)
		fs.put_inode(rip)
		return nil, nil, "", EBUSY
	}

	return rldirp, rip, rest, nil
}

// remove_dir removes a directory from the filesystem. In order for this
// function to work, five conditions must be met:
//   - The file must be a directory
//   - The directory must be empty (except for . and ..)
//   - The directory must not be the root of a mounted file system
//   - The directory must not be anybody's root/working directory
func (fs *fileSystem) remove_dir(proc *Process, rldirp, rip *Inode, dir_name string) os.Error {
	// check to see if the directory is empty
	zeroinode := 0
	if err := fs.search_dir(rip, "", &zeroinode, IS_EMPTY); err != nil {
		return err
	}

	if dir_name == "." || dir_name == ".." {
		return EINVAL
	}
	if rip.inum == ROOT_INODE { // can't remove 'root'
		return EBUSY
	}

	for _, proc := range fs.procs {
		if proc != nil && (proc.rootdir == rip || proc.workdir == rip) {
			return EBUSY // can't remove anyone's working directory
		}
	}

	// Actually try to unlink the file; fails if parent is mode 0, etc.
	if err := fs.unlink_file(rldirp, rip, dir_name); err != nil {
		return err
	}

	// Unlink . and .. from the dir. The super user can link and unlink any
	// dir, so don't make too many assumptions about them.
	fs.unlink_file(rip, nil, ".")
	fs.unlink_file(rip, nil, "..")
	return nil
}

// Unlink 'file_name'; rip must be the inode of 'file_name' or nil
func (fs *fileSystem) unlink_file(dirp, rip *Inode, file_name string) os.Error {
	var numb int

	// If rip is not nil, it is used to get faster access to the inode
	if rip == nil {
		// Search for file in directory and try to get its inode
		err := fs.search_dir(dirp, file_name, &numb, LOOK_UP)
		if err == nil {
			rip, err = fs.get_inode(dirp.dev, uint(numb))
		}
		if err != nil || rip == nil {
			return err
		}
	} else {
		fs.dup_inode(rip) // inode will be returned with put_inode
	}

	zeroinode := 0
	err := fs.search_dir(dirp, file_name, &zeroinode, DELETE)

	if err == nil {
		rip.DecNlinks()
		// TODO: update times
		// rip.update |= CTIME
		rip.SetDirty(true)
	}

	fs.put_inode(rip)
	return err
}
