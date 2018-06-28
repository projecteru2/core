package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// 在podname上cache这个image
// 实际上就是在所有的node上去pull一次
func (c *Calcium) cacheImage(ctx context.Context, podname, image string) error {
	nodes, err := c.ListPodNodes(ctx, podname, false)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	wg := sync.WaitGroup{}
	defer wg.Wait()
	for i, node := range nodes {
		// 这个函数在 create_container.go 里面
		// 同步地pull image
		if (i != 0) && (i%maxPuller == 0) {
			wg.Wait()
		}
		wg.Add(1)
		go func(node *types.Node) {
			defer wg.Done()
			auth, err := makeEncodedAuthConfigFromRemote(c.config.Docker.AuthConfigs, image)
			if err != nil {
				log.Errorf("[cacheImage] Cache image failed %v", err)
				return
			}
			pullImage(ctx, node, image, auth)
		}(node)
	}

	return nil
}

// 清理一个pod上的全部这个image
// 对里面所有node去执行
func (c *Calcium) cleanImage(ctx context.Context, podname, image string) error {
	nodes, err := c.ListPodNodes(ctx, podname, false)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	wg := sync.WaitGroup{}
	defer wg.Wait()
	for i, node := range nodes {
		if (i != 0) && (i%maxPuller == 0) {
			wg.Wait()
		}
		wg.Add(1)
		go func(node *types.Node) {
			defer wg.Done()
			if err := cleanImageOnNode(ctx, node, image, c.config.ImageCache); err != nil {
				log.Errorf("[cleanImage] CleanImageOnNode error: %s", err)
			}
		}(node)
	}
	return nil
}
