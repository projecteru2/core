package calcium

import (
	"sort"
	"strings"

	enginetypes "github.com/docker/docker/api/types"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

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

	for _, node := range nodes {
		// 这个函数在 create_container.go 里面
		// 同步地pull image
		pullImage(node, image)
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

	for _, node := range nodes {
		cleanImageOnNode(node, image)
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
func cleanImageOnNode(node *types.Node, image string) error {
	images, err := node.Engine.ImageList(context.Background(), enginetypes.ImageListOptions{})
	if err != nil {
		return err
	}

	image = normalizeImage(image)

	toDelete := imageList{}
	for _, img := range images {
		// 没有名字的不管
		if len(img.RepoTags) == 0 {
			continue
		}
		tag := img.RepoTags[0]
		// 别人的镜像不管
		if !strings.HasPrefix(tag, image) {
			continue
		}
		toDelete = append(toDelete, img)
	}
	sort.Sort(toDelete)

	for index, img := range toDelete {
		// 如果是最新的两个(index是0或者1), 就忽略
		if index <= 1 {
			continue
		}
		// 其他都无情删掉
		// 可能因为有容器的存在而失败, 就放他苟且吧
		node.Engine.ImageRemove(context.Background(), img.RepoTags[0], enginetypes.ImageRemoveOptions{
			Force:         false,
			PruneChildren: true,
		})
	}
	return nil
}
