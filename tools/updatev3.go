package main

import (
	"context"
	"fmt"
	"os"

	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func main() {
	if len(os.Args) != 2 {
		panic("wrong args")
	}
	config, _ := utils.LoadConfig(os.Args[1])
	store, _ := etcdv3.New(config, false)
	ctx := context.Background()
	cs, err := store.ListContainers(ctx, "", "", "", 0)
	if err != nil {
		panic(err)
	}
	for _, c := range cs {
		ci, err := c.Inspect(ctx)
		if err != nil {
			panic(err)
		}
		c.Meta = types.Meta{
			ID:       c.ID,
			Labels:   ci.Labels,
			Networks: ci.Networks,
			Running:  ci.Running,
		}
		c.User = ci.User
		c.Image = ci.Image
		c.Env = ci.Env
		fmt.Println(c.ID, c.Networks)
		if err := store.UpdateContainer(ctx, c); err != nil {
			panic(err)
		}
	}
}
