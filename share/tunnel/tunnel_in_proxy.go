package tunnel

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/jpillora/chisel/share/cio"
	"github.com/jpillora/chisel/share/settings"
	"github.com/jpillora/sizestr"
	"golang.org/x/crypto/ssh"
)

//sshTunnel exposes a subset of Tunnel to subtypes
type sshTunnel interface {
	getSSH(ctx context.Context) ssh.Conn
	IsInbound() bool
}

//Proxy is the inbound portion of a Tunnel
type Proxy struct {
	*cio.Logger
	sshTun sshTunnel
	id     int
	count  int
	remote *settings.Remote
	dialer net.Dialer
	tcp    *net.TCPListener
	udp    *udpListener
	mu     sync.Mutex
	// Enhanced connection management
	connPool     chan struct{}
	maxConns     int
	activeConns  int32
	connStats    *ConnectionStats
}

// ConnectionStats tracks proxy connection statistics
type ConnectionStats struct {
	TotalConnections int64
	ActiveConnections int32
	FailedConnections int64
	BytesSent        int64
	BytesReceived    int64
}

//NewProxy creates a Proxy
func NewProxy(logger *cio.Logger, sshTun sshTunnel, index int, remote *settings.Remote) (*Proxy, error) {
	id := index + 1
	p := &Proxy{
		Logger:    logger.Fork("proxy#%s", remote.String()),
		sshTun:    sshTun,
		id:        id,
		remote:    remote,
		maxConns:  100, // Maximum concurrent connections
		connPool:  make(chan struct{}, 100),
		connStats: &ConnectionStats{},
	}
	return p, p.listen()
}

func (p *Proxy) listen() error {
	if p.remote.Reverse && !p.sshTun.IsInbound() {
		// For reverse proxies, we don't listen locally on the server side
		// The client will listen locally and forward connections through SSH
		p.Infof("Reverse proxy configured")
		return nil
	}
	if p.remote.Stdio {
		//TODO check if pipes active?
	} else if p.remote.LocalProto == "tcp" {
		addr, err := net.ResolveTCPAddr("tcp", p.remote.LocalHost+":"+p.remote.LocalPort)
		if err != nil {
			return p.Errorf("resolve: %s", err)
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return p.Errorf("tcp: %s", err)
		}
		p.Infof("Listening")
		p.tcp = l
	} else if p.remote.LocalProto == "udp" {
		l, err := listenUDP(p.Logger, p.sshTun, p.remote)
		if err != nil {
			return err
		}
		p.Infof("Listening")
		p.udp = l
	} else {
		return p.Errorf("unknown local proto")
	}
	return nil
}

//Run enables the proxy and blocks while its active,
//close the proxy by cancelling the context.
func (p *Proxy) Run(ctx context.Context) error {
	if p.remote.Stdio {
		return p.runStdio(ctx)
	} else if p.remote.LocalProto == "tcp" {
		return p.runTCP(ctx)
	} else if p.remote.LocalProto == "udp" {
		return p.udp.run(ctx)
	}
	panic("should not get here")
}

func (p *Proxy) runStdio(ctx context.Context) error {
	defer p.Infof("Closed")
	for {
		p.pipeRemote(ctx, cio.Stdio)
		select {
		case <-ctx.Done():
			return nil
		default:
			// the connection is not ready yet, keep waiting
		}
	}
}

func (p *Proxy) runTCP(ctx context.Context) error {
	if p.remote.Reverse && !p.sshTun.IsInbound() {
		// For reverse proxies, we don't run TCP listener on the server side
		// The client will handle the local listening and forwarding
		<-ctx.Done()
		return nil
	}
	
	done := make(chan struct{})
	//implements missing net.ListenContext
	go func() {
		select {
		case <-ctx.Done():
			p.tcp.Close()
		case <-done:
		}
	}()
	
	// Use worker pool for better concurrency control
	for i := 0; i < 10; i++ {
		go p.connectionWorker(ctx, done)
	}
	
	for {
		src, err := p.tcp.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				//listener closed
				err = nil
			default:
				p.Infof("Accept error: %s", err)
			}
			close(done)
			return err
		}
		
		// Use connection pool to limit concurrent connections
		select {
		case p.connPool <- struct{}{}:
			go p.pipeRemote(ctx, src)
		default:
			p.Debugf("Connection pool full, rejecting connection")
			src.Close()
		}
	}
}

// connectionWorker handles connections from the pool
func (p *Proxy) connectionWorker(ctx context.Context, done chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-p.connPool:
			// Worker ready for next connection
		}
	}
}

func (p *Proxy) pipeRemote(ctx context.Context, src io.ReadWriteCloser) {
	defer src.Close()
	defer func() {
		// Release connection from pool
		select {
		case <-p.connPool:
		default:
		}
	}()

	p.mu.Lock()
	p.count++
	cid := p.count
	p.mu.Unlock()

	l := p.Fork("conn#%d", cid)
	l.Debugf("Open")
	
	// Update connection statistics
	atomic.AddInt64(&p.connStats.TotalConnections, 1)
	atomic.AddInt32(&p.connStats.ActiveConnections, 1)
	defer atomic.AddInt32(&p.connStats.ActiveConnections, -1)
	
	sshConn := p.sshTun.getSSH(ctx)
	if sshConn == nil {
		l.Debugf("No remote connection")
		atomic.AddInt64(&p.connStats.FailedConnections, 1)
		return
	}
	//ssh request for tcp connection for this proxy's remote
	dst, reqs, err := sshConn.OpenChannel("chisel", []byte(p.remote.Remote()))
	if err != nil {
		l.Infof("Stream error: %s", err)
		atomic.AddInt64(&p.connStats.FailedConnections, 1)
		return
	}
	go ssh.DiscardRequests(reqs)
	//then pipe
	s, r := cio.Pipe(src, dst)
	
	// Update traffic statistics
	atomic.AddInt64(&p.connStats.BytesSent, s)
	atomic.AddInt64(&p.connStats.BytesReceived, r)
	
	l.Debugf("Close (sent %s received %s)", sizestr.ToString(s), sizestr.ToString(r))
}
