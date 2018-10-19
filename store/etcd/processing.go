package etcdstore

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// SaveProcessing save processing status in etcd
func (k *Krypton) SaveProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error {
	///Appname/Entrypoint/Nodename/Ident
	processingKey := filepath.Join(containerProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodeInfo.Name, opts.ProcessIdent)
	_, err := k.etcd.Create(ctx, processingKey, fmt.Sprintf("%d", nodeInfo.Deploy))
	return err
}

// UpdateProcessing update processing status in etcd
func (k *Krypton) UpdateProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error {
	///Appname/Entrypoint/Nodename/Ident
	processingKey := filepath.Join(containerProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodename, opts.ProcessIdent)
	_, err := k.etcd.Update(ctx, processingKey, fmt.Sprintf("%d", count))
	return err
}

// DeleteProcessing delete processing status in etcd
func (k *Krypton) DeleteProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error {
	///Appname/Entrypoint/Nodename/Ident
	processingKey := filepath.Join(containerProcessingPrefix, opts.Name, opts.Entrypoint.Name, nodeInfo.Name, opts.ProcessIdent)
	_, err := k.etcd.Delete(ctx, processingKey, &etcdclient.DeleteOptions{Recursive: true})
	return err
}

func (k *Krypton) loadProcessing(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	processingKey := filepath.Join(containerProcessingPrefix, opts.Name, opts.Entrypoint.Name)
	resp, err := k.etcd.Get(ctx, processingKey, &etcdclient.GetOptions{Recursive: true})
	if err != nil {
		if etcdclient.IsKeyNotFound(err) {
			err = nil
		}
		return nodesInfo, err
	}
	nodesCount := map[string]int{}
	for _, node := range resp.Node.Nodes {
		nodeName := strings.Trim(strings.TrimPrefix(node.Key, resp.Node.Key), "/")
		nodesCount[nodeName] = 0
		for _, process := range node.Nodes {
			count, err := strconv.Atoi(process.Value)
			if err != nil {
				log.Errorf("[loadProcessing] load processing status failed %v", err)
				continue
			}
			nodesCount[nodeName] += count
		}
	}

	for i := range nodesInfo {
		if v, ok := nodesCount[nodesInfo[i].Name]; ok {
			nodesInfo[i].Count += v
		}
	}
	return nodesInfo, nil
}
