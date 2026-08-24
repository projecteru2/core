package utils

import (
	"context"
	"time"

	"github.com/projecteru2/core/log"
)

type contextFunc = func(context.Context) error

// Txn runs cond then then; on any error it runs rollback under a fresh ttl-bounded context.
func Txn(ctx context.Context, cond, then contextFunc, rollback func(context.Context, bool) error, ttl time.Duration) (txnErr error) {
	var condErr, thenErr error
	txnCtx, txnCancel := context.WithTimeout(ctx, ttl)
	defer txnCancel()
	logger := log.WithFunc("utils.Txn")
	defer func() {
		txnErr = condErr
		if txnErr == nil {
			txnErr = thenErr
		}
		if txnErr == nil {
			return
		}
		if rollback == nil {
			logger.Warn(ctx, "txn failed but no rollback function")
			return
		}

		logger.Error(ctx, txnErr, "txn failed, rolling back")

		// rollback must survive cancellation of ctx
		rollbackCtx, rollBackCancel := context.WithTimeout(NewInheritCtx(ctx), ttl)
		defer rollBackCancel()
		failureByCond := condErr != nil
		if err := rollback(rollbackCtx, failureByCond); err != nil {
			logger.Warnf(ctx, "txn failed but rollback also failed: %+v", err)
		}
	}()

	if condErr = cond(txnCtx); condErr == nil && then != nil {
		// with no rollback, then must not be interruptible
		thenCtx := txnCtx
		var thenCancel context.CancelFunc
		if rollback == nil {
			thenCtx, thenCancel = context.WithTimeout(NewInheritCtx(ctx), ttl)
			defer thenCancel()
		}
		thenErr = then(thenCtx)
	}

	return txnErr
}

// PCR runs prepare, commit and rollback; prepare must be side-effect free.
func PCR(ctx context.Context, prepare, commit, rollback func(ctx context.Context) error, ttl time.Duration) error {
	return Txn(ctx, prepare, commit, func(ctx context.Context, failureByCond bool) error {
		if !failureByCond {
			return rollback(ctx)
		}
		return nil
	}, ttl)
}
