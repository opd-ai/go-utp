package utp

import (
    "bytes"
    "errors"
    "math/rand"
    "net"
    "sync"
)

// Listener implements net.Listener interface for uTP
type Listener struct {
    pconn    net.PacketConn
    addr     net.Addr
    connMap  sync.Map
    acceptCh chan net.Conn
    closeCh  chan struct{}
}

// Accept implements net.Listener
func (l *Listener) Accept() (net.Conn, error) {
    select {
    case conn := <-l.acceptCh:
        return conn, nil
    case <-l.closeCh:
        return nil, errors.New("listener closed")
    }
}

// Close implements net.Listener
func (l *Listener) Close() error {
    close(l.closeCh)
    return l.pconn.Close()
}

// Addr implements net.Listener
func (l *Listener) Addr() net.Addr {
    return l.addr
}

// acceptLoop handles incoming connections
func (l *Listener) acceptLoop() {
    buffer := make([]byte, PACKET_SIZE)

    for {
        select {
        case <-l.closeCh:
            return
        default:
        }

        n, addr, err := l.pconn.ReadFrom(buffer)
        if err != nil {
            continue
        }

        var header Header
        if err := header.Unmarshal(buffer[:n]); err != nil {
            continue
        }

        if header.Type == ST_SYN {
            // Create new connection
            conn := &Conn{
                pconn:         l.pconn,
                localAddr:     l.addr,
                remoteAddr:    addr,
                connID:        uint16(rand.Uint32()),
                remoteID:      header.ConnID,
                seqNr:         uint16(rand.Uint32()),
                ackNr:         header.SeqNr,
                windowSize:    DEFAULT_WINDOW_SIZE,
                state:         stateConnected,
                receiveBuffer: make(map[uint16][]byte),
                readBuffer:    bytes.NewBuffer(nil),
                closeCh:       make(chan struct{}),
                errorCh:       make(chan error, 1),
                connectedCh:   make(chan struct{}), // Initialize the new channel
            }
            conn.readCond = sync.NewCond(&conn.mu)
            conn.writeCond = sync.NewCond(&conn.mu)

            // Send SYN-ACK
            conn.sendSynAck()

            // Start receive loop
            go conn.receiveLoop()

            // Store connection
            l.connMap.Store(addr.String(), conn)

            select {
            case l.acceptCh <- conn:
            case <-l.closeCh:
                return
            }
        }
    }
}