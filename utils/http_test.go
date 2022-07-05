package utils

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHTTPClient(t *testing.T) {
	assert.NotNil(t, GetHTTPClient())
}

func TestGetUnixSockClient(t *testing.T) {
	assert.NotNil(t, GetUnixSockClient())
}

func TestGetHTTPSClient(t *testing.T) {
	ctx := context.Background()
	client, err := GetHTTPSClient(ctx, "", "abc", "", "", "")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	client, err = GetHTTPSClient(ctx, os.TempDir(), "abc", "1", "2", "3")
	assert.Error(t, err)
	assert.Nil(t, client)
}
