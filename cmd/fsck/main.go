// This command checks the consistency of Minix3 file system. It does not
// attempt to be general, and does not contain legacy code for older versions
// of the file system.
package main

import "encoding/binary"
import "flag"
import "fmt"
import "log"
import "math"
import "os"

var repair *bool = flag.Bool("repair", false, "repair the filesystem")
var filename *string = flag.String("file", "minix3root.img", "the disk image to check")
var help *bool = flag.Bool("help", false, "print usage information")
var listing *bool = flag.Bool("listing", false, "show a listing")

var firstlist bool   // has the listing header been printed?
var firstcnterr bool // has the count error header been printed?

var fsck_device string // the name of the disk image
var dev *os.File       // the 'device' we are operating on
var sb *superblock     // the superblock of the device
var fs_version int     // the version of the filesystem
var block_size int     // the block size of the device
var rwbuf []byte       // a buffer for reading and writing
var nullbuf []byte     // a zeroed buffer

var imap []bitchunk_t
var zmap []bitchunk_t
var spec_imap []bitchunk_t
var spec_zmap []bitchunk_t
var dirmap []bitchunk_t
var count map[int]int

var nfreeinode, nregular, ndirectory, nblkspec, ncharspec, nbadinode int
var nsock, npipe, nsyml int
var nfreezone int
var ztype map[int]int

type stack struct {
	dir      *Directory
	next     *stack
	presence byte
}

var ftop *stack = nil

const (
	OFFSET_SUPER_BLOCK = 1024
)

func devopen(filename string) {
	var err os.Error

	if *repair {
		dev, err = os.OpenFile(filename, os.O_RDWR, 0666)
	} else {
		dev, err = os.Open(filename)
	}

	if err != nil {
		log.Fatalf("couldn't open device to fsck: %s", filename)
	}
}

func initvars() {
	firstlist = true
	ztype = make(map[int]int)
}

func lsuper() {
	// TODO: Make this a loop that allows the user to change
	// the values for each entry.
	fmt.Printf("ninodes       = %v\n", sb.Ninodes)
	fmt.Printf("nzones        = %v\n", sb.Zones)
	fmt.Printf("imap_blocks   = %v\n", sb.Imap_blocks)
	fmt.Printf("zmap_blocks   = %v\n", sb.Zmap_blocks)
	fmt.Printf("firstdatazone = %v\n", sb.Firstdatazone_old)
	fmt.Printf("log_zone_size = %v\n", sb.Log_zone_size)
	fmt.Printf("maxsize       = %v\n", sb.Max_size)
	fmt.Printf("block size    = %v\n", sb.Block_size)
}

func getsuper() {
	if _, err := dev.Seek(OFFSET_SUPER_BLOCK, 0); err != nil {
		log.Fatalf("couldn't seek to super block: %s", err)
	}

	sb = &superblock{disk_superblock: new(disk_superblock)}
	if err := binary.Read(dev, binary.LittleEndian, sb.disk_superblock); err != nil {
		log.Fatalf("couldn't read super block: %s", err)
	}

	lsuper() // list the information contained in the superblock
	if sb.Magic == SUPER_MAGIC {
		log.Fatalf("Cannot handle V1 filesystems")
	}
	if sb.Magic == SUPER_V2 {
		fs_version = 2
		block_size = 8192 // STATIC_BLOCK_SIZE
	} else if sb.Magic == SUPER_V3 {
		fs_version = 3
		block_size = int(sb.Block_size)
	} else {
		log.Fatal("bad magic number in super block")
	}

	if sb.Ninodes <= 0 {
		log.Fatal("no inodes")
	}
	if sb.Zones <= 0 {
		log.Fatal("no zones")
	}
	if sb.Imap_blocks <= 0 {
		log.Fatal("no imap")
	}
	if sb.Zmap_blocks <= 0 {
		log.Fatal("no zmap")
	}
	if sb.Firstdatazone_old != 0 && sb.Firstdatazone_old <= 4 {
		log.Fatal("first data zone too small")
	}
	if sb.Log_zone_size < 0 {
		log.Fatal("zone size < block size")
	}
	if sb.Max_size <= 0 {
		log.Printf("warning: invalid max file size: %ld", sb.Max_size)
		sb.Max_size = math.MaxInt32
	}
}

