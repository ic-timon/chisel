package tunnel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/armon/go-socks5"
	"github.com/jpillora/chisel/share/cio"
	"github.com/jpillora/chisel/share/cnet"
	"github.com/jpillora/chisel/share/settings"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
)

//Config a Tunnel
type Config struct {
	*cio.Logger
	Inbound   bool
	Outbound  bool
	Socks     bool
	KeepAlive time.Duration
}

//Tunnel represents an SSH tunnel with proxy capabilities.
//Both chisel client and server are Tunnels.
//chisel client has a single set of remotes, whereas
//chisel server has multiple sets of remotes (one set per client).
//Each remote has a 1:1 mapping to a proxy.
//Proxies listen, send data over ssh, and the other end of the ssh connection
//communicates with the endpoint and returns the response.
type Tunnel struct {
	Config
	//ssh connection
	activeConnMut  sync.RWMutex
	activatingConn waitGroup
	activeConn     ssh.Conn
	//proxies
	proxyCount int
	//internals
	connStats   cnet.ConnCount
	socksServer *socks5.Server
	// Enhanced connection management
	connPool     []ssh.Conn
	connPoolMut  sync.RWMutex
	maxPoolSize  int
	healthCheck  *time.Ticker
	lastActivity time.Time
}

//New Tunnel from the given Config
func New(c Config) *Tunnel {
	c.Logger = c.Logger.Fork("tun")
	t := &Tunnel{
		Config:      c,
		maxPoolSize: 5, // Maximum connection pool size
		connPool:    make([]ssh.Conn, 0),
	}
	t.activatingConn.Add(1)
	//setup socks server (not listening on any port!)
	extra := ""
	if c.Socks {
		sl := log.New(io.Discard, "", 0)
		if t.Logger.Debug {
			sl = log.New(os.Stdout, "[socks]", log.Ldate|log.Ltime)
		}
		t.socksServer, _ = socks5.New(&socks5.Config{Logger: sl})
		extra += " (SOCKS enabled)"
	}
	// Start health check for connection pool
	if c.KeepAlive > 0 {
		t.healthCheck = time.NewTicker(c.KeepAlive / 2)
		go t.connectionPoolHealthCheck()
	}
	t.Debugf("Created%s", extra)
	return t
}

//BindSSH provides an active SSH for use for tunnelling
func (t *Tunnel) BindSSH(ctx context.Context, c ssh.Conn, reqs <-chan *ssh.Request, chans <-chan ssh.NewChannel) error {
	//link ctx to ssh-conn
	go func() {
		<-ctx.Done()
		if c.Close() == nil {
			t.Debugf("SSH cancelled")
		}
		t.activatingConn.DoneAll()
	}()
	//mark active and unblock
	t.activeConnMut.Lock()
	if t.activeConn != nil {
		panic("double bind ssh")
	}
	t.activeConn = c
	t.activeConnMut.Unlock()
	t.activatingConn.Done()
	//optional keepalive loop against this connection
	if t.Config.KeepAlive > 0 {
		go t.keepAliveLoop(c)
	}
	//block until closed
	go t.handleSSHRequests(reqs)
	go t.handleSSHChannels(chans)
	t.Debugf("SSH connected")
	t.updateLastActivity()
	err := c.Wait()
	t.Debugf("SSH disconnected")
	//mark inactive and block
	t.activatingConn.Add(1)
	t.activeConnMut.Lock()
	t.activeConn = nil
	t.activeConnMut.Unlock()
	return err
}

//getSSH blocks while connecting
func (t *Tunnel) getSSH(ctx context.Context) ssh.Conn {
	//cancelled already?
	if isDone(ctx) {
		return nil
	}
	t.activeConnMut.RLock()
	c := t.activeConn
	t.activeConnMut.RUnlock()
	//connected already?
	if c != nil {
		return c
	}
	//connecting...
	select {
	case <-ctx.Done(): //cancelled
		return nil
	case <-time.After(settings.EnvDuration("SSH_WAIT", 35*time.Second)):
		return nil //a bit longer than ssh timeout
	case <-t.activatingConnWait():
		t.activeConnMut.RLock()
		c := t.activeConn
		t.activeConnMut.RUnlock()
		return c
	}
}

