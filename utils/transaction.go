package utils

import (
	"cmp"
	"context"
	"time"

	"github.com/projecteru2/core/log"
)

type (
	contextFunc  func(context.Context) error
	rollbackFunc func(context.Context, bool) error
)

// Txn runs cond then then, rolling back under a fresh ttl-bounded context on error; settled reports full compensation.
func Txn(ctx context.Context, cond, then contextFunc, rollback rollbackFunc, ttl time.Duration) (settled bool, txnErr error) {
	var condErr, thenErr error
	txnCtx, txnCancel := context.WithTimeout(ctx, ttl)
	defer txnCancel()
	logger := log.WithFunc("utils.Txn")
	defer func() {
		txnErr = cmp.Or(condErr, thenErr)
		if txnErr == nil {
			settled = true
			return
		}
		if rollback == nil {
			logger.Warn(ctx, "txn failed but no rollback function")
			return
		}

		logger.Error(ctx, txnErr, "txn failed, rolling back")

		// rollback must survive cancellation of ctx
		rollbackCtx, rollBackCancel := context.WithTimeout(context.WithoutCancel(ctx), ttl)
		defer rollBackCancel()
		failureByCond := condErr != nil
		if err := rollback(rollbackCtx, failureByCond); err != nil {
			logger.Warnf(ctx, "txn failed but rollback also failed: %+v", err)
			return
		}
		settled = true
	}()

	if cond != nil {
		condErr = cond(txnCtx)
	}
	if condErr == nil && then != nil {
		// with no rollback, then must not be interruptible
		thenCtx := txnCtx
		var thenCancel context.CancelFunc
		if rollback == nil {
			thenCtx, thenCancel = context.WithTimeout(context.WithoutCancel(ctx), ttl)
			defer thenCancel()
		}
		thenErr = then(thenCtx)
	}

	return settled, txnErr
}

// PCR runs prepare, commit and rollback; prepare must be side-effect free.
func PCR(ctx context.Context, prepare, commit, rollback contextFunc, ttl time.Duration) error {
	_, err := Txn(ctx, prepare, commit, func(ctx context.Context, failureByCond bool) error {
		if !failureByCond && rollback != nil {
			return rollback(ctx)
		}
		return nil
	}, ttl)
	return err
}
