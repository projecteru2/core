package calcium

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type inStream struct {
	*bytes.Buffer
}

func (i *inStream) Close() error {
	return nil
}

func TestExecuteWorkload(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	// failed by GetWorkload
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrInvaildCount).Once()
	ID := "abc"
	ch := c.ExecuteWorkload(ctx, &types.ExecuteWorkloadOptions{WorkloadID: ID}, nil)
	for ac := range ch {
		assert.NotEmpty(t, ac.Data)
	}

	engine := &enginemocks.API{}
	workload := &types.Workload{
		ID:     ID,
		Engine: engine,
	}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)

	// failed by Execute
	result := "def"
	engine.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return(result, nil, nil, nil, types.ErrMockError).Once()
	ch = c.ExecuteWorkload(ctx, &types.ExecuteWorkloadOptions{WorkloadID: ID}, nil)
	for ac := range ch {
		assert.Equal(t, ac.WorkloadID, ID)
	}
	buf := io.NopCloser(bytes.NewBufferString(`echo 1\n`))
	engine.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return(result, buf, nil, nil, nil).Twice()

	// failed by ExecExitCode
	engine.On("ExecExitCode", mock.Anything, mock.Anything, mock.Anything).Return(-1, types.ErrMockError).Once()
	ch = c.ExecuteWorkload(ctx, &types.ExecuteWorkloadOptions{WorkloadID: ID}, nil)
	data := []byte{}
	for ac := range ch {
		assert.Equal(t, ac.WorkloadID, ID)
		data = append(data, ac.Data...)
	}
	assert.Contains(t, string(data), "echo")
	engine.On("ExecExitCode", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
	ch = c.ExecuteWorkload(ctx, &types.ExecuteWorkloadOptions{WorkloadID: ID}, nil)
	for ac := range ch {
		assert.Equal(t, ac.WorkloadID, ID)
		data = append(data, ac.Data...)
	}
	assert.Contains(t, string(data), "exitcode")
	assert.Contains(t, string(data), "0")
	inChan := make(chan []byte)
	inS := &inStream{bytes.NewBufferString("")}
	engine.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return(ID, buf, nil, inS, nil)
	ch = c.ExecuteWorkload(ctx, &types.ExecuteWorkloadOptions{WorkloadID: ID, OpenStdin: true}, inChan)
	inChan <- []byte("a")
	inChan <- escapeCommand
	engine.On("ExecResize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(types.ErrAlreadyFilled)
	w := &window{100, 100}
	b, err := json.Marshal(w)
	assert.NoError(t, err)
	inChan <- append(winchCommand, []byte(`{Row: 100, Col: 100}`)...)
	inChan <- append(winchCommand, b...)
	for ac := range ch {
		assert.Equal(t, ac.WorkloadID, ID)
		data = append(data, ac.Data...)
	}
	assert.Contains(t, inS.String(), "a")
}
