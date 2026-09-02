package calcium

import (
	"testing"

	"github.com/stretchr/testify/mock"

	resourcemocks "github.com/projecteru2/core/resource/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	"github.com/projecteru2/core/types"
)

func TestSendNodeMetricsSurvivesUninitializedMetrics(t *testing.T) {
	c := NewTestCluster()
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodesMetrics", mock.Anything, mock.Anything).Return([]*plugintypes.Metrics{{Name: "cpu_used"}}, nil).Once()

	c.doSendNodeMetrics(t.Context(), &types.Node{NodeMeta: types.NodeMeta{Name: "n1"}})
	rmgr.AssertExpectations(t)
}