// chksuper checks the super block for reasonable contents
func chksuper() {
	n := bitmapsize(int(sb.Ninodes)+1, block_size)
	if sb.Magic != SUPER_V2 && sb.Magic != SUPER_V3 {
		log.Fatal("bad magic number in super block")
	}
	if int(sb.Imap_blocks) < n {
		log.Printf("need %d blocks for inode bitmap; only have %d",
			n, sb.Imap_blocks)
		log.Fatalf("too few imap blocks")
	}
	if int(sb.Imap_blocks) != n {
		log.Printf("warning: expected %d imap_blocks instead of %d",
			n, sb.Imap_blocks)
	}
	n = bitmapsize(int(sb.Zones), block_size)
	if int(sb.Zmap_blocks) < n {
		log.Fatal("too few zmap blocks")
	}
	if int(sb.Zmap_blocks) != n {
		log.Printf("warning: expected %d zmap_blocks instead of %d",
			n, sb.Zmap_blocks)
	}
	if int(sb.Log_zone_size) >= 8*Sizeof_block_nr {
		log.Fatalf("log_zone_size too large")
	}
	if sb.Log_zone_size > 8 {
		log.Printf("warning: large log_zone_size (%d)", sb.Log_zone_size)
	}

	// Compute the first data zone
	INODES_PER_BLOCK := int(sb.Block_size) / V2_INODE_SIZE
	N_ILIST := (int(sb.Ninodes) + INODES_PER_BLOCK - 1) / INODES_PER_BLOCK
	SCALE := 1 << sb.Log_zone_size

	sb.Firstdatazone = (BLK_ILIST() + N_ILIST + SCALE - 1) >> sb.Log_zone_size

	if sb.Firstdatazone_old != 0 {
		if uint32(sb.Firstdatazone_old) >= sb.Zones {
			log.Fatal("first data zone too large")
		}
		if int(sb.Firstdatazone_old) < sb.Firstdatazone {
			log.Fatal("first data zone too small")
		}
		if int(sb.Firstdatazone_old) != sb.Firstdatazone {
			log.Printf("warning: expected first data zone to be %d instead of %d", sb.Firstdatazone, sb.Firstdatazone_old)
			sb.Firstdatazone = int(sb.Firstdatazone_old)
		}
	}

	maxsize := MAX_FILE_POS
	if ((maxsize-1)>>sb.Log_zone_size)/block_size >= MAX_ZONES(block_size) {
		maxsize = (MAX_ZONES(block_size) * block_size) << sb.Log_zone_size
		if maxsize <= 0 {
			maxsize = math.MaxInt32
		}
		if int(sb.Max_size) != maxsize {
			log.Printf("warning: expected max size to be %d instead of %d", maxsize, sb.Max_size)
		}
	}
}

//TODO: Fix this to take 'clist', which seems to be a list of inodes to change
func lsi() {
}

func getbitmaps() {
	ilen := (int(sb.Imap_blocks) * block_size) / Sizeof_bitchunk_t
	zlen := (int(sb.Zmap_blocks) * block_size) / Sizeof_bitchunk_t
	imap = make([]bitchunk_t, ilen)
	zmap = make([]bitchunk_t, zlen)
	spec_imap = make([]bitchunk_t, ilen)
	spec_zmap = make([]bitchunk_t, zlen)
	dirmap = make([]bitchunk_t, ilen)
}

func fillbitmap(bitmap []bitchunk_t, lwb, upb int, list []string) {
}

func getcount() {
	count = make(map[int]int)
}

func chktree() {
	dir := new(Directory)
	nfreeinode = int(sb.Ninodes)
	nfreezone = int(sb.Zones) - sb.Firstdatazone // N_DATA
	dir.Inum = ROOT_INODE
	if !descendtree(dir) {
		log.Fatal("bad root inode")
	}
	fmt.Print("\n")
}

func isprint(b byte) bool {
	return (b >= '\040' && b <= '\077') || (b >= '\100' && b <= '\176')
}

