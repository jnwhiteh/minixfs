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

var firstlist bool // has the listing header been printed?

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
var nfreezone int
var ztype map[int]int

type stack struct {
	dir      *Directory
	next     *stack
	presence byte
}

var ftop = new(stack)

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
			fmt.Print(s[i])
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
		if !visited && !chkinode(ino, inode) {
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
	fmt.Printf(" %3d", ip.Nlinks)
	switch ip.Mode & I_TYPE {
	case I_CHAR_SPECIAL, I_BLOCK_SPECIAL:
		fmt.Printf("  %2x,%2x ", ip.Zone[0]>>MAJOR&0xFF, ip.Zone[0]>>MINOR&0xFF)
	default:
		fmt.Printf("%7d ", ip.Size)
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
		printpath(2, false)
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
	}
	return true
}

func chkfile(ino int, ip *disk_inode) bool {
	var ok bool
	var i int
	var level int
	pos := 0

	ok = chkzones(ino, ip, &pos, ip.Zone[:], 0)
	i = V2_NR_DZONES
	level = 1
	for i < V2_NR_DZONES {
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

func chkdirzone(ino int, ip *disk_inode, pos, zno int) bool {
	return false
}

func chksymlinkzone(ino int, ip *disk_inode, pos, zno int) bool {
	return false
}

func chkindzone(ino int, ip *disk_inode, pos *int, zno, level int) bool {
	return false
}

func yes(s string) bool {
	return false
}

// Read bytes from the image starting at block 'block' into 'buf'
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
