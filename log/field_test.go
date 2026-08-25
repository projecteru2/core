package log

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

var sink *Fields

func TestWithFieldLeavesItsParentUntouched(t *testing.T) {
	base := WithFunc("calcium.RemoveWorkload")
	first := base.WithField("id", "a")
	second := base.WithField("id", "b")

	tests := []struct {
		name   string
		fields *Fields
		want   string
	}{
		{"parent", base, `{"level":"debug","func":"calcium.RemoveWorkload","message":"m"}`},
		{"first child", first, `{"level":"debug","func":"calcium.RemoveWorkload","id":"a","message":"m"}`},
		{"second child", second, `{"level":"debug","func":"calcium.RemoveWorkload","id":"b","message":"m"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, capture(t, func() { tt.fields.Debug(t.Context(), "m") }))
		})
	}
}

func TestWithFieldIsSafeForConcurrentUse(t *testing.T) {
	base := WithFunc("calcium.RemoveWorkload")
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			assert.NotNil(t, base.WithField("id", i))
		})
	}
	wg.Wait()
}

func BenchmarkWithFunc(b *testing.B) {
	for b.Loop() {
		sink = WithFunc("calcium.doDeployWorkloadsOnNode")
	}
}

func BenchmarkWithFuncChain(b *testing.B) {
	for b.Loop() {
		sink = WithFunc("calcium.doDeployWorkloadsOnNode").
			WithField("node", "node-1").
			WithField("ident", "ident-1").
			WithField("deploy", 2).
			WithField("seq", 0)
	}
}
