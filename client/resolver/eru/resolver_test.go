package eru

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

func TestCloseRightAfterNew(t *testing.T) {
	r := New(&stubClientConn{}, "1.2.3.4:5001", "")
	assert.NotPanics(t, r.Close)
}

type stubClientConn struct{}

func (c *stubClientConn) UpdateState(resolver.State) error { return nil }

func (c *stubClientConn) ReportError(error) {}

func (c *stubClientConn) NewAddress([]resolver.Address) {}

func (c *stubClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult { return nil }
