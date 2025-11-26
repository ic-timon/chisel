package cnet

import (
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/jpillora/chisel/share/cio"
	"github.com/jpillora/sizestr"
)

//NewMeter to measure readers/writers
func NewMeter(l *cio.Logger) *Meter {
	return &Meter{l: l}
}

//Meter can be inserted in the path or
//of a reader or writer to measure the
//throughput
type Meter struct {
	//meter state - use atomic operations for thread safety
	sent, recv atomic.Int64
	//print state
	l            *cio.Logger
	printing     atomic.Bool
	last         atomic.Int64
	lsent, lrecv atomic.Int64
}

func (m *Meter) print() {
	//move out of the read/write path asap
	if m.printing.CompareAndSwap(false, true) {
		go m.goprint()
	}
}

func (m *Meter) goprint() {
	time.Sleep(time.Second)
	//snapshot using atomic operations
	s := m.sent.Load()
	r := m.recv.Load()
	//compute speed
	curr := time.Now().UnixNano()
	last := m.last.Load()
	dt := time.Duration(curr-last) * time.Nanosecond
	ls := m.lsent.Load()
	lr := m.lrecv.Load()
	//DEBUG
	// m.l.Infof("%s = %d(%d-%d), %d(%d-%d)", dt, s-ls, s, ls, r-lr, r, lr)
	//scale to per second V=D/T
	sps := int64(float64(s-ls) / float64(dt) * float64(time.Second))
	rps := int64(float64(r-lr) / float64(dt) * float64(time.Second))
	if last > 0 && (sps != 0 || rps != 0) {
		m.l.Debugf("write %s/s read %s/s", sizestr.ToString(sps), sizestr.ToString(rps))
	}
	//record last printed
	m.lsent.Store(s)
	m.lrecv.Store(r)
	//done
	m.last.Store(curr)
	m.printing.Store(false)
}

//TeeReader inserts Meter into the read path
//if the linked logger is in debug mode,
//otherwise this is a no-op
func (m *Meter) TeeReader(r io.Reader) io.Reader {
	if m.l.IsDebug() {
		return &meterReader{m, r}
	}
	return r
}

type meterReader struct {
	*Meter
	inner io.Reader
}

func (m *meterReader) Read(p []byte) (n int, err error) {
	n, err = m.inner.Read(p)
	m.recv.Add(int64(n))
	m.Meter.print()
	return
}

//TeeWriter inserts Meter into the write path
//if the linked logger is in debug mode,
//otherwise this is a no-op
func (m *Meter) TeeWriter(w io.Writer) io.Writer {
	if m.l.IsDebug() {
		return &meterWriter{m, w}
	}
	return w
}

type meterWriter struct {
	*Meter
	inner io.Writer
}

func (m *meterWriter) Write(p []byte) (n int, err error) {
	n, err = m.inner.Write(p)
	m.sent.Add(int64(n))
	m.Meter.print()
	return
}

//MeterConn inserts Meter into the connection path
//if the linked logger is in debug mode,
//otherwise this is a no-op
func MeterConn(l *cio.Logger, conn net.Conn) net.Conn {
	m := NewMeter(l)
	return &meterConn{
		mread:  m.TeeReader(conn),
		mwrite: m.TeeWriter(conn),
		Conn:   conn,
	}
}

type meterConn struct {
	mread  io.Reader
	mwrite io.Writer
	net.Conn
}

func (m *meterConn) Read(p []byte) (n int, err error) {
	return m.mread.Read(p)
}

func (m *meterConn) Write(p []byte) (n int, err error) {
	return m.mwrite.Write(p)
}

//MeterRWC inserts Meter into the RWC path
//if the linked logger is in debug mode,
//otherwise this is a no-op
func MeterRWC(l *cio.Logger, rwc io.ReadWriteCloser) io.ReadWriteCloser {
	m := NewMeter(l)
	return &struct {
		io.Reader
		io.Writer
		io.Closer
	}{
		Reader: m.TeeReader(rwc),
		Writer: m.TeeWriter(rwc),
		Closer: rwc,
	}
}
