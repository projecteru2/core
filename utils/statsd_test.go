package utils

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestStatsdLineProtocol(t *testing.T) {
	tests := []struct {
		name string
		send func(*Statsd)
		want string
	}{
		{"whole gauge", func(s *Statsd) { s.Gauge("core.gauge", 1) }, "core.gauge:1|g"},
		{"fractional gauge", func(s *Statsd) { s.Gauge("core.gauge", 1.5) }, "core.gauge:1.5|g"},
		{"negative gauge", func(s *Statsd) { s.Gauge("core.gauge", -2) }, "core.gauge:0|g\ncore.gauge:-2|g"},
		{"unsampled count", func(s *Statsd) { s.Count("core.count", 3, 1) }, "core.count:3|c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := newStatsdPair(t, time.Hour)
			tt.send(client)
			if err := client.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
			if got := readPacket(t, server); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatsdBatchesMetricsIntoOneDatagram(t *testing.T) {
	server, client := newStatsdPair(t, time.Hour)
	client.Gauge("core.one", 1)
	client.Gauge("core.two", 2)
	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got, want := readPacket(t, server), "core.one:1|g\ncore.two:2|g"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStatsdFlushesWhenTheBufferFills(t *testing.T) {
	const metricSize = 105
	server, client := newStatsdPair(t, time.Hour)
	name := strings.Repeat("a", 100)
	for range statsdMaxPacketSize/metricSize + 1 {
		client.Gauge(name, 1)
	}

	packet := readPacket(t, server)
	if len(packet) > statsdMaxPacketSize {
		t.Errorf("datagram is %d bytes, want at most %d", len(packet), statsdMaxPacketSize)
	}
	if got, want := strings.Count(packet, "\n")+1, statsdMaxPacketSize/metricSize; got != want {
		t.Errorf("got %d metrics in the datagram, want %d", got, want)
	}
}

func TestStatsdFlushesOnItsTicker(t *testing.T) {
	server, client := newStatsdPair(t, 10*time.Millisecond)
	client.Gauge("core.gauge", 1)

	if got, want := readPacket(t, server), "core.gauge:1|g"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStatsdCountCarriesSamplingRate(t *testing.T) {
	server, client := newStatsdPair(t, time.Hour)
	for range 64 {
		client.Count("core.count", 3, 0.5)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if packet, want := readPacket(t, server), "core.count:3|c|@0.5"; !strings.Contains(packet, want) {
		t.Errorf("got %q, want it to contain %q", packet, want)
	}
}

func TestNewStatsdRejectsMalformedAddress(t *testing.T) {
	if _, err := NewStatsd("127.0.0.1"); err == nil {
		t.Error("got nil, want a dial error")
	}
}

func BenchmarkStatsdGauge(b *testing.B) {
	_, client := newStatsdPair(b, time.Hour)
	for b.Loop() {
		client.Gauge("core.node.node-1.memory", 1<<30)
	}
}

func newStatsdPair(tb testing.TB, flushPeriod time.Duration) (net.PacketConn, *Statsd) {
	tb.Helper()
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("listen: %v", err)
	}
	tb.Cleanup(func() { server.Close() })

	client, err := newStatsd(server.LocalAddr().String(), flushPeriod)
	if err != nil {
		tb.Fatalf("dial: %v", err)
	}
	tb.Cleanup(func() { client.Close() })
	return server, client
}

func readPacket(tb testing.TB, server net.PacketConn) string {
	tb.Helper()
	if err := server.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		tb.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 4096)
	n, _, err := server.ReadFrom(buf)
	if err != nil {
		tb.Fatalf("read: %v", err)
	}
	return string(buf[:n])
}
