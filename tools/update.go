package main

import (
	"context"
	"fmt"
	"os"

	"github.com/projecteru2/core/utils"

	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/types"
)

// go run update.go /eru http://ip:port
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
	for _, pod := range pods {
		nodes, err := m.GetNodesByPod(ctx, pod.Name)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		for _, node := range nodes {
			fmt.Println(node.Name)
			containers, err := m.ListNodeContainers(ctx, node.Name)
			if err != nil {
				fmt.Println(err)
				panic(err)
			}
			cpuused := 0.0
			for _, container := range containers {
				fmt.Println(container.ID[:7], container.Quota, container.CPU.Total())
				cpuused = utils.Round(cpuused + container.Quota)
			}
			node.CPUUsed = cpuused
			fmt.Println(m.UpdateNode(ctx, node))
		}
	}
}
