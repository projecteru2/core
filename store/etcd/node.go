package etcdstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"strconv"
	"strings"

	etcdclient "github.com/coreos/etcd/client"
	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// AddNode save it to etcd
// storage path in etcd is `/pod/:podname/node/:nodename/info`
func (k *Krypton) AddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, cpu int, share, memory int64, labels map[string]string) (*types.Node, error) {
	if !strings.HasPrefix(endpoint, "tcp://") {
		return nil, fmt.Errorf("Endpoint must starts with tcp:// %q", endpoint)
	}

	_, err := k.GetPod(ctx, podname)
	if err != nil {
		return nil, err
	}

	nodeKey := fmt.Sprintf(nodeInfoKey, podname, name)
	if _, err := k.etcd.Get(ctx, nodeKey, nil); err == nil {
		return nil, fmt.Errorf("Node (%s, %s) already exists", podname, name)
	}

	// 如果有tls的证书需要保存就保存一下
	if ca != "" && cert != "" && key != "" {
		_, err = k.etcd.Set(ctx, fmt.Sprintf(nodeCaKey, podname, name), ca, nil)
		if err != nil {
			return nil, err
		}
		_, err = k.etcd.Set(ctx, fmt.Sprintf(nodeCertKey, podname, name), cert, nil)
		if err != nil {
			return nil, err
		}
		_, err = k.etcd.Set(ctx, fmt.Sprintf(nodeKeyKey, podname, name), key, nil)
		if err != nil {
			return nil, err
		}
	}

	// 尝试加载docker的客户端
	engine, err := k.makeDockerClient(ctx, podname, name, endpoint, true)
	if err != nil {
		k.deleteNode(ctx, podname, name, endpoint)
		return nil, err
	}

	// 判断这货是不是活着的
	info, err := engine.Info(ctx)
	if err != nil {
		k.deleteNode(ctx, podname, name, endpoint)
		return nil, err
	}

	ncpu := cpu
	memcap := memory
	if cpu == 0 {
		ncpu = info.NCPU
	}
	if memory == 0 {
		memcap = info.MemTotal - gigabyte
	}
	if share == 0 {
		share = k.config.Scheduler.ShareBase
	}
	cpumap := types.CPUMap{}
	for i := 0; i < ncpu; i++ {
		cpumap[strconv.Itoa(i)] = share
	}

	node := &types.Node{
		Name:      name,
		Endpoint:  endpoint,
		Podname:   podname,
		CPU:       cpumap,
		MemCap:    memcap,
		Available: true,
		Labels:    labels,
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		k.deleteNode(ctx, podname, name, endpoint)
		return nil, err
	}

	_, err = k.etcd.Create(ctx, nodeKey, string(bytes))
	if err != nil {
		k.deleteNode(ctx, podname, name, endpoint)
		return nil, err
	}

	_, err = k.etcd.Create(ctx, fmt.Sprintf(nodePodKey, name), podname)
	if err != nil {
		k.deleteNode(ctx, podname, name, endpoint)
		return nil, err
	}

	return node, nil
}

// 因为是先写etcd的证书再拿client
// 所以可能出现实际上node创建失败但是却写好了证书的情况
// 所以需要删除这些留存的证书
// 至于结果是不是成功就无所谓了
func (k *Krypton) deleteNode(ctx context.Context, podname, nodename, endpoint string) {
	key := fmt.Sprintf(nodePrefixKey, podname, nodename)
	k.etcd.Delete(ctx, key, &etcdclient.DeleteOptions{Recursive: true})
	k.etcd.Delete(ctx, fmt.Sprintf(nodePodKey, nodename), &etcdclient.DeleteOptions{})
	u, err := url.Parse(endpoint)
	if err != nil {
		log.Errorf("[deleteNode] Bad endpoint: %s", endpoint)
		return
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		log.Errorf("[deleteNode] Bad addr: %s", u.Host)
		return
	}
	_cache.delete(host)
	log.Debugf("[deleteNode] Node (%s, %s, %s) deleted", podname, nodename, endpoint)
}

// DeleteNode delete a node
func (k *Krypton) DeleteNode(ctx context.Context, node *types.Node) {
	k.deleteNode(ctx, node.Podname, node.Name, node.Endpoint)
}

// GetNode get a node from etcd
// and construct it's docker client
// a node must belong to a pod
// and since node is not the smallest unit to user, to get a node we must specify the corresponding pod
// storage path in etcd is `/pod/:podname/node/:nodename/info`
func (k *Krypton) GetNode(ctx context.Context, podname, nodename string) (*types.Node, error) {
	key := fmt.Sprintf(nodeInfoKey, podname, nodename)
	resp, err := k.etcd.Get(ctx, key, nil)
	if err != nil {
		return nil, err
	}
	if resp.Node.Dir {
		return nil, fmt.Errorf("Node storage path %q in etcd is a directory", key)
	}

	node := &types.Node{}
	if err := json.Unmarshal([]byte(resp.Node.Value), node); err != nil {
		return nil, err
	}

	engine, err := k.makeDockerClient(ctx, podname, nodename, node.Endpoint, false)
	if err != nil {
		return nil, err
	}

	node.Engine = engine
	return node, nil
}

