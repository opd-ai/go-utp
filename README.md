# uTP (Micro Transport Protocol) Library for Go

A minimal implementation of the Micro Transport Protocol (uTP) in pure Go, providing reliable, ordered delivery over UDP. Implements standard `net.Conn` and `net.Listener` interfaces for drop-in compatibility with existing Go code.

## Features

- Pure Go (no CGO dependencies)
- Implements `net.Conn` and `net.Listener` interfaces
- Reliable, ordered packet delivery
- Basic congestion control
- Thread-safe

## Installation

```bash
go get github.com/yourusername/utp
```

## Usage

### Server
```go
listener, err := utp.Listen("utp", ":9000")
conn, err := listener.Accept()
data := make([]byte, 1024)
n, err := conn.Read(data)
```

### Client
```go
conn, err := utp.Dial("utp", "127.0.0.1:9000")
_, err = conn.Write([]byte("Hello"))
```

## API

- `Listen(network, address string) (net.Listener, error)` - Create a uTP listener
- `Dial(network, address string) (net.Conn, error)` - Connect to a uTP server

Both functions return standard Go networking interfaces, ensuring compatibility with existing code that accepts `net.Conn` or `net.Listener`.

## Implementation Details

- 20-byte uTP header with sequence numbers and acknowledgments
- Fixed retransmission timeout (500ms)
- Basic windowing for flow control
- Connection states: IDLE → SYN_SENT → CONNECTED → CLOSED

## Limitations

This is a minimal implementation:
- No adaptive timeouts
- Fixed window size
- No SACK support

## License

MIT License