func printname(s [60]byte) {
	for i := 0; i < 60; i++ {
		if s[i] == 0 {
			break
		} else if !isprint(s[i]) {
			fmt.Print('?')
		} else {
			fmt.Printf("%c", s[i])
		}
	}
}

func printrec(sp *stack) {
	if sp.next != nil {
		printrec(sp.next)
		fmt.Print("/")
		printname(sp.dir.Name)
	}

}

func printpath(mode int, nlcr bool) {
	if ftop.next == nil {
		fmt.Print("/")
	} else {
		printrec(ftop)
	}
	switch mode {
	case 1:
		fmt.Printf(" (ino = %v, ", ftop.dir.Inum)
	case 2:
		fmt.Printf(" (ino = %v)", ftop.dir.Inum)
	}
	if nlcr {
		fmt.Print("\n")
	}
}

// Check the directory entry pointed to by dp, by checking the inode.
func descendtree(dp *Directory) bool {
	ino := int(dp.Inum)
	inode := new(disk_inode)

	stk := new(stack)
	stk.dir = dp
	stk.next = ftop
	ftop = stk
	if bitset(spec_imap, ino) {
		fmt.Printf("found inode %v: ", ino)
		printpath(0, true)
	}

	visited := bitset(imap, ino)
	if !visited || *listing {
		devread(inoblock(ino), inooff(ino), inode, INODE_SIZE)
		if *listing {
			list(ino, inode)
		}
		if !visited && !chkinode(ino, inode) { // this triggers the chk* functions
			setbit(spec_imap, ino)
			if yes("remove") {
				count[ino] += int(inode.Nlinks) - 1
				clrbit(imap, ino)
				// TODO: Fix this
				// devwrite(inoblock(ino), inooff(ino), nullbuf, INODE_SIZE)
				ftop = ftop.next
				return false
			}
		}
	}
	ftop = ftop.next
	return true
}

func list(ino int, ip *disk_inode) {
	if firstlist {
		firstlist = false
		fmt.Printf(" inode permission link   size name\n")
	}
	fmt.Printf("%6d ", ino)
	switch ip.Mode & I_TYPE {
	case I_REGULAR:
		fmt.Print("-")
	case I_DIRECTORY:
		fmt.Print("d")
	case I_CHAR_SPECIAL:
		fmt.Print("c")
	case I_BLOCK_SPECIAL:
		fmt.Print("b")
	case I_NAMED_PIPE:
		fmt.Print("p")
	case I_UNIX_SOCKET:
		fmt.Print("s")
	case I_SYMBOLIC_LINK:
		fmt.Print("s")
	default:
		fmt.Printf("?")
	}
	printperm(ip.Mode, 6, I_SET_UID_BIT, "s")
	printperm(ip.Mode, 3, I_SET_GID_BIT, "s")
	printperm(ip.Mode, 0, STICKY_BIT, "t")
	fmt.Printf(" %3d ", ip.Nlinks)
	switch ip.Mode & I_TYPE {
	case I_CHAR_SPECIAL, I_BLOCK_SPECIAL:
		fmt.Printf("  %2x,%2x ", ip.Zone[0]>>MAJOR&0xFF, ip.Zone[0]>>MINOR&0xFF)
	default:
		fmt.Printf("%7v ", ip.Size)
	}
	printpath(0, true)
}

func printperm(mode uint16, shift uint, special int, overlay string) {
	if (mode>>shift)&R_BIT > 0 {
		fmt.Print("r")
	} else {
		fmt.Print("-")
	}
	if (mode>>shift)&W_BIT > 0 {
		fmt.Print("w")
	} else {
		fmt.Print("-")
	}
	if (mode & uint16(special)) > 0 {
		fmt.Print(overlay)
	} else {
		if (mode>>shift)&X_BIT > 0 {
			fmt.Print("x")
		} else {
			fmt.Print("-")
		}
	}
}

