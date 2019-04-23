package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/coreos/etcd/clientv3"
	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/types"
)

const (
	oldNodeInfoKey = "/pod/nodes/%s/%s"
	nodeInfoKey    = "/pod/%s:nodes/%s" // /pod/{podname}:nodes/{nodenmae} -> for node info

	oldNodeCaKey         = "/node/ca/%s"
	oldNodeCertKey       = "/node/cert/%s"
	oldNodeKeyKey        = "/node/key/%s"
	oldNodePodKey        = "/node/pod/%s"           // /node/pod/node1 value -> podname
	oldNodeContainersKey = "/node/containers/%s/%s" // /node/containers/n1/[64]

	nodeCaKey         = "/node/%s:ca"            // /node/{nodename}:ca
	nodeCertKey       = "/node/%s:cert"          // /node/{nodename}:cert
	nodeKeyKey        = "/node/%s:key"           // /node/{nodename}:key
	nodePodKey        = "/node/%s:pod"           // /node/{nodename}:pod value -> podname
	nodeContainersKey = "/node/%s:containers/%s" // /node/{nodename}:containers/{containerID}
)

func main() {
	if len(os.Args) != 3 {
		panic("wrong args")
	}
	config := types.Config{
		Etcd: types.EtcdConfig{
			Prefix:   os.Args[1],
			Machines: []string{os.Args[2]},
		},
	}
	m, err := etcdv3.New(config)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	ctx := context.Background()
	pods, err := m.GetAllPods(ctx)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	prefix := "/pod/nodes/%s"
	for _, pod := range pods {
		podname := pod.Name
		keyPrefix := fmt.Sprintf(prefix, podname)
		rs, err := m.Get(ctx, keyPrefix, clientv3.WithPrefix())
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		for _, kv := range rs.Kvs {
			node := &types.Node{}
			json.Unmarshal(kv.Value, node)
			fmt.Println(node.Name)
			newKey := fmt.Sprintf(nodeInfoKey, podname, node.Name)
			oldKey := fmt.Sprintf(oldNodeInfoKey, podname, node.Name)
			fmt.Println(newKey)
			fmt.Println(oldKey)
			m.Put(ctx, newKey, string(kv.Value))
			m.Delete(ctx, oldKey)
			oldKey = fmt.Sprintf(oldNodePodKey, node.Name)
			newKey = fmt.Sprintf(nodePodKey, node.Name)
			kv, err := m.GetOne(ctx, oldKey)
			if err != nil {
				fmt.Println(err)
				panic(err)
			}
			m.Put(ctx, newKey, string(kv.Value))
			m.Delete(ctx, oldKey)
			oldKey = fmt.Sprintf(oldNodeCaKey, node.Name)
			newKey = fmt.Sprintf(nodeCaKey, node.Name)
			kv, err = m.GetOne(ctx, oldKey)
			if err == nil {
				m.Put(ctx, newKey, string(kv.Value))
				m.Delete(ctx, oldKey)
			}
			oldKey = fmt.Sprintf(oldNodeCertKey, node.Name)
			newKey = fmt.Sprintf(nodeCertKey, node.Name)
			kv, err = m.GetOne(ctx, oldKey)
			if err == nil {
				m.Put(ctx, newKey, string(kv.Value))
				m.Delete(ctx, oldKey)
			}
			oldKey = fmt.Sprintf(oldNodeKeyKey, node.Name)
			newKey = fmt.Sprintf(nodeKeyKey, node.Name)
			kv, err = m.GetOne(ctx, oldKey)
			if err == nil {
				m.Put(ctx, newKey, string(kv.Value))
				m.Delete(ctx, oldKey)
			}

			oldKeyPrefix := fmt.Sprintf(oldNodeContainersKey, node.Name, "")
			resp, err := m.Get(ctx, oldKeyPrefix, clientv3.WithPrefix())
			if err != nil {
				fmt.Println(err)
				panic(err)
			}
			for _, ev := range resp.Kvs {
				c := &types.Container{}
				json.Unmarshal(ev.Value, c)
				oldKey = fmt.Sprintf(oldNodeContainersKey, node.Name, c.ID)
				newKey = fmt.Sprintf(nodeContainersKey, node.Name, c.ID)
				m.Put(ctx, newKey, string(ev.Value))
				m.Delete(ctx, oldKey)
			}
		}
	}
}
