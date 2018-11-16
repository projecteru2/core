package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageBucket(t *testing.T) {
	ib := newImageBucket()
	podname := "p1"
	img1 := "img1"
	img2 := "img2"
	ib.Add(podname, img1)
	ib.Add(podname, img2)

	r := ib.Dump()
	assert.Len(t, r, 1)
	assert.Len(t, r[podname], 2)
}