func chkinode(ino int, ip *disk_inode) bool {
	if ino == ROOT_INODE && (ip.Mode&I_TYPE) != I_DIRECTORY {
		fmt.Printf("root inode is not a directory ")
		fmt.Printf("(ino = %v, mode = %o)\n", ino, ip.Mode)
		log.Fatal("")
	}
	if ip.Nlinks == 0 {
		fmt.Printf("link count zero of ")
		printpath(2, true)
		return true
	}
	nfreeinode--
	setbit(imap, ino)
	if ip.Nlinks > math.MaxInt16 { // TODO: Does this work? When would this happen?
		fmt.Printf("link count too big in ")
		printpath(1, false)
		fmt.Printf("cnt = %v)\n", ip.Nlinks)
		count[ino] -= math.MaxInt16
		setbit(spec_imap, ino)
	} else {
		count[ino] -= int(ip.Nlinks)
	}

	return chkmode(ino, ip)
}

// Check the mode and the contents of an inode
func chkmode(ino int, ip *disk_inode) bool {
	switch ip.Mode & I_TYPE {
	case I_REGULAR:
		nregular++
		return chkfile(ino, ip)
	case I_DIRECTORY:
		ndirectory++
		return chkdirectory(ino, ip)
	case I_BLOCK_SPECIAL:
		nblkspec++
		return chkspecial(ino, ip)
	case I_CHAR_SPECIAL:
		ncharspec++
		return chkspecial(ino, ip)
	case I_NAMED_PIPE:
		npipe++
		return chkfile(ino, ip)
	case I_UNIX_SOCKET:
		nsock++
		return chkfile(ino, ip)
	case I_SYMBOLIC_LINK:
		nsyml++
		return chklink(ino, ip)
	}

	// default case
	nbadinode++
	fmt.Printf("bad mode of ")
	printpath(1, false)
	fmt.Printf("mode = %o)", ip.Mode)
	return false
}

func chkfile(ino int, ip *disk_inode) bool {
	var ok bool
	var i int
	var level int
	pos := 0

	ok = chkzones(ino, ip, &pos, ip.Zone[0:V2_NR_DZONES], 0)
	i = V2_NR_DZONES
	level = 1
	for i < V2_NR_TZONES { // all zones, not just direct
		ok = ok && chkzones(ino, ip, &pos, ip.Zone[i:i+1], level)
		i++
		level++
	}
	return ok
}

func chkdirectory(ino int, ip *disk_inode) bool {
	var ok bool

	setbit(dirmap, ino)
	ok = chkfile(ino, ip)
	if !(ftop.presence&DOT > 0) {
		fmt.Printf(". missing in ")
		printpath(2, true)
		ok = false
	}
	if !(ftop.presence&DOTDOT > 0) {
		fmt.Printf(".. missing in ")
		printpath(2, true)
		ok = false
	}
	return ok
}

// Check a list of zones given by 'zlist'
func chkzones(ino int, ip *disk_inode, pos *int, zlist []uint32, level int) bool {
	var ok bool = true

	for i := 0; i < len(zlist); i++ {
		if zlist[i] == NO_ZONE {
			*pos += jump(level)
		} else if !markzone(int(zlist[i]), level, *pos) {
			*pos += jump(level)
			ok = false
		} else if !zonechk(ino, ip, pos, int(zlist[i]), level) {
			ok = false
		}
	}

	return ok
}

func chkspecial(ino int, ip *disk_inode) bool {
	ok := true
	if ip.Zone[0] == NO_DEV {
		fmt.Printf("illegal device number %d for special file ", ip.Zone[0])
		printpath(2, true)
		ok = false
	}

	// File system will not use the remaining "zone numbers", but 1.6.11++ will
	// panic if they are nonzero, since this should not happen.
	for i := 1; i < NR_ZONE_NUMS; i++ {
		if ip.Zone[i] != NO_ZONE {
			fmt.Printf("nonzero zone number %d for special file ", ip.Zone[i])
			printpath(2, true)
			ok = false
		}
	}

	return ok
}

