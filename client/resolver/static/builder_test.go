package static

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

func TestBuildStripsLeadingSlashFromEndpoint(t *testing.T) {
	u, err := url.Parse("static://_/1.2.3.4:5001,5.6.7.8:5001")
	assert.NoError(t, err)

	cc := &stubClientConn{}
	b := &staticResolverBuilder{}
	r, err := b.Build(resolver.Target{URL: *u}, cc, resolver.BuildOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, r)

	assert.Equal(t, []resolver.Address{
		{Addr: "1.2.3.4:5001"},
		{Addr: "5.6.7.8:5001"},
	}, cc.state.Addresses)
}

type stubClientConn struct {
	state resolver.State
}

func (c *stubClientConn) UpdateState(s resolver.State) error {
	c.state = s
	return nil
}

func (c *stubClientConn) ReportError(error) {}

func (c *stubClientConn) NewAddress([]resolver.Address) {}

func (c *stubClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult {
	return nil
}
