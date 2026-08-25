package utils

import (
	"math/rand/v2"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	statsdMaxPacketSize = 1440
	statsdFlushPeriod   = 100 * time.Millisecond
)

// Statsd batches metrics into UDP datagrams and drops send failures.
type Statsd struct {
	conn net.Conn
	done chan struct{}

	mu     sync.Mutex
	buf    []byte
	closed bool
}

// NewStatsd dials the statsd server at addr and flushes buffered metrics every 100ms.
func NewStatsd(addr string) (*Statsd, error) {
	return newStatsd(addr, statsdFlushPeriod)
}

func newStatsd(addr string, flushPeriod time.Duration) (*Statsd, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	s := &Statsd{
		conn: conn,
		done: make(chan struct{}),
		buf:  make([]byte, 0, statsdMaxPacketSize*2),
	}
	go s.flushEvery(flushPeriod)
	return s, nil
}

// Gauge sets name to value, prefixed by a zero when value is negative because statsd reads a leading minus as a delta.
func (s *Statsd) Gauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filled := len(s.buf)
	if value < 0 {
		s.appendName(name)
		s.buf = append(s.buf, '0', '|', 'g', '\n')
	}
	s.appendName(name)
	s.buf = strconv.AppendFloat(s.buf, value, 'f', -1, 64)
	s.buf = append(s.buf, '|', 'g', '\n')
	s.flushIfFull(filled)
}

// Count adds n to name, sampling at rate.
func (s *Statsd) Count(name string, n int, rate float32) {
	if rate != 1 && rand.Float32() > rate { //nolint:gosec
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	filled := len(s.buf)
	s.appendName(name)
	s.buf = strconv.AppendInt(s.buf, int64(n), 10)
	s.buf = append(s.buf, '|', 'c')
	if rate != 1 {
		s.buf = append(s.buf, '|', '@')
		s.buf = strconv.AppendFloat(s.buf, float64(rate), 'f', -1, 32)
	}
	s.buf = append(s.buf, '\n')
	s.flushIfFull(filled)
}

// Close flushes the buffer, stops the flush ticker and releases the connection.
func (s *Statsd) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	s.flushAll()
	return s.conn.Close()
}

func (s *Statsd) flushEvery(period time.Duration) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			s.flushAll()
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}

func (s *Statsd) appendName(name string) {
	s.buf = append(s.buf, name...)
	s.buf = append(s.buf, ':')
}

func (s *Statsd) flushIfFull(filled int) {
	if len(s.buf) <= statsdMaxPacketSize {
		return
	}
	if filled == 0 {
		s.flushAll()
		return
	}
	s.flushFirst(filled)
}

func (s *Statsd) flushAll() {
	s.flushFirst(len(s.buf))
}

func (s *Statsd) flushFirst(n int) {
	if n == 0 {
		return
	}
	_, _ = s.conn.Write(s.buf[:n-1])
	copy(s.buf, s.buf[n:])
	s.buf = s.buf[:len(s.buf)-n]
}
