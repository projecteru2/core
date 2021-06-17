package embedded

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmbededCluster(t *testing.T) {
	embededETCD := NewCluster(t, "/test")
	cliv3 := embededETCD.RandClient()
	_, err := cliv3.MemberList(context.Background())
	assert.NoError(t, err)
}
