package fs

type server_FS struct {
	in chan reqFS
	out chan resFS
}

func (s *server_FS) loop() {
	alive := true
	for alive {
		req := <-s.in
		switch req := req.(type) {
		case req_FS_Mount:
			// Code here
		case req_FS_Unmount:
			// Code here
		case req_FS_Sync:
			// Code here
		case req_FS_Shutdown:
			// Code here
		case req_FS_Fork:
			// Code here
		case req_FS_Exit:
			// Code here
		case req_FS_Open:
			// Code here
		case req_FS_Creat:
			// Code here
		case req_FS_Close:
			// Code here
		case req_FS_Stat:
			// Code here
		case req_FS_Chmod:
			// Code here
		case req_FS_Link:
			// Code here
		case req_FS_Unlink:
			// Code here
		case req_FS_Mkdir:
			// Code here
		case req_FS_Rmdir:
			// Code here
		case req_FS_Chdir:
			// Code here
		default:
			// This can be removed when you utilize 'req'
			_ = req
		}
	}
}
