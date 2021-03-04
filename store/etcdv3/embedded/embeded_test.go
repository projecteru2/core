package embedded

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmbededCluster(t *testing.T) {
	embededETCD := NewCluster()
	cliv3 := embededETCD.Cluster.RandClient()
	_, err := cliv3.MemberList(context.Background())
	assert.NoError(t, err)
	embededETCD.TerminateCluster()
}
