package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coreos/etcd/clientv3"
	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func main() {
	if len(os.Args) != 2 {
		panic("wrong args")
	}
	config, _ := utils.LoadConfig(os.Args[1])
	m, err := etcdv3.New(config, false)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	ctx := context.Background()

	containerInfoKey := "/containers/"
	resp, err := m.Get(ctx, containerInfoKey, clientv3.WithPrefix())
	if err != nil {
		panic(err)
	}
	containerDeployPrefix := "/deploy"

	for _, kvs := range resp.Kvs {
		container := &types.Container{}
		if err := json.Unmarshal(kvs.Value, container); err != nil {
			panic(err)
		}
		appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
		if err != nil {
			panic(err)
		}
		fmt.Println(appname, entrypoint, container.Nodename)
		key := filepath.Join(containerDeployPrefix, appname, entrypoint, container.Nodename, container.ID)
		if _, err := m.Put(ctx, key, string(kvs.Value)); err != nil {
			panic(err)
		}
	}
}