//IsInbound returns true if this is a client tunnel (inbound)
func (t *Tunnel) IsInbound() bool {
	return t.Inbound
}

func (t *Tunnel) activatingConnWait() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		t.activatingConn.Wait()
		close(ch)
	}()
	return ch
}

//BindRemotes converts the given remotes into proxies, and blocks
//until the caller cancels the context or there is a proxy error.
func (t *Tunnel) BindRemotes(ctx context.Context, remotes []*settings.Remote) error {
	if len(remotes) == 0 {
		return errors.New("no remotes")
	}
	if !t.Inbound {
		return errors.New("inbound connections blocked")
	}
	proxies := make([]*Proxy, len(remotes))
	for i, remote := range remotes {
		p, err := NewProxy(t.Logger, t, t.proxyCount, remote)
		if err != nil {
			return err
		}
		proxies[i] = p
		t.proxyCount++
	}
	//TODO: handle tunnel close
	eg, ctx := errgroup.WithContext(ctx)
	for _, proxy := range proxies {
		p := proxy
		eg.Go(func() error {
			return p.Run(ctx)
		})
	}
	t.Debugf("Bound proxies")
	err := eg.Wait()
	t.Debugf("Unbound proxies")
	return err
}

func (t *Tunnel) keepAliveLoop(sshConn ssh.Conn) {
	//ping forever with randomized intervals
	//Default: ±30% jitter to make traffic patterns less predictable
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	jitterPercent := 0.30 // 30% jitter (highest level)
	
	for {
		// Calculate jitter: ±30% of KeepAlive duration
		baseDuration := float64(t.Config.KeepAlive)
		jitterRange := baseDuration * jitterPercent
		// Random value between -jitterRange and +jitterRange
		jitter := (rng.Float64()*2 - 1) * jitterRange
		actualDuration := time.Duration(baseDuration + jitter)
		
		// Ensure duration is at least 10% of original to avoid too short intervals
		minDuration := time.Duration(baseDuration * 0.1)
		if actualDuration < minDuration {
			actualDuration = minDuration
		}
		
		time.Sleep(actualDuration)
		_, b, err := sshConn.SendRequest("ping", true, nil)
		if err != nil {
			break
		}
		if len(b) > 0 && !bytes.Equal(b, []byte("pong")) {
			t.Debugf("strange ping response")
			break
		}
		t.updateLastActivity()
	}
	//close ssh connection on abnormal ping
	sshConn.Close()
}

// connectionPoolHealthCheck periodically checks connection pool health
func (t *Tunnel) connectionPoolHealthCheck() {
	if t.healthCheck == nil {
		return
	}
	
	for range t.healthCheck.C {
		t.connPoolMut.Lock()
		// Clean up dead connections
		validConns := make([]ssh.Conn, 0)
		for _, conn := range t.connPool {
			if conn != nil && !isConnDead(conn) {
				validConns = append(validConns, conn)
			} else {
				t.Debugf("Removed dead connection from pool")
			}
		}
		t.connPool = validConns
		t.connPoolMut.Unlock()
		
		// Log pool status
		if t.Logger.Debug {
			t.Debugf("Connection pool size: %d", len(t.connPool))
		}
	}
}

// updateLastActivity updates the last activity timestamp
func (t *Tunnel) updateLastActivity() {
	t.connPoolMut.Lock()
	t.lastActivity = time.Now()
	t.connPoolMut.Unlock()
}

// isConnDead checks if an SSH connection is dead
func isConnDead(conn ssh.Conn) bool {
	// Try to send a small ping to check connection health
	_, _, err := conn.SendRequest("keepalive@openssh.com", true, nil)
	return err != nil
}
