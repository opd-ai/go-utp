// Package utp implements a minimal Micro Transport Protocol (uTP) over UDP.
// It provides reliable, ordered delivery with basic congestion control.
package utp

import (
	"time"
)

// Protocol constants
const (
	// Packet types
	ST_DATA  = 0
	ST_FIN   = 1
	ST_STATE = 2
	ST_RESET = 3
	ST_SYN   = 4

	// Protocol version
	VERSION = 1

	// Header size in bytes
	HEADER_SIZE = 20

	// Default values
	DEFAULT_WINDOW_SIZE = 1024 * 64
	DEFAULT_TIMEOUT     = 500 * time.Millisecond
	MAX_RETRIES         = 5
	PACKET_SIZE         = 1400
)

// Connection states
const (
	stateIdle = iota
	stateSynSent
	stateConnected
	stateFinSent
	stateClosed
)

// timeoutError implements net.Error for timeout errors
type timeoutError struct {
	op string
}

func (e *timeoutError) Error() string   { return e.op + " timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }