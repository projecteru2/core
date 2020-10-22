package utils

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// ContextFunc .
type contextFunc = func(context.Context) error

// Txn provides unified API to perform txn
func Txn(ctx context.Context, cond contextFunc, then contextFunc, rollback contextFunc, ttl time.Duration) error {
	var txnErr error
	txnCtx, txnCancel := context.WithTimeout(ctx, ttl)
	defer txnCancel()
	defer func() { // rollback
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
		if err := rollback(rollbackCtx); err != nil {
			log.Errorf("[txn] txn failed but rollback also failed, %v", err)
		}
	}()

	// let caller decide process then or not
	if txnErr = cond(txnCtx); txnErr == nil && then != nil {
		// no rollback and forbid interrupting further process
		var thenCtx context.Context = txnCtx
		var thenCancel context.CancelFunc
		if rollback == nil {
			thenCtx, thenCancel = context.WithTimeout(context.Background(), ttl)
			defer thenCancel()
		}
		txnErr = then(thenCtx)
	}

	return txnErr
}
