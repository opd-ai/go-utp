package utp

import (
    "bytes"
    "fmt"
    "io"
    "log"
    "net"
    "os"
    "sync"
    "testing"
    "time"
)

// Test logger setup
var testLogger *log.Logger

func init() {
    testLogger = log.New(os.Stdout, "[UTP-TEST] ", log.LstdFlags|log.Lmicroseconds)
}

func TestBasicConnection(t *testing.T) {
    testLogger.Println("=== Starting TestBasicConnection ===")
    
    // Start a listener
    testLogger.Println("Creating listener on 127.0.0.1:0")
    listener, err := Listen("utp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("Failed to create listener: %v", err)
    }
    defer func() {
        testLogger.Println("Closing listener")
        listener.Close()
    }()

    // Get the actual address the listener is bound to
    addr := listener.Addr().String()
    testLogger.Printf("Listener bound to address: %s", addr)

    // Channel to collect any server errors
    serverErr := make(chan error, 1)
    serverData := make(chan []byte, 1)

    // Start server goroutine
    testLogger.Println("Starting server goroutine")
    go func() {
        testLogger.Println("Server: Waiting for connection...")
        conn, err := listener.Accept()
        if err != nil {
            testLogger.Printf("Server: Accept failed: %v", err)
            serverErr <- err
            return
        }
        defer func() {
            testLogger.Println("Server: Closing connection")
            conn.Close()
        }()

        testLogger.Printf("Server: Accepted connection from %s to %s", conn.RemoteAddr(), conn.LocalAddr())

        // Read data from client
        testLogger.Println("Server: Reading data from client...")
        buffer := make([]byte, 1024)
        n, err := conn.Read(buffer)
        if err != nil {
            testLogger.Printf("Server: Read failed: %v", err)
            serverErr <- err
            return
        }
        testLogger.Printf("Server: Read %d bytes: %q", n, buffer[:n])
        serverData <- buffer[:n]

        // Echo data back to client
        testLogger.Println("Server: Echoing data back to client...")
        written, err := conn.Write(buffer[:n])
        if err != nil {
            testLogger.Printf("Server: Write failed: %v", err)
            serverErr <- err
            return
        }
        testLogger.Printf("Server: Wrote %d bytes back to client", written)
    }()

    // Give server time to start
    testLogger.Println("Waiting 100ms for server to start...")
    time.Sleep(100 * time.Millisecond)

    // Connect as client
    testLogger.Printf("Client: Connecting to %s", addr)
    conn, err := Dial("utp", addr)
    if err != nil {
        t.Fatalf("Failed to dial: %v", err)
    }
    defer func() {
        testLogger.Println("Client: Closing connection")
        conn.Close()
    }()

    testLogger.Printf("Client: Connected from %s to %s", conn.LocalAddr(), conn.RemoteAddr())

    // Send test data
    testData := []byte("Hello, uTP!")
    testLogger.Printf("Client: Sending test data: %q", testData)
    written, err := conn.Write(testData)
    if err != nil {
        t.Fatalf("Failed to write: %v", err)
    }
    testLogger.Printf("Client: Wrote %d bytes", written)

    // Read echoed data
    testLogger.Println("Client: Reading echoed data...")
    buffer := make([]byte, 1024)
    n, err := conn.Read(buffer)
    if err != nil {
        t.Fatalf("Failed to read: %v", err)
    }
    testLogger.Printf("Client: Read %d bytes: %q", n, buffer[:n])

    // Verify data
    if !bytes.Equal(testData, buffer[:n]) {
        t.Fatalf("Data mismatch: expected %q, got %q", testData, buffer[:n])
    }
    testLogger.Println("Client: Data verification successful")

    // Check for server errors
    testLogger.Println("Checking for server completion...")
    select {
    case err := <-serverErr:
        t.Fatalf("Server error: %v", err)
    case receivedData := <-serverData:
        if !bytes.Equal(testData, receivedData) {
            t.Fatalf("Server received wrong data: expected %q, got %q", testData, receivedData)
        }
        testLogger.Printf("Server received correct data: %q", receivedData)
    case <-time.After(1 * time.Second):
        t.Fatal("Timeout waiting for server to receive data")
    }

    testLogger.Println("=== TestBasicConnection completed successfully ===")
}

