package utp

import (
	"time"
	"errors"
)

// sendPackets handles packet transmission with retries
func (c *Conn) sendPackets() {
    c.mu.Lock()
    
    // Create a map to track unacked packets with timestamps
    unackedPackets := make(map[uint16]*pendingPacket)
    
    // Send all packets in buffer
    for i := range c.sendBuffer {
        pkt := &c.sendBuffer[i]
        data := append(pkt.header.Marshal(), pkt.data...)
        
        _, err := c.pconn.WriteTo(data, c.remoteAddr)
        if err != nil {
            c.mu.Unlock()
            select {
            case c.errorCh <- err:
            default:
            }
            return
        }
        
        // Track packet for retransmission
        unackedPackets[pkt.header.SeqNr] = &pendingPacket{
            packet:    *pkt,
            data:      data,
            timestamp: time.Now(),
            retries:   0,
        }
    }
    
    // Clear sent buffer immediately since we're tracking in unackedPackets
    c.sendBuffer = c.sendBuffer[:0]
    c.mu.Unlock()
    
    // Start retransmission loop
    c.retransmitLoop(unackedPackets)
}

// retransmitLoop handles packet retransmission with proper ACK tracking
func (c *Conn) retransmitLoop(unackedPackets map[uint16]*pendingPacket) {
    ticker := time.NewTicker(DEFAULT_TIMEOUT / 2) // Check more frequently than timeout
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            c.mu.Lock()
            now := time.Now()
            
            // Remove packets that have been ACKed
            for seqNr := range unackedPackets {
                if seqNr <= c.lastAckNr {
                    delete(unackedPackets, seqNr)
                }
            }
            
            // Check if all packets are ACKed
            if len(unackedPackets) == 0 {
                c.writeCond.Signal() // Signal that write buffer is clear
                c.mu.Unlock()
                return
            }
            
            // Check for timeouts and retransmit
            for seqNr, pending := range unackedPackets {
                if now.Sub(pending.timestamp) >= DEFAULT_TIMEOUT {
                    if pending.retries >= MAX_RETRIES {
                        // Max retries reached, close connection with error
                        delete(unackedPackets, seqNr)
                        c.state = stateClosed
                        c.mu.Unlock()
                        close(c.closeCh)
                        select {
                        case c.errorCh <- errors.New("max retries exceeded for packet"):
                        default:
                        }
                        return
                    }
                    
                    // Retransmit packet
                    _, err := c.pconn.WriteTo(pending.data, c.remoteAddr)
                    if err != nil {
                        c.mu.Unlock()
                        select {
                        case c.errorCh <- err:
                        default:
                        }
                        return
                    }
                    
                    pending.timestamp = now
                    pending.retries++
                }
            }
            
            c.mu.Unlock()
            
        case <-c.closeCh:
            return
        }
    }
}

// ...existing code...

// processPacket handles incoming packet based on type
func (c *Conn) processPacket(header *Header, data []byte) {
    c.mu.Lock()
    defer c.mu.Unlock()

    switch header.Type {
    case ST_DATA:
        // Store in receive buffer only if not already received
        if _, exists := c.receiveBuffer[header.SeqNr]; !exists {
            c.receiveBuffer[header.SeqNr] = make([]byte, len(data))
            copy(c.receiveBuffer[header.SeqNr], data)
        }

        // Process in-order packets
        for {
            if data, exists := c.receiveBuffer[c.ackNr+1]; exists {
                c.readBuffer.Write(data)
                delete(c.receiveBuffer, c.ackNr+1)
                c.ackNr++
                c.readCond.Signal()
            } else {
                break
            }
        }

        // Send ACK for highest consecutive received packet
        c.sendAck()

    case ST_STATE:
        // Handle SYN-ACK during connection establishment
        if c.state == stateSynSent && header.AckNr == c.seqNr {
            c.remoteID = header.ConnID
            c.ackNr = header.SeqNr
            c.state = stateConnected
            // Signal successful connection
            select {
            case c.connectedCh <- struct{}{}:
            default:
            }
        }
        
        // Process ACK for data packets
        if header.AckNr > c.lastAckNr {
            c.lastAckNr = header.AckNr
            // Signal to retransmission loop that ACKs were received
            // The loop will check and clean up acknowledged packets
        }

    case ST_FIN:
        // Send ACK for FIN
        c.sendAck()
        c.state = stateClosed
        close(c.closeCh)

    case ST_SYN:
        // This should be handled by the listener, not individual connections
        // during normal operation. If received here, it's likely a duplicate.
        if c.state == stateIdle {
            c.remoteID = header.ConnID
            c.ackNr = header.SeqNr
            c.state = stateConnected
            c.sendSynAck()
        }
    }
}
