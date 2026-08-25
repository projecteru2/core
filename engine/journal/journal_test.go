package journal

import (
	"slices"
	"testing"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestFlags(t *testing.T) {
	tests := []struct {
		name string
		opts *enginetypes.VirtualizationLogStreamOptions
		want []string
	}{
		{"defaults", &enginetypes.VirtualizationLogStreamOptions{ID: "w1"}, []string{"-o", "cat"}},
		{
			"a tail",
			&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Tail: "10"},
			[]string{"-o", "cat", "-n", "10"},
		},
		{
			"unix seconds pass through as an absolute stamp",
			&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Since: "1700000000"},
			[]string{"-o", "cat", "--since", "@1700000000"},
		},
		{
			"an RFC3339 window becomes absolute stamps",
			&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Since: "2023-11-14T22:13:20Z", Until: "2023-11-14T23:13:20Z"},
			[]string{"-o", "cat", "--since", "@1700000000", "--until", "@1700003600"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Flags(tt.opts)
			if err != nil {
				t.Fatalf("flags: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlagsRejectAnUnreadableTimestamp(t *testing.T) {
	_, err := Flags(&enginetypes.VirtualizationLogStreamOptions{ID: "w1", Since: "yesterday"})
	if !errors.Is(err, coretypes.ErrInvaildWorkloadOps) {
		t.Errorf("got %v, want ErrInvaildWorkloadOps", err)
	}
}
