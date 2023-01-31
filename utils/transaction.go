package utils

import (
	"context"
	"time"

	"github.com/projecteru2/core/log"
)

// ContextFunc .
type contextFunc = func(context.Context) error

// Txn provides unified API to perform txn
func Txn(ctx context.Context, cond contextFunc, then contextFunc, rollback func(context.Context, bool) error, ttl time.Duration) (txnErr error) {
	var condErr, thenErr error
	txnCtx, txnCancel := context.WithTimeout(ctx, ttl)
	defer txnCancel()
	logger := log.WithFunc("utils.Txn")
	defer func() { // rollback
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

		// forbid interrupting rollback
		rollbackCtx, rollBackCancel := context.WithTimeout(NewInheritCtx(ctx), ttl)
		defer rollBackCancel()
		failureByCond := condErr != nil
		if err := rollback(rollbackCtx, failureByCond); err != nil {
			logger.Warnf(ctx, "txn failed but rollback also failed: %+v", err)
		}
	}()

	// let caller decide process then or not
	if condErr = cond(txnCtx); condErr == nil && then != nil {
		// no rollback and forbid interrupting further process
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

// PCR Prepare, Commit, Rollback.
// `prepare` should be a pure calculation process without side effects.
// `commit` writes the calculation result of `prepare` into database.
// if `commit` returns error, `rollback` will be performed.
func PCR(ctx context.Context, prepare func(ctx context.Context) error, commit func(ctx context.Context) error, rollback func(ctx context.Context) error, ttl time.Duration) error {
	return Txn(ctx, prepare, commit, func(ctx context.Context, failureByCond bool) error {
		if !failureByCond {
			return rollback(ctx)
		}
		return nil
	}, ttl)
}
