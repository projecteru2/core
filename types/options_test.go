package types

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/buffer"
)

func TestParseTriOption(t *testing.T) {
	assert.False(t, ParseTriOption(TriFalse, true))
	assert.True(t, ParseTriOption(TriTrue, false))
	assert.False(t, ParseTriOption(TriKeep, false))
	assert.True(t, ParseTriOption(TriKeep, true))
}

func TestSetNodeOptions(t *testing.T) {
	o := &SetNodeOptions{
		ResourceOpts: map[string]interface{}{},
	}
	assert.Equal(t, ErrEmptyNodeName, o.Validate())

	o.Nodename = "nodename"
	assert.NoError(t, o.Validate())
}

func TestDeployOptions(t *testing.T) {
	assert := assert.New(t)

	o := &DeployOptions{Entrypoint: &Entrypoint{}}
	assert.Equal(ErrEmptyAppName, o.Validate())

	assert.NotNil(t, o.GetProcessing("t"))

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

func TestLinuxFile(t *testing.T) {
	lf := LinuxFile{Filename: "s"}
	nlf := lf.Clone()
	assert.Equal(t, lf.Filename, nlf.Filename)
	assert.NotEmpty(t, lf.String())
	b := bufio.NewWriter(&buffer.Buffer{})
	lf.LitterDump(b)
	assert.NoError(t, b.Flush())
}

func TestSendOptions(t *testing.T) {
	assert := assert.New(t)

	o := &SendOptions{}
	assert.Equal(ErrNoWorkloadIDs, o.Validate())

	o.IDs = []string{"workload_id1", "workload_id2"}
	assert.Equal(ErrNoFilesToSend, o.Validate())

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
	assert.Equal(ErrInvaildNodeEndpoint, o.Validate())

	o.Endpoint = "tcp://endpoint:2376"
	assert.NoError(o.Validate())
}

func TestImageOptions(t *testing.T) {
	assert := assert.New(t)

	o := &ImageOptions{}
	assert.Equal(ErrEmptyPodName, o.Validate())

	o.Podname = "podname"
	assert.NoError(o.Validate())
}

func TestRawArges(t *testing.T) {
	ra := RawArgs([]byte("abc"))
	assert.Equal(t, ra.String(), "abc")
	b := bufio.NewWriter(&buffer.Buffer{})
	ra.LitterDump(b)
	assert.NoError(t, b.Flush())
}