// Check the validity of a symbolic link
func chklink(ino int, ip *disk_inode) bool {
	var ok bool

	ok = chkfile(ino, ip)
	if ip.Size <= 0 || int(ip.Size) > block_size {
		if ip.Size == 0 {
			fmt.Printf("empty symbolic link ")
		} else {
			fmt.Printf("symbolic link too large (size %d) ", ip.Size)
		}
		printpath(2, true)
		ok = false
	}
	return ok
}

func jump(level int) int {
	power := ZONE_SIZE()
	for level != 0 {
		power *= V2_INDIRECTS()
		level--
	}
	return power
}

func markzone(zno, level, pos int) bool {
	bit := zno - FIRST() + 1
	ztype[level]++
	if zno < FIRST() || zno >= int(sb.Zones) {
		errzone("out-of-range", zno, level, pos)
		return false
	}
	if bitset(zmap, bit) {
		setbit(spec_zmap, bit)
		errzone("duplicate", zno, level, pos)
		return false
	}
	nfreezone--
	if bitset(spec_zmap, bit) {
		errzone("found", zno, level, pos)
	}
	setbit(zmap, bit)
	return true
}

func errzone(mess string, zno, level, pos int) {
	fmt.Printf("%s zone in ", mess)
	printpath(1, false)
	fmt.Printf("zno = %d, type = ", zno)
	switch level {
	case 0:
		fmt.Print("DATA")
	case 1:
		fmt.Print("SINGLE INDIRECT")
	case 2:
		fmt.Print("DOUBLE INDIRECT")
	default:
		fmt.Print("VERY INDIRECT")
	}
	fmt.Printf(", pos = %d)\n", pos)
}

func zonechk(ino int, ip *disk_inode, pos *int, zno, level int) bool {
	if level == 0 {
		if (ip.Mode&I_TYPE) == I_DIRECTORY && !chkdirzone(ino, ip, *pos, zno) {
			return false
		}
		if (ip.Mode&I_TYPE) == I_SYMBOLIC_LINK && chksymlinkzone(ino, ip, *pos, zno) {
			return false
		}
		*pos += ZONE_SIZE()
		return true
	}
	return chkindzone(ino, ip, pos, zno, level)
}

// Check a zone of a directory by checking all of the entries in the zone.
// This function has been simplified from the original version to remove
// complicated logic.
func chkdirzone(ino int, ip *disk_inode, pos, zno int) bool {
	block := ztob(zno)
	offset := 0
	size := 0 // the 'size' of the directory
	dirblk := new(Directory)
	dp := dirblk // alias
	var dirty bool
	numentries := SCALE() * (NR_DIR_ENTRIES(block_size) / CDIRECT) // number of entries

	for i := numentries; i > 0; i-- {
		devread(block, offset, dirblk, DIR_ENTRY_SIZE) // read a directory entry
		dirty = false

		if dp.Inum != NO_ENTRY && !chkentry(ino, pos, dp) {
			dirty = true
		}

		pos += DIR_ENTRY_SIZE
		if dp.Inum != NO_ENTRY {
			size = pos
		}
		if dirty {
			// TODO: This will not work.
			devwrite(block, offset, dp, DIRCHUNK)
		}
		offset += DIR_ENTRY_SIZE
	}

	if size > int(ip.Size) {
		fmt.Printf("size not updated of directory ")
		printpath(2, false)
		if yes(". extend") {
			setbit(spec_imap, ino)
			ip.Size = int32(size)
			devwrite(inoblock(ino), inooff(ino), ip, INODE_SIZE)
		}
	}

	return true
}

