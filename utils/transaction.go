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

		log.Warnf("[txn] rollback due to %+v", txnErr)
		// forbid interrupting rollback
		rollbackCtx, rollBackCancel := context.WithTimeout(context.Background(), ttl)
		defer rollBackCancel()
		failureByCond := condErr != nil
		if err := rollback(rollbackCtx, failureByCond); err != nil {
			log.Errorf("[txn] txn failed but rollback also failed, %v", err)
		}
	}()

	// let caller decide process then or not
	if condErr = cond(txnCtx); condErr == nil && then != nil {
		// no rollback and forbid interrupting further process
		var thenCtx context.Context = txnCtx
		var thenCancel context.CancelFunc
		if rollback == nil {
			thenCtx, thenCancel = context.WithTimeout(context.Background(), ttl)
			defer thenCancel()
		}
		thenErr = then(thenCtx)
	}

	return txnErr
}
