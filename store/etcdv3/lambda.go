package etcdv3

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/etcd/clientv3"

	"github.com/projecteru2/core/types"
)

// WatchLambda watches Lambda.
func (m *Mercury) WatchLambda(ctx context.Context) <-chan *types.LambdaStatus {
	ch := make(chan *types.LambdaStatus)

	go func() {
		prefix := makeLambdaKey("")

		defer log.Infof("[WatchLambda] exit from Lambda watcher: %s", prefix)
		defer close(ch)

		var err error

		for resp := range m.watch(ctx, prefix, clientv3.WithPrefix()) {
			if resp.Err() != nil {
				log.Errorf("[WatchLambda] watch failed: %v", resp.Err())
				return
			}

			for _, event := range resp.Events {
				msg := &types.LambdaStatus{}
				if msg.Lambda, err = m.decodeLambda(event.Kv.Value); err != nil {
					log.Errorf("[WatchLambda] decode Lambda %s failed %v", event.Kv.Value, err)
					msg.Error = err
				}

				ch <- msg
			}
		}
	}()

	return ch
}

// GetLambda reads a Lambda from metadata.
func (m *Mercury) GetLambda(ctx context.Context, id string) (*types.Lambda, error) {
	lambdas, err := m.GetLambdas(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return lambdas[0], nil
}

// GetLambdas gets exactly matched Lambdas from metadata.
func (m *Mercury) GetLambdas(ctx context.Context, ids []string) (lambdas []*types.Lambda, err error) {
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = makeLambdaKey(id)
	}

	kvs, err := m.GetMulti(ctx, keys)
	if err != nil {
		return nil, err
	}

	lambdas = make([]*types.Lambda, len(ids))

	for i, kv := range kvs {
		if lambdas[i], err = m.decodeLambda(kv.Value); err != nil {
			return nil, err
		}
	}

	return
}

func (m *Mercury) decodeLambda(bytes []byte) (*types.Lambda, error) {
	lambda := &types.Lambda{}
	err := json.Unmarshal(bytes, lambda)
	return lambda, err
}

// AddLambda persists the Lambda.
func (m *Mercury) AddLambda(ctx context.Context, lambda *types.Lambda) error {
	if len(lambda.ID) < 1 {
		lambda.ID = m.newLambdaID()
	}

	bytes, err := json.Marshal(lambda)
	if err != nil {
		return err
	}

	data := map[string]string{makeLambdaKey(lambda.ID): string(bytes)}

	_, err = m.batchCreate(ctx, data)

	return err
}

func (m *Mercury) newLambdaID() string {
	id := uuid.New()
	return strings.ReplaceAll(id.String(), "-", "")
}
