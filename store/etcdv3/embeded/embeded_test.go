package embeded

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmbededCluster(t *testing.T) {
	cliv3 := NewCluster()
	_, err := cliv3.MemberList(context.Background())
	assert.NoError(t, err)
	TerminateCluster()
}
