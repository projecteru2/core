package types

import (
	"testing"

	"github.com/pkg/errors"
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
	assert.Equal(t, ErrEmptyNodeName, errors.Unwrap(o.Validate()))

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
	assert.Equal(ErrEmptyAppName, errors.Unwrap(o.Validate()))

	o.Name = "testname"
	assert.Equal(ErrEmptyPodName, errors.Unwrap(o.Validate()))

	o.Podname = "podname"
	assert.Equal(ErrEmptyImage, errors.Unwrap(o.Validate()))

	o.Image = "image"
	assert.Equal(ErrEmptyCount, errors.Unwrap(o.Validate()))

	o.Count = 1
	assert.Equal(ErrEmptyEntrypointName, errors.Unwrap(o.Validate()))

	o.Entrypoint.Name = "bad_entry_point"
	assert.Equal(ErrUnderlineInEntrypointName, errors.Unwrap(o.Validate()))

	o.Entrypoint.Name = "good-entry-point"
	assert.NoError(o.Validate())
}

func TestCopyOptions(t *testing.T) {
	assert := assert.New(t)

	o := &CopyOptions{}
	assert.Equal(ErrNoFilesToCopy, errors.Unwrap(o.Validate()))

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
	assert.Equal(ErrNoWorkloadIDs, errors.Unwrap(o.Validate()))

	o.IDs = []string{"workload_id1", "workload_id2"}
	assert.Equal(ErrNoFilesToSend, errors.Unwrap(o.Validate()))

	o.Files = []LinuxFile{
		{
			Filename: "filepath1",
			Content:  []byte("filecontent1"),
		},
		{
			Filename: "filepath2",
			Content:  []byte("filecontent2"),
		},
	}
	assert.NoError(o.Validate())
}

func TestReplaceOptions(t *testing.T) {
	assert := assert.New(t)

	o := &ReplaceOptions{DeployOptions: DeployOptions{Entrypoint: &Entrypoint{}}}
	assert.Equal(ErrEmptyAppName, errors.Unwrap(o.Validate()))

	o.DeployOptions.Name = "testname"
	assert.Equal(ErrEmptyEntrypointName, errors.Unwrap(o.Validate()))

	o.DeployOptions.Entrypoint.Name = "bad_entry_point"
	assert.Equal(ErrUnderlineInEntrypointName, errors.Unwrap(o.Validate()))

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
	assert.Equal(ErrEmptyNodeName, errors.Unwrap(o.Validate()))

	o.Nodename = "nodename"
	assert.Equal(ErrEmptyPodName, errors.Unwrap(o.Validate()))

	o.Podname = "podname"
	assert.Equal(ErrEmptyNodeEndpoint, errors.Unwrap(o.Validate()))

	o.Endpoint = "tcp://endpoint:2376"
	o.CPU = -1
	o.Share = -1
	o.Memory = -1
	o.NumaMemory = NUMAMemory{"0": -1}
	o.Volume = VolumeMap{"/data": -1}
	o.Storage = -1

	assert.Equal(ErrNegativeCPU, errors.Unwrap(o.Validate()))
	o.CPU = 1
	assert.Equal(ErrNegativeShare, errors.Unwrap(o.Validate()))
	o.Share = 100
	assert.Equal(ErrNegativeMemory, errors.Unwrap(o.Validate()))
	o.Memory = 100
	assert.Equal(ErrNegativeNUMAMemory, errors.Unwrap(o.Validate()))
	o.NumaMemory = nil
	assert.Equal(ErrNegativeVolumeSize, errors.Unwrap(o.Validate()))
	o.Volume = nil
	assert.Equal(ErrNegativeStorage, errors.Unwrap(o.Validate()))
	o.Storage = 1
	assert.NoError(o.Validate())
}

func TestImageOptions(t *testing.T) {
	assert := assert.New(t)

	o := &ImageOptions{Step: -1}
	assert.Equal(ErrEmptyPodName, errors.Unwrap(o.Validate()))

	o.Podname = "podname"
	assert.NoError(o.Validate())

	assert.Equal(o.Step, -1)
	o.Normalize()
	assert.Equal(o.Step, 1)

	o.Step = 3
	o.Normalize()
	assert.Equal(o.Step, 3)
}
