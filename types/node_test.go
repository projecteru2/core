package types

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
)

func TestNodeInfo(t *testing.T) {
	mockEngine := &enginemocks.API{}
	r := &enginetypes.Info{ID: "test"}
	mockEngine.On("Info", mock.Anything).Return(r, ErrNoOps).Once()

	node := &Node{}
	ctx := t.Context()

	node.Engine = mockEngine
	err := node.Info(ctx)
	assert.Error(t, err)
	mockEngine.On("Info", mock.Anything).Return(r, nil)
	err = node.Info(ctx)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(node.NodeInfo, "test"))

	node.Bypass = true
	assert.True(t, node.IsDown())
}

func TestNodeFilterNarrow(t *testing.T) {
	configured := NodeFilter{
		Podname:  "buildpod",
		Includes: []string{"n1", "n2"},
		Excludes: []string{"n3"},
		Labels:   map[string]string{"eru.build": "1"},
		All:      true,
	}

	tests := []struct {
		name      string
		requested *NodeFilter
		want      *NodeFilter
	}{
		{"no request keeps the configured filter", nil, &configured},
		{
			"names intersect",
			&NodeFilter{Includes: []string{"n2", "n9"}},
			&NodeFilter{Podname: "buildpod", Includes: []string{"n2"}, Excludes: []string{"n3"}, Labels: configured.Labels, All: true},
		},
		{
			"exclusions accumulate",
			&NodeFilter{Excludes: []string{"n1"}},
			&NodeFilter{Podname: "buildpod", Includes: []string{"n1", "n2"}, Excludes: []string{"n3", "n1"}, Labels: configured.Labels, All: true},
		},
		{
			"a new label is added",
			&NodeFilter{Labels: map[string]string{"zone": "a"}},
			&NodeFilter{
				Podname: "buildpod", Includes: []string{"n1", "n2"}, Excludes: []string{"n3"},
				Labels: map[string]string{"eru.build": "1", "zone": "a"}, All: true,
			},
		},
		{
			"all stays with the configured value",
			&NodeFilter{All: false},
			&configured,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configured.Narrow(tt.requested)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNodeFilterNarrowRejectsWidening(t *testing.T) {
	configured := NodeFilter{Podname: "buildpod", Includes: []string{"n1"}, Labels: map[string]string{"eru.build": "1"}}

	tests := []struct {
		name      string
		requested *NodeFilter
	}{
		{"another pod", &NodeFilter{Podname: "elsewhere"}},
		{"another value for a configured label", &NodeFilter{Labels: map[string]string{"eru.build": "0"}}},
		{"nodes the configured list never allowed", &NodeFilter{Includes: []string{"n9"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := configured.Narrow(tt.requested)
			assert.ErrorIs(t, err, ErrInvaildNodeFilter)
		})
	}
}

func TestNodeFilterNarrowLeavesTheConfiguredFilterAlone(t *testing.T) {
	configured := NodeFilter{Includes: []string{"n1", "n2"}, Labels: map[string]string{"eru.build": "1"}}

	_, err := configured.Narrow(&NodeFilter{Includes: []string{"n1"}, Labels: map[string]string{"zone": "a"}})
	assert.NoError(t, err)
	assert.Equal(t, []string{"n1", "n2"}, configured.Includes)
	assert.Equal(t, map[string]string{"eru.build": "1"}, configured.Labels)
}
