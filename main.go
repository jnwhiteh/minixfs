package main

type ipc_message struct {
	m_source int		// the source of the message
	m_type int			// what kind of message it is
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
