package calcium

import (
	"context"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"gitlab.ricebook.net/platform/core/types"
)

const maxPuller = 10

// 在podname上cache这个image
// 实际上就是在所有的node上去pull一次
func (c *calcium) cacheImage(podname, image string) error {
	nodes, err := c.ListPodNodes(podname, false)
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
			pullImage(node, image)
		}(node)
	}

	return nil
}

// 清理一个pod上的全部这个image
// 对里面所有node去执行
func (c *calcium) cleanImage(podname, image string) error {
	nodes, err := c.ListPodNodes(podname, false)
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
			if err := cleanImageOnNode(node, image, c.config.ImageCache); err != nil {
				log.Errorf("cleanImageOnNode error: %s", err)
			}
		}(node)
	}

	return nil
}

// 只要一个image的前面, tag不要
func normalizeImage(image string) string {
	if strings.Contains(image, ":") {
		t := strings.Split(image, ":")
		return t[0]
	}
	return image
}

type imageList []enginetypes.ImageSummary

func (x imageList) Len() int           { return len(x) }
func (x imageList) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x imageList) Less(i, j int) bool { return x[i].Created > x[j].Created }

// 清理一个node上的这个image
// 只清理同名字不同tag的
// 并且保留最新的两个
func cleanImageOnNode(node *types.Node, image string, count int) error {
	log.Debugf("[cleanImageOnNode] node: %s, image: %s", node.Name, strings.Split(image, ":")[0])
	imgListFilter := filters.NewArgs()
	image = normalizeImage(image)
	imgListFilter.Add("reference", image) // 相同repo的image
	images, err := node.Engine.ImageList(context.Background(), enginetypes.ImageListOptions{Filters: imgListFilter})
	if err != nil {
		return err
	}

	if len(images) < count {
		return nil
	}

	images = images[count:]
	log.Debugf("Delete Images: %v", images)

	for _, image := range images {
		_, err := node.Engine.ImageRemove(context.Background(), image.ID, enginetypes.ImageRemoveOptions{
			Force:         false,
			PruneChildren: true,
		})
		if err != nil {
			log.Errorf("[cleanImageOnNode] Node %s ImageRemove error: %s, imageID: %s", node.Name, err, image.ID)
		}
	}
	return nil
}
