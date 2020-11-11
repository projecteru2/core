package utils

import (
	"context"
	"errors"
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
