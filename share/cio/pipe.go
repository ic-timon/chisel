package cio

import (
	"io"
	"log"
	"math/rand"
	"sync"
	"time"
)

// Chunk size range for packet chunking (default: 1KB-32KB, highest level)
const (
	chunkSizeMin = 1 * 1024  // 1KB
	chunkSizeMax = 32 * 1024 // 32KB
	chunkDelayMin = 0 * time.Millisecond
	chunkDelayMax = 20 * time.Millisecond
)

// chunkedCopy copies data in randomized chunks to simulate real network behavior
func chunkedCopy(dst io.Writer, src io.Reader, rng *rand.Rand) (int64, error) {
	buf := make([]byte, chunkSizeMax)
	var total int64
	
	for {
		// Determine chunk size: random between min and max (normal distribution approximation)
		chunkSize := chunkSizeMin + rng.Intn(chunkSizeMax-chunkSizeMin+1)
		
		nr, er := src.Read(buf[:chunkSize])
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = io.ErrShortWrite
				}
			}
			total += int64(nw)
			if ew != nil {
				return total, ew
			}
			if nr != nw {
				return total, io.ErrShortWrite
			}
			
			// Add random delay between chunks (0-20ms, highest level)
			delayRange := chunkDelayMax - chunkDelayMin
			if delayRange > 0 {
				delay := time.Duration(rng.Int63n(int64(delayRange))) + chunkDelayMin
				if delay > 0 {
					time.Sleep(delay)
				}
			}
		}
		if er != nil {
			if er != io.EOF {
				return total, er
			}
			break
		}
	}
	return total, nil
}

func Pipe(src io.ReadWriteCloser, dst io.ReadWriteCloser) (int64, int64) {
	var sent, received int64
	var wg sync.WaitGroup
	var o sync.Once
	close := func() {
		src.Close()
		dst.Close()
	}
	
	// Use randomized chunking for more realistic traffic patterns
	// Default: enabled at highest level
	rng1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng2 := rand.New(rand.NewSource(time.Now().UnixNano() + 1))
	
	wg.Add(2)
	go func() {
		received, _ = chunkedCopy(src, dst, rng1)
		o.Do(close)
		wg.Done()
	}()
	go func() {
		sent, _ = chunkedCopy(dst, src, rng2)
		o.Do(close)
		wg.Done()
	}()
	wg.Wait()
	return sent, received
}

const vis = false

type pipeVisPrinter struct {
	name string
}

func (p pipeVisPrinter) Write(b []byte) (int, error) {
	log.Printf(">>> %s: %x", p.name, b)
	return len(b), nil
}

func pipeVis(name string, r io.Reader) io.Reader {
	if vis {
		return io.TeeReader(r, pipeVisPrinter{name})
	}
	return r
}
