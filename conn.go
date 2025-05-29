package utp

import (
    "bytes"
    "errors"
    "io"
    "net"
    "sync"
    "time"
)

// Conn implements net.Conn interface for uTP connections
type Conn struct {
    pconn      net.PacketConn
    localAddr  net.Addr
    remoteAddr net.Addr

    mu          sync.RWMutex
    state       int
    connID      uint16
    remoteID    uint16
    seqNr       uint16
    ackNr       uint16
    lastAckNr   uint16
    windowSize  uint32

    sendBuffer    []packet
    receiveBuffer map[uint16][]byte
    readBuffer    *bytes.Buffer

    readDeadline  time.Time
    writeDeadline time.Time

    closeCh     chan struct{}
    errorCh     chan error
    connectedCh chan struct{} // New channel for connection establishment

    // Synchronization
    readCond  *sync.Cond
    writeCond *sync.Cond
}

// Read implements net.Conn
func (c *Conn) Read(b []byte) (n int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()

    for {
        select {
        case <-c.closeCh:
            return 0, io.EOF
        case err := <-c.errorCh:
            return 0, err
        default:
        }

        if c.readBuffer.Len() > 0 {
            return c.readBuffer.Read(b)
        }

        if !c.readDeadline.IsZero() && time.Now().After(c.readDeadline) {
            return 0, &timeoutError{op: "read"}
        }

        c.readCond.Wait()
    }
}

// Write implements net.Conn
func (c *Conn) Write(b []byte) (n int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.state != stateConnected {
        return 0, errors.New("connection not established")
    }

    select {
    case <-c.closeCh:
        return 0, errors.New("connection closed")
    default:
    }

    if !c.writeDeadline.IsZero() && time.Now().After(c.writeDeadline) {
        return 0, &timeoutError{op: "write"}
    }

    // Fragment data into packets
    totalWritten := 0
    for len(b) > 0 {
        size := len(b)
        if size > PACKET_SIZE-HEADER_SIZE {
            size = PACKET_SIZE - HEADER_SIZE
        }

        header := Header{
            Type:      ST_DATA,
            Version:   VERSION,
            ConnID:    c.remoteID,
            Timestamp: uint32(time.Now().UnixMicro()),
            WndSize:   c.windowSize,
            SeqNr:     c.seqNr,
            AckNr:     c.ackNr,
        }

        pkt := packet{
            header: header,
            data:   make([]byte, size),
        }
        copy(pkt.data, b[:size])

        c.sendBuffer = append(c.sendBuffer, pkt)
        c.seqNr++
        b = b[size:]
        totalWritten += size
    }

    // Send packets
    go c.sendPackets()

    return totalWritten, nil
}

// Close implements net.Conn
func (c *Conn) Close() error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.state == stateClosed {
        return nil
    }

    // Send FIN packet
    header := Header{
        Type:      ST_FIN,
        Version:   VERSION,
        ConnID:    c.remoteID,
        Timestamp: uint32(time.Now().UnixMicro()),
        SeqNr:     c.seqNr,
        AckNr:     c.ackNr,
    }

    data := header.Marshal()
    _, err := c.pconn.WriteTo(data, c.remoteAddr)

    c.state = stateClosed
    close(c.closeCh)

    return err
}

// LocalAddr implements net.Conn
func (c *Conn) LocalAddr() net.Addr {
    return c.localAddr
}

// RemoteAddr implements net.Conn
func (c *Conn) RemoteAddr() net.Addr {
    return c.remoteAddr
}

// SetDeadline implements net.Conn
func (c *Conn) SetDeadline(t time.Time) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.readDeadline = t
    c.writeDeadline = t
    return nil
}

// SetReadDeadline implements net.Conn
func (c *Conn) SetReadDeadline(t time.Time) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.readDeadline = t
    return nil
}

// SetWriteDeadline implements net.Conn
func (c *Conn) SetWriteDeadline(t time.Time) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.writeDeadline = t
    return nil
}

// receiveLoop handles incoming packets
func (c *Conn) receiveLoop() {
    buffer := make([]byte, PACKET_SIZE)

    for {
        select {
        case <-c.closeCh:
            return
        default:
        }

        n, addr, err := c.pconn.ReadFrom(buffer)
        if err != nil {
            select {
            case c.errorCh <- err:
            case <-c.closeCh:
            }
            return
        }

        // Verify packet is from expected remote
        if addr.String() != c.remoteAddr.String() {
            continue
        }

        var header Header
        if err := header.Unmarshal(buffer[:n]); err != nil {
            continue
        }

        c.processPacket(&header, buffer[HEADER_SIZE:n])
    }
}

// sendAck sends an acknowledgment packet
func (c *Conn) sendAck() {
    header := Header{
        Type:      ST_STATE,
        Version:   VERSION,
        ConnID:    c.remoteID,
        Timestamp: uint32(time.Now().UnixMicro()),
        WndSize:   c.windowSize,
        SeqNr:     c.seqNr,
        AckNr:     c.ackNr,
    }

    data := header.Marshal()
    c.pconn.WriteTo(data, c.remoteAddr)
}

// sendSynAck sends a SYN-ACK packet
func (c *Conn) sendSynAck() {
    header := Header{
        Type:      ST_STATE,
        Version:   VERSION,
        ConnID:    c.connID,
        Timestamp: uint32(time.Now().UnixMicro()),
        WndSize:   c.windowSize,
        SeqNr:     c.seqNr,
        AckNr:     c.ackNr,
    }

    data := header.Marshal()
    c.pconn.WriteTo(data, c.remoteAddr)
}