func chkentry(ino int, pos int, dp *Directory) bool {
	if dp.Inum < ROOT_INODE || dp.Inum > sb.Ninodes {
		fmt.Printf("bad inode found in directory ")
		printpath(1, false)
		fmt.Printf("ino found = %d, ", dp.Inum)
		fmt.Printf("name = '")
		printname(dp.Name)
		fmt.Printf("')")
		if yes(". remove entry") {
			// How do we remove this entry?
			return false
		}
		return true
	}
	if count[int(dp.Inum)] == math.MaxInt16 {
		fmt.Printf("too many links to ino %d\n", dp.Inum)
		fmt.Printf("discovoered at entry '")
		printname(dp.Name)
		fmt.Printf("' in directory ")
		printpath(0, true)
		if remove(dp) {
			return false
		}
	}
	count[int(dp.Inum)]++
	if strcmp(dp.Name[:], ".") {
		ftop.presence |= DOT
		return chkdots(ino, pos, dp, ino)
	}
	if strcmp(dp.Name[:], "..") {
		ftop.presence |= DOTDOT
		var x int
		if ino == ROOT_INODE {
			x = ino
		} else {
			x = int(ftop.next.dir.Inum)
		}
		return chkdots(ino, pos, dp, x)
	}
	if !chkname(ino, dp) {
		return false
	}
	if bitset(dirmap, int(dp.Inum)) {
		fmt.Printf("link to directory discovered in ")
		printpath(1, false)
		fmt.Printf("name = '")
		printname(dp.Name)
		fmt.Printf("', dir ino = %d)", dp.Inum)
		return !remove(dp)
	}
	return descendtree(dp)
}

// TODO: Implement
func chksymlinkzone(ino int, ip *disk_inode, pos, zno int) bool {
	if int(ip.Size) > PATH_MAX {
		log.Fatalf("chksymlinkzone: fsck program inconsistency")
	}

	target := make([]byte, ip.Size)
	block := ztob(zno)
	devread(block, 0, target, int(ip.Size))
	slen := strlen(target)
	if slen != int(ip.Size) {
		fmt.Printf("bad size in symbolic link (%d instead of %d) ",
			ip.Size, slen)
		printpath(2, false)
		if yes(". update") {
			setbit(spec_imap, ino)
			ip.Size = int32(slen)
			devwrite(inoblock(ino), inooff(ino), ip, INODE_SIZE)
		}
	}
	return true
}

func strlen(b []byte) int {
	for idx, c := range b {
		if c == 0 {
			return idx
		}
	}

	return len(b)
}

// Check an indirect zone by checking all of its entries.
func chkindzone(ino int, ip *disk_inode, pos *int, zno, level int) bool {
	indirect := make([]uint32, CINDIR)
	n := V2_INDIRECTS() / CINDIR
	block := ztob(zno)
	offset := 0

	for {
		devread(block, offset, indirect, CINDIR*4) // size
		if !chkzones(ino, ip, pos, indirect, level-1) {
			return false
		}
		offset += (CINDIR * 4)
		n--
		if !(n > 0 && *pos < int(ip.Size)) {
			break
		}
	}

	return true
}

func chkdots(ino int, pos int, dp *Directory, exp int) bool {
	if int(dp.Inum) != exp {
		printable_name := make_printable_name(dp.Name)
		fmt.Printf("bad %s in ", printable_name)
		printpath(1, false)
		fmt.Printf("%s is linked to %d ", printable_name, dp.Inum)
		fmt.Printf("instead of %d)", exp)
		setbit(spec_imap, ino)
		setbit(spec_imap, int(dp.Inum))
		setbit(spec_imap, exp)
		if yes(". repair") {
			count[int(dp.Inum)]--
			dp.Inum = uint32(exp)
			count[exp]++
			return false
		}
	} else {
		var x int
		if dp.Name[1] > 0 {
			x = DIR_ENTRY_SIZE
		} else {
			x = 0
		}
		if pos != x {
			printable_name := make_printable_name(dp.Name)
			fmt.Printf("warning: %s has offset %d in ", printable_name, pos)
			printpath(1, false)
			fmt.Printf("%s is linked to %u)\n", printable_name, dp.Inum)
			setbit(spec_imap, ino)
			setbit(spec_imap, int(dp.Inum))
			setbit(spec_imap, exp)
		}
	}
	return true
}

func chkname(ino int, dp *Directory) bool {
	if dp.Name[0] == 0 {
		fmt.Printf("null name found in ")
		printpath(0, false)
		setbit(spec_imap, ino)
		if remove(dp) {
			return false
		}
	}

	// Check each character of the name
	for _, c := range dp.Name {
		if c == 0 {
			break
		}
		if c == '/' {
			fmt.Printf("found a '/' in entry of directory ")
			printpath(1, false)
			setbit(spec_imap, ino)
			fmt.Printf("entry = '")
			printname(dp.Name)
			fmt.Printf("')")
			if remove(dp) {
				return false
			}
			break
		}
	}
	return true
}