func TestMultipleConnections(t *testing.T) {
    testLogger.Println("=== Starting TestMultipleConnections ===")
    
    // Start a listener
    testLogger.Println("Creating listener on 127.0.0.1:0")
    listener, err := Listen("utp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("Failed to create listener: %v", err)
    }
    defer func() {
        testLogger.Println("Closing listener")
        listener.Close()
    }()

    addr := listener.Addr().String()
    testLogger.Printf("Listener bound to address: %s", addr)
    
    numConnections := 3
    testLogger.Printf("Planning to create %d concurrent connections", numConnections)
    var wg sync.WaitGroup

    // Start server to handle multiple connections
    testLogger.Println("Starting server goroutine for multiple connections")
    go func() {
        for i := 0; i < numConnections; i++ {
            testLogger.Printf("Server: Waiting for connection #%d", i)
            conn, err := listener.Accept()
            if err != nil {
                testLogger.Printf("Server: Failed to accept connection %d: %v", i, err)
                t.Errorf("Failed to accept connection %d: %v", i, err)
                return
            }

            testLogger.Printf("Server: Accepted connection #%d from %s", i, conn.RemoteAddr())

            go func(connNum int, c net.Conn) {
                defer func() {
                    testLogger.Printf("Server: Closing connection #%d", connNum)
                    c.Close()
                }()
                
                testLogger.Printf("Server: Handler #%d reading data...", connNum)
                buffer := make([]byte, 1024)
                n, err := c.Read(buffer)
                if err != nil {
                    testLogger.Printf("Server: Connection %d read error: %v", connNum, err)
                    t.Errorf("Connection %d read error: %v", connNum, err)
                    return
                }
                testLogger.Printf("Server: Connection #%d read %d bytes: %q", connNum, n, buffer[:n])
                
                // Echo back
                testLogger.Printf("Server: Connection #%d echoing data back...", connNum)
                written, err := c.Write(buffer[:n])
                if err != nil {
                    testLogger.Printf("Server: Connection %d write error: %v", connNum, err)
                    t.Errorf("Connection %d write error: %v", connNum, err)
                    return
                }
                testLogger.Printf("Server: Connection #%d wrote %d bytes back", connNum, written)
            }(i, conn)
        }
        testLogger.Println("Server: All connection handlers started")
    }()

    // Give server time to start
    testLogger.Println("Waiting 100ms for server to start...")
    time.Sleep(100 * time.Millisecond)

    // Create multiple client connections
    testLogger.Printf("Starting %d client connections", numConnections)
    for i := 0; i < numConnections; i++ {
        wg.Add(1)
        go func(connNum int) {
            defer wg.Done()

            testLogger.Printf("Client #%d: Connecting to %s", connNum, addr)
            conn, err := Dial("utp", addr)
            if err != nil {
                testLogger.Printf("Client #%d: Dial failed: %v", connNum, err)
                t.Errorf("Connection %d failed to dial: %v", connNum, err)
                return
            }
            defer func() {
                testLogger.Printf("Client #%d: Closing connection", connNum)
                conn.Close()
            }()

            testLogger.Printf("Client #%d: Connected from %s to %s", connNum, conn.LocalAddr(), conn.RemoteAddr())

            testData := []byte(fmt.Sprintf("Test data from connection %d", connNum))
            
            // Write data
            testLogger.Printf("Client #%d: Sending data: %q", connNum, testData)
            written, err := conn.Write(testData)
            if err != nil {
                testLogger.Printf("Client #%d: Write failed: %v", connNum, err)
                t.Errorf("Connection %d failed to write: %v", connNum, err)
                return
            }
            testLogger.Printf("Client #%d: Wrote %d bytes", connNum, written)

            // Read echo
            testLogger.Printf("Client #%d: Reading echo...", connNum)
            buffer := make([]byte, 1024)
            n, err := conn.Read(buffer)
            if err != nil {
                testLogger.Printf("Client #%d: Read failed: %v", connNum, err)
                t.Errorf("Connection %d failed to read: %v", connNum, err)
                return
            }
            testLogger.Printf("Client #%d: Read %d bytes: %q", connNum, n, buffer[:n])

            if !bytes.Equal(testData, buffer[:n]) {
                testLogger.Printf("Client #%d: Data mismatch!", connNum)
                t.Errorf("Connection %d data mismatch: expected %q, got %q", connNum, testData, buffer[:n])
                return
            }
            testLogger.Printf("Client #%d: Data verification successful", connNum)
        }(i)
    }

    testLogger.Println("Waiting for all client connections to complete...")
    wg.Wait()
    testLogger.Println("=== TestMultipleConnections completed successfully ===")
}

