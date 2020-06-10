package calcium

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// ContextFunc .
type contextFunc = func(context.Context) error

// Transaction provides unified API to perform tnx
func (c *Calcium) Transaction(ctx context.Context, cond contextFunc, then contextFunc, rollback contextFunc) error {
	var tnxErr error
	defer func() { // rollback
		if tnxErr == nil {
			return
		}
		if rollback == nil {
			log.Error("[Transaction] tnx failed but no rollback function")
			return
		}
		// forbid interrupting rollback
		if err := rollback(context.Background()); err != nil {
			log.Errorf("[Transaction] tnx failed but rollback also failed, %v", err)
		}
	}()

	// let caller decide process then or not
	if tnxErr = cond(ctx); tnxErr == nil && then != nil {
		tnxErr = then(ctx)
	}

	return tnxErr
}
