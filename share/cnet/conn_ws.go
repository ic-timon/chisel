package cnet

import (
	"math/rand"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type wsConn struct {
	*websocket.Conn
	buff []byte
	rng  *rand.Rand
	// Buffer management
	readBufSize  int
	writeBufSize int
}

var (
	// Default packet delay range: 0-100ms (highest level)
	packetDelayMin = 0 * time.Millisecond
	packetDelayMax = 100 * time.Millisecond
)

//NewWebSocketConn converts a websocket.Conn into a net.Conn
func NewWebSocketConn(websocketConn *websocket.Conn) net.Conn {
	c := wsConn{
		Conn:         websocketConn,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
		readBufSize:  32 * 1024,  // 32KB read buffer
		writeBufSize: 64 * 1024,  // 64KB write buffer
	}
	return &c
}

//Read is not threadsafe though thats okay since there
//should never be more than one reader
func (c *wsConn) Read(dst []byte) (int, error) {
	ldst := len(dst)
	//use buffer or read new message
	var src []byte
	if len(c.buff) > 0 {
		src = c.buff
		c.buff = nil
	} else {
		// Set read buffer size for better performance
		c.Conn.SetReadLimit(int64(c.readBufSize))
		if _, msg, err := c.Conn.ReadMessage(); err == nil {
			src = msg
		} else {
			return 0, err
		}
	}
	//copy src->dest
	var n int
	if len(src) > ldst {
		//copy as much as possible of src into dst
		n = copy(dst, src[:ldst])
		//copy remainder into buffer with capacity management
		r := src[ldst:]
		lr := len(r)
		if lr <= c.readBufSize {
			c.buff = make([]byte, lr)
			copy(c.buff, r)
		} else {
			// If buffer too large, only keep reasonable amount
			c.buff = make([]byte, c.readBufSize)
			copy(c.buff, r[:c.readBufSize])
		}
	} else {
		//copy all of src into dst
		n = copy(dst, src)
	}
	//return bytes copied
	return n, nil
}

func (c *wsConn) Write(b []byte) (int, error) {
	// Add randomized delay to make packet timing less predictable
	// Default: 0-100ms delay (highest level)
	// Delay is proportional to packet size to simulate real network behavior
	delayRange := packetDelayMax - packetDelayMin
	if delayRange > 0 {
		// Base delay: random between min and max
		baseDelay := time.Duration(c.rng.Int63n(int64(delayRange))) + packetDelayMin
		// Add small additional delay based on packet size (larger packets = slightly more delay)
		// Scale factor: 0-10% of base delay based on packet size (max 64KB)
		maxPacketSize := 64 * 1024
		sizeFactor := float64(len(b)) / float64(maxPacketSize)
		if sizeFactor > 1.0 {
			sizeFactor = 1.0
		}
		additionalDelay := time.Duration(float64(baseDelay) * sizeFactor * 0.1)
		totalDelay := baseDelay + additionalDelay
		
		// Only add delay if it's significant (> 1ms) to avoid unnecessary overhead
		if totalDelay > 1*time.Millisecond {
			time.Sleep(totalDelay)
		}
	}
	
	// Chunk large writes for better performance and reliability
	maxChunkSize := c.writeBufSize
	if len(b) > maxChunkSize {
		for i := 0; i < len(b); i += maxChunkSize {
			end := i + maxChunkSize
			if end > len(b) {
				end = len(b)
			}
			chunk := b[i:end]
			if err := c.Conn.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
				return i, err
			}
		}
		return len(b), nil
	}
	
	if err := c.Conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	n := len(b)
	return n, nil
}

func (c *wsConn) SetDeadline(t time.Time) error {
	if err := c.Conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.Conn.SetWriteDeadline(t)
}