func TestConnectionTimeout(t *testing.T) {
    testLogger.Println("=== Starting TestConnectionTimeout ===")
    
    // Try to connect to a non-existent server
    testAddr := "127.0.0.1:19999" // Hopefully unused port
    testLogger.Printf("Attempting to connect to non-existent server: %s", testAddr)
    
    start := time.Now()
    testLogger.Printf("Connection attempt started at: %v", start)
    
    _, err := Dial("utp", testAddr)
    duration := time.Since(start)
    
    testLogger.Printf("Connection attempt completed at: %v (duration: %v)", time.Now(), duration)

    if err == nil {
        testLogger.Println("ERROR: Expected connection to fail, but it succeeded")
        t.Fatal("Expected connection to fail, but it succeeded")
    }
    testLogger.Printf("Connection failed as expected with error: %v", err)

    // Should timeout in approximately 5 seconds
    if duration < 4*time.Second || duration > 6*time.Second {
        testLogger.Printf("ERROR: Timeout duration unexpected: %v (expected 4-6 seconds)", duration)
        t.Fatalf("Timeout duration unexpected: %v", duration)
    }
    testLogger.Printf("Timeout duration is within expected range: %v", duration)

    if err.Error() != "connection timeout" {
        testLogger.Printf("ERROR: Expected timeout error, got: %v", err)
        t.Fatalf("Expected timeout error, got: %v", err)
    }
    testLogger.Println("Received expected timeout error message")
    testLogger.Println("=== TestConnectionTimeout completed successfully ===")
}

func TestLargeDataTransfer(t *testing.T) {
    testLogger.Println("=== Starting TestLargeDataTransfer ===")
    
    // Start a listener
    testLogger.Println("Creating listener on 127.0.0.1:0")
    listener, err := Listen("utp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("Failed to create listener: %v", err)
    }
    defer func() {
        testLogger.Println("Closing listener")
        listener.Close()
    }()

    addr := listener.Addr().String()
    testLogger.Printf("Listener bound to address: %s", addr)
    
    testSize := 10 * 1024 // 10KB of data
    testLogger.Printf("Preparing to transfer %d bytes of test data", testSize)

    // Create test data
    testData := make([]byte, testSize)
    for i := range testData {
        testData[i] = byte(i % 256)
    }
    testLogger.Printf("Generated test data pattern (first 20 bytes): %v", testData[:20])

    serverDone := make(chan []byte, 1)
    serverErr := make(chan error, 1)

    // Start server
    testLogger.Println("Starting server for large data transfer")
    go func() {
        testLogger.Println("Server: Waiting for connection...")
        conn, err := listener.Accept()
        if err != nil {
            testLogger.Printf("Server: Accept failed: %v", err)
            serverErr <- err
            return
        }
        defer func() {
            testLogger.Println("Server: Closing connection")
            conn.Close()
        }()

        testLogger.Printf("Server: Accepted connection from %s", conn.RemoteAddr())

        // Read all data
        var received bytes.Buffer
        buffer := make([]byte, 1024)
        readCount := 0
        
        testLogger.Println("Server: Starting to read large data...")
        for received.Len() < testSize {
            n, err := conn.Read(buffer)
            readCount++
            if err != nil {
                if err == io.EOF {
                    testLogger.Printf("Server: Received EOF after %d reads, total bytes: %d", readCount, received.Len())
                    break
                }
                testLogger.Printf("Server: Read error after %d reads: %v", readCount, err)
                serverErr <- err
                return
            }
            received.Write(buffer[:n])
            testLogger.Printf("Server: Read #%d: %d bytes (total: %d/%d)", readCount, n, received.Len(), testSize)
        }

        testLogger.Printf("Server: Finished reading. Total bytes received: %d", received.Len())
        serverDone <- received.Bytes()
    }()

    // Give server time to start
    testLogger.Println("Waiting 100ms for server to start...")
    time.Sleep(100 * time.Millisecond)

    // Connect and send data
    testLogger.Printf("Client: Connecting to %s for large data transfer", addr)
    conn, err := Dial("utp", addr)
    if err != nil {
        t.Fatalf("Failed to dial: %v", err)
    }
    defer func() {
        testLogger.Println("Client: Closing connection")
        conn.Close()
    }()

    testLogger.Printf("Client: Connected from %s to %s", conn.LocalAddr(), conn.RemoteAddr())

    // Send data in chunks
    chunkSize := 512
    testLogger.Printf("Client: Sending data in chunks of %d bytes", chunkSize)
    chunkCount := 0
    totalSent := 0
    
    for i := 0; i < len(testData); i += chunkSize {
        end := i + chunkSize
        if end > len(testData) {
            end = len(testData)
        }
        
        chunkCount++
        written, err := conn.Write(testData[i:end])
        if err != nil {
            testLogger.Printf("Client: Failed to write chunk #%d at offset %d: %v", chunkCount, i, err)
            t.Fatalf("Failed to write chunk at %d: %v", i, err)
        }
        totalSent += written
        testLogger.Printf("Client: Sent chunk #%d: %d bytes (offset %d-%d, total sent: %d)", chunkCount, written, i, end-1, totalSent)
    }

    testLogger.Printf("Client: Finished sending all %d chunks, total bytes: %d", chunkCount, totalSent)

    // Close to signal end of data
    testLogger.Println("Client: Closing connection to signal end of data")
    conn.Close()

    // Wait for server to finish
    testLogger.Println("Waiting for server to finish receiving data...")
    select {
    case receivedData := <-serverDone:
        testLogger.Printf("Server completed. Received %d bytes", len(receivedData))
        if !bytes.Equal(testData, receivedData) {
            testLogger.Printf("ERROR: Data mismatch! Expected %d bytes, got %d bytes", len(testData), len(receivedData))
            // Compare first few bytes for debugging
            if len(receivedData) > 0 && len(testData) > 0 {
                testLogger.Printf("First 20 bytes expected: %v", testData[:min(20, len(testData))])
                testLogger.Printf("First 20 bytes received: %v", receivedData[:min(20, len(receivedData))])
            }
            t.Fatalf("Large data transfer failed: lengths %d vs %d", len(testData), len(receivedData))
        }
        testLogger.Println("Large data transfer verification successful")
    case err := <-serverErr:
        testLogger.Printf("Server error during large transfer: %v", err)
        t.Fatalf("Server error during large transfer: %v", err)
    case <-time.After(10 * time.Second):
        testLogger.Println("ERROR: Timeout during large data transfer")
        t.Fatal("Timeout during large data transfer")
    }
    
    testLogger.Println("=== TestLargeDataTransfer completed successfully ===")
}

