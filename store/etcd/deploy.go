package etcdstore

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strconv"

	etcdclient "github.com/coreos/etcd/client"
	"gitlab.ricebook.net/platform/core/types"
)

func (k *krypton) UpdateDeployStatus(opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	var err error
	prefix, err := makePrefix(opts)
	if err != nil {
		return nodesInfo, err
	}

	for _, nodeInfo := range nodesInfo {
		key := fmt.Sprintf(deployStatusKey, prefix, nodeInfo.Name)
		resp, err := k.etcd.Get(context.Background(), key, nil)
		if err != nil {
			if etcdclient.IsKeyNotFound(err) {
				continue
			} else {
				return nodesInfo, err
			}
		}
		nodeInfo.Count, _ = strconv.Atoi(resp.Node.Value)
	}
	return nodesInfo, nil
}

func (k *krypton) StoreNodeStatus(opts *types.DeployOptions, name string, value int) error {
	var err error
	prefix, err := makePrefix(opts)
	if err != nil {
		return err
	}

	key := fmt.Sprintf(deployStatusKey, prefix, name)
	old := 0
	if resp, err := k.etcd.Get(context.Background(), key, nil); !etcdclient.IsKeyNotFound(err) {
		return err
	} else if err == nil {
		old, _ = strconv.Atoi(resp.Node.Value)
	}
	// 更新数据
	value += old
	_, err = k.etcd.Set(context.Background(), key, fmt.Sprintf("%d", value), nil)
	return err
}

//func (k *krypton) UpdateDeployStatus(opts *types.DeployOptions, status []types.DeployStatus) error {
//	var err error
//	key, err := makeKey(opts)
//	if err != nil {
//		return err
//	}
//	v, err := json.Marshal(status)
//	if err != nil {
//		return err
//	}
//	if _, err = k.etcd.Set(context.Background(), key, fmt.Sprintf("%s", v), nil); err != nil {
//		return err
//	}
//	return nil
//}
//
//func (k *krypton) RemoveDeployStatus(opts *types.DeployOptions) error {
//	var err error
//	key, err := makeKey(opts)
//	if err != nil {
//		return err
//	}
//	if _, err = k.etcd.Delete(context.Background(), key, nil); err != nil {
//		return err
//	}
//	return nil
//}

func makePrefix(opts *types.DeployOptions) (string, error) {
	// 可以再考虑多种情况
	key := fmt.Sprintf("%s|%s|%s", opts.Appname, opts.Podname, opts.Entrypoint)
	h := sha1.New()
	if _, err := h.Write([]byte(key)); err != nil {
		return "", err
	}
	bs := h.Sum(nil)
	return fmt.Sprintf("%s", bs), nil
}
