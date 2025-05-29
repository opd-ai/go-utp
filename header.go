package utp

import (
	"encoding/binary"
	"errors"
)

// Header represents a uTP packet header
type Header struct {
	Type      uint8
	Version   uint8
	Extension uint8
	ConnID    uint16
	Timestamp uint32
	TimeDiff  uint32
	WndSize   uint32
	SeqNr     uint16
	AckNr     uint16
}

// Marshal serializes the header to bytes
func (h *Header) Marshal() []byte {
	buf := make([]byte, HEADER_SIZE)
	buf[0] = (h.Type << 4) | h.Version
	buf[1] = h.Extension
	binary.BigEndian.PutUint16(buf[2:4], h.ConnID)
	binary.BigEndian.PutUint32(buf[4:8], h.Timestamp)
	binary.BigEndian.PutUint32(buf[8:12], h.TimeDiff)
	binary.BigEndian.PutUint32(buf[12:16], h.WndSize)
	binary.BigEndian.PutUint16(buf[16:18], h.SeqNr)
	binary.BigEndian.PutUint16(buf[18:20], h.AckNr)
	return buf
}

// Unmarshal deserializes the header from bytes
func (h *Header) Unmarshal(data []byte) error {
	if len(data) < HEADER_SIZE {
		return errors.New("insufficient data for header")
	}
	h.Type = (data[0] >> 4) & 0xF
	h.Version = data[0] & 0xF
	h.Extension = data[1]
	h.ConnID = binary.BigEndian.Uint16(data[2:4])
	h.Timestamp = binary.BigEndian.Uint32(data[4:8])
	h.TimeDiff = binary.BigEndian.Uint32(data[8:12])
	h.WndSize = binary.BigEndian.Uint32(data[12:16])
	h.SeqNr = binary.BigEndian.Uint16(data[16:18])
	h.AckNr = binary.BigEndian.Uint16(data[18:20])
	return nil
}