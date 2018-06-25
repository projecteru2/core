package calcium

import (
	"context"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestCopy(t *testing.T) {
	initMockConfig()

	opts := &types.CopyOptions{
		Targets: map[string][]string{
			"f1f9da344e8f8f90f73899ddad02da6cdf2218bbe52413af2bcfef4fba2d22de": []string{
				"/usr/bin/sh",
				"/etc",
			},
		},
	}

	_, err = mockc.Copy(context.Background(), opts)
	assert.NoError(t, err)
}