func TestConnectionInterfaces(t *testing.T) {
    testLogger.Println("=== Starting TestConnectionInterfaces ===")
    
    // Test that our types properly implement net interfaces
    testLogger.Println("Creating listener to test interfaces")
    listener, err := Listen("utp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("Failed to create listener: %v", err)
    }
    defer func() {
        testLogger.Println("Closing listener")
        listener.Close()
    }()

    // Verify listener implements net.Listener
    var _ net.Listener = listener
    testLogger.Println("✓ Listener implements net.Listener interface")

    // Test address methods
    addr := listener.Addr()
    if addr == nil {
        t.Fatal("Listener address is nil")
    }
    testLogger.Printf("✓ Listener address: %s", addr)

    serverReady := make(chan bool, 1)

    // Start a simple server
    testLogger.Println("Starting interface test server")
    go func() {
        serverReady <- true
        testLogger.Println("Server: Waiting for connection...")
        conn, err := listener.Accept()
        if err != nil {
            testLogger.Printf("Server: Accept failed: %v", err)
            return
        }
        defer func() {
            testLogger.Println("Server: Closing connection")
            conn.Close()
        }()
        
        testLogger.Printf("Server: Accepted connection from %s", conn.RemoteAddr())
        
        // Verify conn implements net.Conn
        var _ net.Conn = conn
        testLogger.Println("✓ Server connection implements net.Conn interface")
        
        // Test address methods
        if conn.LocalAddr() == nil {
            testLogger.Println("ERROR: Server connection local address is nil")
            t.Error("Connection local address is nil")
        } else {
            testLogger.Printf("✓ Server connection local address: %s", conn.LocalAddr())
        }
        if conn.RemoteAddr() == nil {
            testLogger.Println("ERROR: Server connection remote address is nil")
            t.Error("Connection remote address is nil")
        } else {
            testLogger.Printf("✓ Server connection remote address: %s", conn.RemoteAddr())
        }
        
        // Keep connection open for client tests
        time.Sleep(2 * time.Second)
    }()

    // Wait for server to be ready
    <-serverReady
    time.Sleep(100 * time.Millisecond)

    // Connect to test net.Conn interface
    testLogger.Printf("Client: Connecting to %s to test interfaces", listener.Addr().String())
    conn, err := Dial("utp", listener.Addr().String())
    if err != nil {
        t.Fatalf("Failed to dial: %v", err)
    }
    defer func() {
        testLogger.Println("Client: Closing connection")
        conn.Close()
    }()

    testLogger.Printf("Client: Connected from %s to %s", conn.LocalAddr(), conn.RemoteAddr())

    // Verify conn implements net.Conn
    var _ net.Conn = conn
    testLogger.Println("✓ Client connection implements net.Conn interface")

    // Test deadline functionality
    testLogger.Println("Testing deadline functionality...")
    deadline := time.Now().Add(1 * time.Second)
    
    testLogger.Printf("Setting deadline to: %v", deadline)
    err = conn.SetDeadline(deadline)
    if err != nil {
        testLogger.Printf("SetDeadline failed: %v", err)
        t.Fatalf("SetDeadline failed: %v", err)
    }
    testLogger.Println("✓ SetDeadline successful")

    err = conn.SetReadDeadline(deadline)
    if err != nil {
        testLogger.Printf("SetReadDeadline failed: %v", err)
        t.Fatalf("SetReadDeadline failed: %v", err)
    }
    testLogger.Println("✓ SetReadDeadline successful")

    err = conn.SetWriteDeadline(deadline)
    if err != nil {
        testLogger.Printf("SetWriteDeadline failed: %v", err)
        t.Fatalf("SetWriteDeadline failed: %v", err)
    }
    testLogger.Println("✓ SetWriteDeadline successful")
    
    testLogger.Println("=== TestConnectionInterfaces completed successfully ===")
}

