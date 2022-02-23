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
	defer func() { // rollback
		txnErr = condErr
		if txnErr == nil {
			txnErr = thenErr
		}
		if txnErr == nil {
			return
		}
		if rollback == nil {
			log.Error("[txn] txn failed but no rollback function")
			return
		}

		log.Warnf(ctx, "[txn] txn failed, rolling back: %v", txnErr)
		// forbid interrupting rollback
		rollbackCtx, rollBackCancel := context.WithTimeout(InheritTracingInfo(ctx, context.TODO()), ttl)
		defer rollBackCancel()
		failureByCond := condErr != nil
		if err := rollback(rollbackCtx, failureByCond); err != nil {
			log.Warnf(ctx, "[txn] txn failed but rollback also failed: %v", err)
		}
	}()

	// let caller decide process then or not
	if condErr = cond(txnCtx); condErr == nil && then != nil {
		// no rollback and forbid interrupting further process
		thenCtx := txnCtx
		var thenCancel context.CancelFunc
		if rollback == nil {
			thenCtx, thenCancel = context.WithTimeout(InheritTracingInfo(ctx, context.TODO()), ttl)
			defer thenCancel()
		}
		thenErr = then(thenCtx)
	}

	return txnErr
}

// Pcr Prepare, Commit, Rollback.
// `prepare` should be a pure calculation process without side effects.
// `commit` writes the calculation result of `prepare` into database.
// if `commit` returns error, `rollback` will be performed.
func Pcr(ctx context.Context, prepare func(ctx context.Context) error, commit func(ctx context.Context) error, rollback func(ctx context.Context) error, ttl time.Duration) error {
	return Txn(ctx, prepare, commit, func(ctx context.Context, failureByCond bool) error {
		if !failureByCond {
			return rollback(ctx)
		}
		return nil
	}, ttl)
}
