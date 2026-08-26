package calcium

import (
	"testing"

	"github.com/stretchr/testify/mock"

	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestSetNodeUpdatesTLSOnlyWhenRequested(t *testing.T) {
	tests := []struct {
		name      string
		updateTLS bool
		wantCA    string
		wantCert  string
		wantKey   string
	}{
		{name: "keeps tls", wantCA: "old-ca", wantCert: "old-cert", wantKey: "old-key"},
		{name: "updates tls", updateTLS: true, wantCA: "new-ca", wantCert: "new-cert", wantKey: "new-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			c := NewTestCluster()
			store := c.store.(*storemocks.Store)
			lock := &lockmocks.DistributedLock{}
			lock.On("Lock", mock.Anything).Return(ctx, nil)
			lock.On("Unlock", mock.Anything).Return(nil)
			store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
			store.On("GetNode", mock.Anything, mock.Anything).Return(&types.Node{
				NodeMeta: types.NodeMeta{Name: "node1", Ca: "old-ca", Cert: "old-cert", Key: "old-key"},
			}, nil)
			store.On("UpdateNodes", mock.Anything, mock.Anything).Return(types.ErrMockError)
			c.rmgr.(*resourcemocks.Manager).On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)

			node, err := c.SetNode(ctx, &types.SetNodeOptions{
				Nodename:  "node1",
				UpdateTLS: tt.updateTLS,
				Ca:        "new-ca",
				Cert:      "new-cert",
				Key:       "new-key",
			})
			if err == nil {
				t.Fatal("got nil, want the store failure")
			}
			if node.Ca != tt.wantCA || node.Cert != tt.wantCert || node.Key != tt.wantKey {
				t.Errorf("got ca=%q cert=%q key=%q, want ca=%q cert=%q key=%q", node.Ca, node.Cert, node.Key, tt.wantCA, tt.wantCert, tt.wantKey)
			}
		})
	}
}
