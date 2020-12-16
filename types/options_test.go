package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTriOption(t *testing.T) {
	assert.False(t, ParseTriOption(TriFalse, true))
	assert.True(t, ParseTriOption(TriTrue, false))
	assert.False(t, ParseTriOption(TriKeep, false))
	assert.True(t, ParseTriOption(TriKeep, true))
}

func TestSetNodeOptions(t *testing.T) {
	o := &SetNodeOptions{
		DeltaVolume:  VolumeMap{"/data": 1, "/data2": 2},
		DeltaStorage: -10,
	}
	assert.Equal(t, ErrEmptyNodeName, o.Validate())

	o.Nodename = "nodename"
	assert.NoError(t, o.Validate())

	o.Normalize(nil)
	assert.EqualValues(t, -7, o.DeltaStorage)

	node := &Node{
		NodeMeta: NodeMeta{
			InitVolume: VolumeMap{"/data0": 100, "/data1": 3},
		},
	}
	o = &SetNodeOptions{
		DeltaVolume:  VolumeMap{"/data0": 0, "/data1": 10},
		DeltaStorage: 10,
	}
	o.Normalize(node)
	assert.EqualValues(t, 10-100+10, o.DeltaStorage)
}

func TestDeployOptions(t *testing.T) {
	assert := assert.New(t)

	o := &DeployOptions{Entrypoint: &Entrypoint{}}
	assert.Equal(ErrEmptyAppName, o.Validate())

	o.Name = "testname"
	assert.Equal(ErrEmptyPodName, o.Validate())

	o.Podname = "podname"
	assert.Equal(ErrEmptyImage, o.Validate())

	o.Image = "image"
	assert.Equal(ErrEmptyCount, o.Validate())

	o.Count = 1
	assert.Equal(ErrEmptyEntrypointName, o.Validate())

	o.Entrypoint.Name = "bad_entry_point"
	assert.Equal(ErrUnderlineInEntrypointName, o.Validate())

	o.Entrypoint.Name = "good-entry-point"
	assert.NoError(o.Validate())
}

func TestCopyOptions(t *testing.T) {
	assert := assert.New(t)

	o := &CopyOptions{}
	assert.Equal(ErrNoFilesToCopy, o.Validate())

	o.Targets = map[string][]string{
		"workload_id": {
			"filepath1",
			"filepath2",
		},
	}
	assert.NoError(o.Validate())
}

func TestSendOptions(t *testing.T) {
	assert := assert.New(t)

	o := &SendOptions{}
	assert.Equal(ErrNoWorkloadIDs, o.Validate())

	o.IDs = []string{"workload_id1", "workload_id2"}
	assert.Equal(ErrNoFilesToSend, o.Validate())

	o.Data = map[string][]byte{
		"filepath1": []byte("filecontent1"),
		"filepath2": []byte("filecontent2"),
	}
	assert.NoError(o.Validate())
}

func TestReplaceOptions(t *testing.T) {
	assert := assert.New(t)

	o := &ReplaceOptions{DeployOptions: DeployOptions{Entrypoint: &Entrypoint{}}}
	assert.Equal(ErrEmptyAppName, o.Validate())

	o.DeployOptions.Name = "testname"
	assert.Equal(ErrEmptyEntrypointName, o.Validate())

	o.DeployOptions.Entrypoint.Name = "bad_entry_point"
	assert.Equal(ErrUnderlineInEntrypointName, o.Validate())

	o.DeployOptions.Entrypoint.Name = "good-entry-point"
	assert.NoError(o.Validate())

	assert.Equal(o.Count, 0)
	o.Normalize()
	assert.Equal(o.Count, 1)
	o.Count = 2
	o.Normalize()
	assert.Equal(o.Count, 2)
}

func TestValidatingAddNodeOptions(t *testing.T) {
	assert := assert.New(t)

	o := &AddNodeOptions{}
	assert.Equal(ErrEmptyNodeName, o.Validate())

	o.Nodename = "nodename"
	assert.Equal(ErrEmptyPodName, o.Validate())

	o.Podname = "podname"
	assert.Equal(ErrEmptyNodeEndpoint, o.Validate())

	o.Endpoint = "tcp://endpoint:2376"
	assert.NoError(o.Validate())
}

func TestImageOptions(t *testing.T) {
	assert := assert.New(t)

	o := &ImageOptions{Step: -1}
	assert.Equal(ErrEmptyPodName, o.Validate())

	o.Podname = "podname"
	assert.NoError(o.Validate())

	assert.Equal(o.Step, -1)
	o.Normalize()
	assert.Equal(o.Step, 1)

	o.Step = 3
	o.Normalize()
	assert.Equal(o.Step, 3)
}