// GetNodeByName get node by name
func (k *Krypton) GetNodeByName(ctx context.Context, nodename string) (node *types.Node, err error) {
	key := fmt.Sprintf(nodePodKey, nodename)
	resp, err := k.etcd.Get(ctx, key, nil)
	if err != nil {
		return nil, err
	}
	podname := resp.Node.Value
	return k.GetNode(ctx, podname, nodename)
}

// GetNodesByPod get all nodes bound to pod
// here we use podname instead of pod instance
// storage path in etcd is `/pod/:podname/node`
func (k *Krypton) GetNodesByPod(ctx context.Context, podname string) (nodes []*types.Node, err error) {
	key := fmt.Sprintf(podNodesKey, podname)
	resp, err := k.etcd.Get(ctx, key, nil)
	if err != nil {
		return nodes, err
	}
	if !resp.Node.Dir {
		return nil, fmt.Errorf("Node storage path %q in etcd is not a directory", key)
	}

	for _, node := range resp.Node.Nodes {
		nodename := utils.Tail(node.Key)
		n, err := k.GetNode(ctx, podname, nodename)
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, n)
	}
	return nodes, err
}

// GetAllNodes get all nodes from etcd
// any error will break and return immediately
func (k *Krypton) GetAllNodes(ctx context.Context) (nodes []*types.Node, err error) {
	pods, err := k.GetAllPods(ctx)
	if err != nil {
		return nodes, err
	}

	for _, pod := range pods {
		ns, err := k.GetNodesByPod(ctx, pod.Name)
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, ns...)
	}
	return nodes, err
}

// UpdateNode update a node, save it to etcd
// storage path in etcd is `/pod/:podname/node/:nodename/info`
func (k *Krypton) UpdateNode(ctx context.Context, node *types.Node) error {
	lock, err := k.CreateLock(fmt.Sprintf("%s_%s", node.Podname, node.Name), k.config.LockTimeout)
	if err != nil {
		return err
	}

	if err := lock.Lock(ctx); err != nil {
		return err
	}
	defer lock.Unlock(ctx)

	key := fmt.Sprintf(nodeInfoKey, node.Podname, node.Name)
	bytes, err := json.Marshal(node)
	if err != nil {
		return err
	}

	_, err = k.etcd.Set(ctx, key, string(bytes), nil)
	return err
}

// UpdateNodeResource update cpu and mem on a node, either add or substract
// need to lock
func (k *Krypton) UpdateNodeResource(ctx context.Context, podname, nodename string, cpu types.CPUMap, mem int64, action string) error {
	lock, err := k.CreateLock(fmt.Sprintf("%s_%s", podname, nodename), k.config.LockTimeout)
	if err != nil {
		return err
	}

	if err := lock.Lock(ctx); err != nil {
		return err
	}
	defer lock.Unlock(ctx)

	nodeKey := fmt.Sprintf(nodeInfoKey, podname, nodename)
	resp, err := k.etcd.Get(ctx, nodeKey, nil)
	if err != nil {
		return err
	}
	if resp.Node.Dir {
		return fmt.Errorf("Node storage path %q in etcd is a directory", nodeKey)
	}

	node := &types.Node{}
	if err := json.Unmarshal([]byte(resp.Node.Value), node); err != nil {
		return err
	}

	if action == "add" || action == "+" {
		node.CPU.Add(cpu)
		node.MemCap += mem
	} else if action == "sub" || action == "-" {
		node.CPU.Sub(cpu)
		node.MemCap -= mem
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		return err
	}

	log.Debugf("[UpdateNodeResource] new node info: %s", string(bytes))
	_, err = k.etcd.Set(ctx, nodeKey, string(bytes), nil)
	return err
}

func (k *Krypton) makeDockerClient(ctx context.Context, podname, nodename, endpoint string, force bool) (*engineapi.Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, err
	}

	// try get client, if nil, create a new one
	var client *engineapi.Client
	client = _cache.get(host)
	if client == nil || force {
		if k.config.Docker.CertPath == "" {
			client, err = makeRawClient(endpoint, k.config.Docker.APIVersion)
		} else {
			keyFormats := []string{nodeCaKey, nodeCertKey, nodeKeyKey}
			data := []string{}
			for i := 0; i < 3; i++ {
				resp, err := k.etcd.Get(ctx, fmt.Sprintf(keyFormats[i], podname, nodename), nil)
				if err != nil {
					return nil, err
				}
				data = append(data, resp.Node.Value)
			}
			caFile, err := ioutil.TempFile(k.config.Docker.CertPath, fmt.Sprintf("ca-%s", host))
			if err != nil {
				return nil, err
			}
			certFile, err := ioutil.TempFile(k.config.Docker.CertPath, fmt.Sprintf("cert-%s", host))
			if err != nil {
				return nil, err
			}
			keyFile, err := ioutil.TempFile(k.config.Docker.CertPath, fmt.Sprintf("key-%s", host))
			if err != nil {
				return nil, err
			}
			if err := dumpFromString(caFile, certFile, keyFile, data[0], data[1], data[2]); err != nil {
				return nil, err
			}
			client, err = makeRawClientWithTLS(caFile, certFile, keyFile, endpoint, k.config.Docker.APIVersion)
		}
		if err != nil {
			return nil, err
		}
		_cache.set(host, client)
	}
	return client, nil
}