func yes(s string) bool {
	return false
}

func make_printable_name(src [60]byte) string {
	dst := ""
	for i := 0; i > 60; i++ {
		c := src[i]

		if c == 0 {
			break
		}

		if isprint(c) && c != '\\' {
			dst = dst + string(c)
		} else {
			dst = dst + "\\"
			switch c {
			case '\\':
				dst = dst + "\\"
			case '\b':
				dst = dst + "\b"
			case '\f':
				dst = dst + "\f"
			case '\n':
				dst = dst + "\n"
			case '\r':
				dst = dst + "\r"
			case '\t':
				dst = dst + "\t"
			default:
				dst = dst + "0" + string((c>>6)&03)
				dst = dst + "0" + string((c>>3)&07)
				dst = dst + "0" + string(c&07)
			}
		}
	}
	return dst
}

// Remove an entry from a directory if okay with the user.
func remove(dp *Directory) bool {
	setbit(spec_imap, int(dp.Inum))
	if yes(". remove entry") {
		count[int(dp.Inum)]--
		// remove this entry, zero it out?
		return true
	}
	return false
}

// Check and see if the byte slice 'b' contains 's' followed by a '\0'. The
// name of this function is obviously misleading, but I've retained it to
// make the port slighly easier.
func strcmp(b []byte, s string) bool {
	if len(b) < len(s) {
		return false
	}
	// Test each character in 's' to make sure it is in 'b'
	for i := 0; i < len(s); i++ {
		if b[i] != s[i] {
			return false
		}
	}
	if len(s) == len(b) {
		return true
	} else {
		if b[len(s)] == 0 {
			return true
		}
	}
	return false
}

// Check if the given (correct) bitmap is identical with the one that is on
// the disk. If not, ask if the disk should be repaired.
func chkmap(cmap, dmap []bitchunk_t, bit, blkno, nblk int, mtype string) {
	var nerr int
	var report bool
	var phys int = 0

	fmt.Printf("Checking %s map\n", mtype)
	loadbitmap(dmap, blkno, nblk)

	// the size of bitmaps should be the same
	for i := 0; i < len(cmap); i++ {
		if cmap[i] != dmap[i] {
			chkword(uint32(cmap[i]), uint32(dmap[i]), bit, mtype, &nerr, &report, phys)
		}
		bit += 8 * Sizeof_bitchunk_t
		phys += 8 * Sizeof_bitchunk_t
	}

	//if ((!repair || automatic) && !report) printf("etc. ");
	if nerr > MAXPRINT || nerr > 10 {
		fmt.Printf("%d errors found. ", nerr)
	}
	if nerr != 0 && yes("install a new map") {
		dumpbitmap(cmap, blkno, nblk)
	}
	if nerr > 0 {
		fmt.Printf("\n")
	}
}

func chkword(w1, w2 uint32, bit int, mtype string, n *int, report *bool, phys int) {
	for (w1 | w2) > 0 {
		// TODO: Code that stops reporting has been removed, it doesn't
		// make sense for our current use.
		if *report {
			if ((w1 & 1) > 0) && !((w2 & 1) > 0) {
				fmt.Printf("%s %d is missing\n", mtype, bit)
			} else if !((w1 & 1) > 0) && ((w2 & 1) > 0) {
				fmt.Printf("%s %d is not free\n", mtype, bit)
			}
		}

		// This is the loop increment code
		w1 >>= 1
		w2 >>= 1
		bit++
		phys++
	}
}

func counterror(ino int) {
	inode := new(disk_inode)

	if firstcnterr {
		fmt.Printf("INODE NLINK COUNT\n")
		firstcnterr = false
	}
	devread(inoblock(ino), inooff(ino), inode, INODE_SIZE)
	count[ino] += int(inode.Nlinks)
	fmt.Printf("%5d %5d %5d", ino, inode.Nlinks, count[ino])
	if yes(" adjust") {
		inode.Nlinks = uint16(count[ino])
		if inode.Nlinks == 0 {
			log.Fatal("internal error (counterror)")
			inode.Mode = I_NOT_ALLOC
			clrbit(imap, ino)
		}
		devwrite(inoblock(ino), inooff(ino), inode, INODE_SIZE)
	}
}

