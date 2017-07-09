package etcdstore

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"

	"gitlab.ricebook.net/platform/core/types"
)

func (k *krypton) GetDeployStatus(opts *types.DeployOptions) ([]types.DeployStatus, error) {
	var err error
	key, err := makeKey(opts)
	if err != nil {
		return nil, err
	}
	resp, err := k.etcd.Get(context.Background(), key, nil)
	if err != nil {
		return nil, err
	}
	status := []types.DeployStatus{}
	if err = json.Unmarshal([]byte(resp.Node.Value), &status); err != nil {
		return nil, err
	}
	return status, nil
}

func (k *krypton) UpdateDeployStatus(opts *types.DeployOptions, status []types.DeployStatus) error {
	var err error
	key, err := makeKey(opts)
	if err != nil {
		return err
	}
	v, err := json.Marshal(status)
	if err != nil {
		return err
	}
	if _, err = k.etcd.Set(context.Background(), key, fmt.Sprintf("%s", v), nil); err != nil {
		return err
	}
	return nil
}

func (k *krypton) RemoveDeployStatus(opts *types.DeployOptions) error {
	var err error
	key, err := makeKey(opts)
	if err != nil {
		return err
	}
	if _, err = k.etcd.Delete(context.Background(), key, nil); err != nil {
		return err
	}
	return nil
}

func makeKey(opts *types.DeployOptions) (string, error) {
	// 可以再考虑多种情况
	key := fmt.Sprintf("%s|%s|%s", opts.Appname, opts.Podname, opts.Entrypoint)
	h := sha1.New()
	if _, err := h.Write([]byte(key)); err != nil {
		return "", err
	}
	bs := h.Sum(nil)
	return fmt.Sprintf(deployStatusKey, bs), nil
}
