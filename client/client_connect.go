package chclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jpillora/backoff"
	chshare "github.com/jpillora/chisel/share"
	"github.com/jpillora/chisel/share/cnet"
	"github.com/jpillora/chisel/share/cos"
	"github.com/jpillora/chisel/share/settings"
	"golang.org/x/crypto/ssh"
)

func (c *Client) connectionLoop(ctx context.Context) error {
	//connection loop!
	b := &backoff.Backoff{Max: c.config.MaxRetryInterval}
	var lastSuccess time.Time
	var consecutiveFailures int
	var adaptiveBackoff time.Duration
	
	for {
		connected, err := c.connectionOnce(ctx)
		//reset backoff after successful connections
		if connected {
			b.Reset()
			lastSuccess = time.Now()
			consecutiveFailures = 0
			adaptiveBackoff = 0
		} else {
			consecutiveFailures++
		}
		
		//connection error
		attempt := int(b.Attempt())
		maxAttempt := c.config.MaxRetryCount
		//dont print closed-connection errors
		if err != nil && strings.HasSuffix(err.Error(), "use of closed network connection") {
			err = io.EOF
		}
		//show error message and attempt counts (excluding disconnects)
		if err != nil && err != io.EOF {
			msg := fmt.Sprintf("Connection error: %s", err)
			if attempt > 0 {
				maxAttemptVal := fmt.Sprint(maxAttempt)
				if maxAttempt < 0 {
					maxAttemptVal = "unlimited"
				}
				msg += fmt.Sprintf(" (Attempt: %d/%s)", attempt, maxAttemptVal)
			}
			c.Infof(msg)
		}
		//give up?
		if maxAttempt >= 0 && attempt >= maxAttempt {
			c.Infof("Give up")
			break
		}
		
		// Adaptive backoff calculation
		baseDuration := b.Duration()
		
		// Increase backoff based on consecutive failures
		if consecutiveFailures > 3 {
			adaptiveBackoff = baseDuration * time.Duration(consecutiveFailures/3)
			if adaptiveBackoff > 10*time.Minute {
				adaptiveBackoff = 10 * time.Minute
			}
		}
		
		// Network quality assessment
		if !lastSuccess.IsZero() && time.Since(lastSuccess) > 5*time.Minute {
			// If no successful connection for 5 minutes, reduce backoff to probe network
			adaptiveBackoff = 5 * time.Second
		}
		
		finalBackoff := baseDuration + adaptiveBackoff
		if finalBackoff > 10*time.Minute {
			finalBackoff = 10 * time.Minute
		}
		
		c.Infof("Retrying in %s (adaptive: %s)...", finalBackoff, adaptiveBackoff)
		select {
		case <-cos.AfterSignal(finalBackoff):
			continue //retry now
		case <-ctx.Done():
			c.Infof("Cancelled")
			return nil
		}
	}
	c.Close()
	return nil
}

// connectionOnce connects to the chisel server and blocks
func (c *Client) connectionOnce(ctx context.Context) (connected bool, err error) {
	//already closed?
	select {
	case <-ctx.Done():
		return false, errors.New("Cancelled")
	default:
		//still open
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	//prepare dialer
	// Use masked protocol to hide chisel identity
	// The actual protocol verification happens via SSH custom request after handshake
	d := websocket.Dialer{
		HandshakeTimeout: settings.EnvDuration("WS_TIMEOUT", 45*time.Second),
		Subprotocols:     []string{chshare.MaskedWebSocketProtocol},
		TLSClientConfig:  c.tlsConfig,
		ReadBufferSize:   settings.EnvInt("WS_BUFF_SIZE", 0),
		WriteBufferSize:  settings.EnvInt("WS_BUFF_SIZE", 0),
		NetDialContext:   c.config.DialContext,
	}
	//optional proxy
	if p := c.proxyURL; p != nil {
		if err := c.setProxy(p, &d); err != nil {
			return false, err
		}
	}
	
	// Connection timeout with adaptive strategy
	connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
	defer connectCancel()
	
	// Remove Connection header to avoid duplicate with WebSocket library
	headers := c.config.Headers.Clone()
	if headers != nil {
		headers.Del("Connection")
	}
	
	wsConn, _, err := d.DialContext(connectCtx, c.server, headers)
	if err != nil {
		// Check for specific error types to adjust strategy
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
			c.Debugf("Connection timeout, network may be unstable")
		}
		return false, err
	}
	conn := cnet.NewWebSocketConn(wsConn)
	// perform SSH handshake on net.Conn
	c.Debugf("Handshaking...")
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, "", c.sshConfig)
	if err != nil {
		e := err.Error()
		if strings.Contains(e, "unable to authenticate") {
			c.Infof("Authentication failed")
			c.Debugf(e)
		} else {
			c.Infof(e)
		}
		return false, err
	}
	defer sshConn.Close()
	// chisel client handshake (reverse of server handshake)
	// send configuration
	c.Debugf("Sending config")
	t0 := time.Now()
	_, configerr, err := sshConn.SendRequest(
		"config",
		true,
		settings.EncodeConfig(c.computed),
	)
	if err != nil {
		c.Infof("Config verification failed")
		return false, err
	}
	if len(configerr) > 0 {
		return false, errors.New(string(configerr))
	}
	c.Infof("Connected (Latency %s)", time.Since(t0))
	//connected, handover ssh connection for tunnel to use, and block
	err = c.tunnel.BindSSH(ctx, sshConn, reqs, chans)
	c.Infof("Disconnected")
	connected = time.Since(t0) > 5*time.Second
	return connected, err
}
