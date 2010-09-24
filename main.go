package main

const (
	M1			= 1
	M3			= 3
	M4			= 4
	M3_STRING	= 14
)

type mess_1 struct {m1i1, m1i2, m1i3 int32; m1p1, m1p2, m1p3 *[]byte}
type mess_2 struct {m2i1, m2i2, m2i3 int32; m2l1, m2l2 int64; m2p1 *[]byte}
type mess_3 struct {m3i1, m3i2 int32; m3p1 []byte; m3ca1 [M3_STRING]byte}
type mess_4 struct {m4l1, m4l2, m4l3, m4l4, m4l5 int64}
type mess_5 struct {m5c1, m5c2 int8; m5i1, m5i2 int32; m5l1, m5l2, m5l3 int64}
type mess_7 struct {m7i1, m7i2, m7i3, m7i4 int32; m7p1, m7p2 *[]byte}
type mess_8 struct {m8i1, m8i2 int32; m8p1, m8p2, m8p3, m8p4 *[]byte}

// TODO: Need to figure out how to store a message in this struct
type ipc_message struct {
	m_source int		// the source of the message
	m_type int			// what kind of message it is
	m_u interface{}		// the message
}

func main() {
	for {
		// Receive work
		// Look up caller in process table to determine permissions
		// Do some sanity checking of input
		// Perform work
		// Return results
	}
}
