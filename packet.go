package utp

import "time"

// packet represents a uTP packet
type packet struct {
	header Header
	data   []byte
}

// pendingPacket tracks packets awaiting acknowledgment
type pendingPacket struct {
	packet    packet
	data      []byte
	timestamp time.Time
	retries   int
}
