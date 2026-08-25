package virt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCombineUserImage(t *testing.T) {
	user := "user"
	image := "image"

	combine := combineUserImage(user, image)
	require.Equal(t, "user@image", combine)
	u, i, err := splitUserImage(combine)
	require.NoError(t, err)
	require.Equal(t, user, u)
	require.Equal(t, image, i)

	combine = combineUserImage("", image)
	require.Equal(t, image, combine)
	u, i, err = splitUserImage(combine)
	require.NoError(t, err)
	require.Equal(t, "", u)
	require.Equal(t, image, i)

	combine = combineUserImage(user, "")
	require.Equal(t, "", combine)
	u, i, err = splitUserImage(combine)
	require.Error(t, err)

	combine = combineUserImage("", "")
	require.Equal(t, "", combine)

	u, i, err = splitUserImage("@")
	require.Error(t, err)

	u, i, err = splitUserImage("hello@")
	require.Error(t, err)
}