func TestInvalidNetwork(t *testing.T) {
    testLogger.Println("=== Starting TestInvalidNetwork ===")
    
    // Test invalid network types
    testLogger.Println("Testing invalid network 'tcp' for Listen")
    _, err := Listen("tcp", "127.0.0.1:0")
    if err == nil {
        testLogger.Println("ERROR: Expected error for invalid network 'tcp', but got none")
        t.Fatal("Expected error for invalid network 'tcp'")
    }
    testLogger.Printf("✓ Listen correctly rejected 'tcp' network: %v", err)

    testLogger.Println("Testing invalid network 'udp' for Dial")
    _, err = Dial("udp", "127.0.0.1:9999")
    if err == nil {
        testLogger.Println("ERROR: Expected error for invalid network 'udp', but got none")
        t.Fatal("Expected error for invalid network 'udp'")
    }
    testLogger.Printf("✓ Dial correctly rejected 'udp' network: %v", err)
    
    testLogger.Println("=== TestInvalidNetwork completed successfully ===")
}

// Helper function for min (for older Go versions)
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

// Benchmark basic connection establishment and data transfer
func BenchmarkConnectionEstablishment(b *testing.B) {
    testLogger.Println("=== Starting BenchmarkConnectionEstablishment ===")
    
    listener, err := Listen("utp", "127.0.0.1:0")
    if err != nil {
        b.Fatalf("Failed to create listener: %v", err)
    }
    defer listener.Close()

    addr := listener.Addr().String()
    testLogger.Printf("Benchmark listener bound to: %s", addr)

    // Start server
    testLogger.Println("Starting benchmark echo server")
    go func() {
        connCount := 0
        for {
            conn, err := listener.Accept()
            if err != nil {
                return
            }
            connCount++
            go func(c net.Conn, num int) {
                defer c.Close()
                if num <= 5 { // Log first few connections
                    testLogger.Printf("Benchmark server: Handling connection #%d", num)
                }
                io.Copy(c, c) // Echo server
            }(conn, connCount)
        }
    }()

    time.Sleep(100 * time.Millisecond)
    testLogger.Printf("Starting %d benchmark iterations", b.N)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        if i < 5 || i%100 == 0 { // Log first few and every 100th iteration
            testLogger.Printf("Benchmark iteration %d/%d", i+1, b.N)
        }
        conn, err := Dial("utp", addr)
        if err != nil {
            b.Fatalf("Dial failed on iteration %d: %v", i, err)
        }
        conn.Close()
    }
    
    testLogger.Printf("Benchmark completed %d iterations", b.N)
    testLogger.Println("=== BenchmarkConnectionEstablishment completed ===")
}