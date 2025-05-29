package utp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

// Listen creates a new uTP listener
func Listen(network, address string) (net.Listener, error) {
	if network != "utp" {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	pconn, err := net.ListenPacket("udp", address)
	if err != nil {
		return nil, err
	}

	listener := &Listener{
		pconn:    pconn,
		addr:     pconn.LocalAddr(),
		acceptCh: make(chan net.Conn),
		closeCh:  make(chan struct{}),
	}

	go listener.acceptLoop()

	return listener, nil
}

// Dial establishes a uTP connection
func Dial(network, address string) (net.Conn, error) {
	if network != "utp" {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	// Resolve address
	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	// Create UDP connection
	pconn, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, err
	}

	conn := &Conn{
		pconn:         pconn,
		localAddr:     pconn.LocalAddr(),
		remoteAddr:    raddr,
		connID:        uint16(rand.Uint32()),
		seqNr:         uint16(rand.Uint32()),
		windowSize:    DEFAULT_WINDOW_SIZE,
		state:         stateSynSent,
		receiveBuffer: make(map[uint16][]byte),
		readBuffer:    bytes.NewBuffer(nil),
		closeCh:       make(chan struct{}),
		errorCh:       make(chan error, 1),
	}
	conn.readCond = sync.NewCond(&conn.mu)
	conn.writeCond = sync.NewCond(&conn.mu)

	// Send SYN
	header := Header{
		Type:      ST_SYN,
		Version:   VERSION,
		ConnID:    conn.connID,
		Timestamp: uint32(time.Now().UnixMicro()),
		WndSize:   conn.windowSize,
		SeqNr:     conn.seqNr,
	}

	data := header.Marshal()
	if _, err := pconn.WriteTo(data, raddr); err != nil {
		return nil, err
	}

	// Start receive loop
	go conn.receiveLoop()

	// Wait for connection establishment
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			conn.Close()
			return nil, errors.New("connection timeout")
		case <-ticker.C:
			conn.mu.RLock()
			state := conn.state
			conn.mu.RUnlock()
			if state == stateConnected {
				return conn, nil
			}
		}
	}
}