package utils

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// ContextFunc .
type contextFunc = func(context.Context) error

// Txn provides unified API to perform tnx
func Txn(ctx context.Context, cond contextFunc, then contextFunc, rollback contextFunc, ttl time.Duration) error {
	var tnxErr error
	txnCtx, txnCancel := context.WithTimeout(ctx, ttl)
	defer txnCancel()
	defer func() { // rollback
		if tnxErr == nil {
			return
		}
		if rollback == nil {
			log.Error("[txn] tnx failed but no rollback function")
			return
		}
		// forbid interrupting rollback
		rollbackCtx, rollBackCancel := context.WithTimeout(context.Background(), ttl)
		defer rollBackCancel()
		if err := rollback(rollbackCtx); err != nil {
			log.Errorf("[txn] tnx failed but rollback also failed, %v", err)
		}
	}()

	// let caller decide process then or not
	if tnxErr = cond(txnCtx); tnxErr == nil && then != nil {
		// no rollback and forbid interrupting further process
		var thenCtx context.Context = txnCtx
		var thenCancel context.CancelFunc
		if rollback == nil {
			thenCtx, thenCancel = context.WithTimeout(context.Background(), ttl)
			defer thenCancel()
		}
		tnxErr = then(thenCtx)
	}

	return tnxErr
}
