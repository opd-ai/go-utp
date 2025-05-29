package utp

// packet represents a uTP packet
type packet struct {
	header Header
	data   []byte
}