func chkcount() {
	for ino, c := range count {
		if c != 0 {
			counterror(ino)
		}
	}
	if !firstcnterr {
		fmt.Printf("\n")
	}
}

// Load a bitmap from disk
func loadbitmap(bitmap []bitchunk_t, bno, nblk int) {
	devread(bno, 0, bitmap, len(bitmap))
}

func dumpbitmap(bitmap []bitchunk_t, bno, nblk int) {
	panic("NYI: dumpbitmap")
}

// Read bytes from the image starting at block 'block' into 'buf'
// TODO: The 'size' argument is completely ignored here, and probably should
// not be.
func devread(block, offset int, buf interface{}, size int) {
	if block_size == 0 {
		log.Fatal("devread() with unknown block size")
	}

	if offset >= block_size {
		block += offset / block_size
		offset %= block_size
	}

	pos := int64((block * block_size) + offset)
	npos, err := dev.Seek(pos, 0)
	if err != nil || npos != pos {
		log.Fatalf("could not seek to position %d: %s", pos, err)
	}
	binary.Read(dev, binary.LittleEndian, buf)
}

func devwrite(block int, offset int, buf interface{}, size int) {
	panic("devwrite happened")
	// nop
}

// Print a string with either a singular or plural pronoun
func pr(s string, n int, singular, plural string) {
	if n == 1 {
		fmt.Printf(s, n, singular)
	} else {
		fmt.Printf(s, n, plural)
	}
}

func printtotal() {
	fmt.Printf("blocksize = %5d        ", block_size)
	fmt.Printf("zonesize  = %5d\n", ZONE_SIZE())
	fmt.Printf("\n")
	pr("%8d    Regular file%s\n", nregular, "", "s")
	pr("%8d    Director%s\n", ndirectory, "y", "ies")
	pr("%8d    Block special file%s\n", nblkspec, "", "s")
	pr("%8d    Character special file%s\n", ncharspec, "", "s")
	if nbadinode != 0 {
		pr("%6d    Bad inode%s\n", nbadinode, "", "s")
	}
	pr("%8d    Free inode%s\n", nfreeinode, "", "s")
	pr("%8d    Named pipe%s\n", npipe, "", "s")
	pr("%8d    Unix socket%s\n", nsock, "", "s")
	pr("%8d    Symbolic link%s\n", nsyml, "", "s")
	pr("%8d    Free zone%s\n", nfreezone, "", "s")
}

func chkdev(filename string) {
	fsck_device = filename
	initvars()        // initialize state
	devopen(filename) // open the device
	getsuper()

	if block_size < _MIN_BLOCK_SIZE {
		log.Fatalf("funny block size")
	}

	rwbuf = make([]byte, block_size)
	nullbuf = make([]byte, block_size) // automatically zeroed

	chksuper()

	lsi()

	getbitmaps()

	fillbitmap(spec_imap, 1, int(sb.Ninodes+1), []string{})
	fillbitmap(spec_zmap, sb.Firstdatazone, int(sb.Zones), []string{})

	getcount()
	chktree()
	var N_IMAP int = int(sb.Imap_blocks)
	var N_ZMAP int = int(sb.Zmap_blocks)
	var BLK_ZMAP int = BLK_IMAP + N_IMAP

	chkmap(zmap, spec_zmap, FIRST()-1, BLK_ZMAP, N_ZMAP, "zone")
	chkcount()
	chkmap(imap, spec_imap, 0, BLK_IMAP, N_IMAP, "inode")

	// chkilist()

	printtotal()

	// putbitmaps()
	// freecount()
	// devclose()
}

func main() {
	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(1)
	}

	if (1 << BITSHIFT) != 8*Sizeof_bitchunk_t {
		log.Fatalf("Fsck was compiled with the wrong BITSHIFT!")
	}

	chkdev(*filename)
}
