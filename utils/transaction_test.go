package utils

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTxn(t *testing.T) {
	err1 := errors.New("err1")
	err := Txn(
		context.Background(),
		func(context.Context) error {
			return err1
		},
		nil,
		func(context.Context, bool) error {
			return errors.New("error 2")
		},
		10*time.Second,
	)
	assert.Contains(t, err.Error(), err1.Error())
	err = Txn(
		context.Background(),
		func(context.Context) error {
			return nil
		},
		func(context.Context) error {
			return err1
		},
		nil,
		10*time.Second,
	)
	assert.Contains(t, err.Error(), err1.Error())
	err = Txn(
		context.Background(),
		func(context.Context) error {
			return nil
		},
		nil,
		nil,
		10*time.Second,
	)
	assert.NoError(t, err)
}

func TestPCR(t *testing.T) {
	prepare := func(context.Context) error {
		return os.ErrClosed
	}
	commit := func(context.Context) error {
		return os.ErrClosed
	}

	ctx := context.Background()
	assert.Error(t, PCR(ctx, prepare, commit, commit, time.Second))
}
