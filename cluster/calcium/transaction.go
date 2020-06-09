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
	defer func() {
		if tnxErr != nil && rollback != nil {
			// forbid interrupting rollback
			ctx, cancel := context.WithTimeout(context.Background(), c.config.GlobalTimeout)
			defer cancel()
			if e := rollback(ctx); e != nil {
				log.Errorf("[Transaction] tnx failed but rollback also failed")
			}
		}

		if tnxErr != nil && rollback == nil {
			log.Errorf("[Transaction] tnx failed but no rollback function")
		}
	}()

	if tnxErr = cond(ctx); tnxErr == nil {
		// no rollback and forbid interrupting further process
		var cancel context.CancelFunc
		if rollback == nil {
			ctx, cancel = context.WithTimeout(context.Background(), c.config.GlobalTimeout)
			defer cancel()
		}
		if then != nil {
			tnxErr = then(ctx)
		}
	}

	return tnxErr
}
