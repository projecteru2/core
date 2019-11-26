package etcdv3

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"context"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// AddContainer add a container
// mainly record its relationship on pod and node
// actually if we already know its node, we will know its pod
// but we still store it
// storage path in etcd is `/container/:containerid`
func (m *Mercury) AddContainer(ctx context.Context, container *types.Container) error {
	return m.doOpsContainer(ctx, container, true)
}

// UpdateContainer update a container
func (m *Mercury) UpdateContainer(ctx context.Context, container *types.Container) error {
	return m.doOpsContainer(ctx, container, false)
}

// RemoveContainer remove a container
// container id must be in full length
func (m *Mercury) RemoveContainer(ctx context.Context, container *types.Container) error {
	if l := len(container.ID); l != 64 {
		return types.NewDetailedErr(types.ErrBadContainerID,
			fmt.Sprintf("containerID: %s, length: %d",
				container.ID, len(container.ID)))
	}
	appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
	if err != nil {
		return err
	}

	return m.cleanContainerData(ctx, container.ID, appname, entrypoint, container.Nodename)
}

// GetContainer get a container
// container if must be in full length, or we can't find it in etcd
// storage path in etcd is `/container/:containerid`
func (m *Mercury) GetContainer(ctx context.Context, ID string) (*types.Container, error) {
	containers, err := m.GetContainers(ctx, []string{ID})
	if err != nil {
		return nil, err
	}
	return containers[0], nil
}

// GetContainers get many containers
func (m *Mercury) GetContainers(ctx context.Context, IDs []string) (containers []*types.Container, err error) {
	keys := []string{}
	for _, ID := range IDs {
		keys = append(keys, fmt.Sprintf(containerInfoKey, ID))
	}

	return m.doGetContainers(ctx, keys)
}

// GetContainerStatus get container status
func (m *Mercury) GetContainerStatus(ctx context.Context, ID string) (types.StatusMeta, error) {
	container, err := m.GetContainer(ctx, ID)
	if err != nil {
		return types.StatusMeta{}, err
	}
	return container.StatusMeta, nil
}

// SetContainerStatus set container status
func (m *Mercury) SetContainerStatus(ctx context.Context, container *types.Container, ttl int64, force bool) error {
	appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
	if err != nil {
		return err
	}
	data, err := json.Marshal(container.StatusMeta)
	if err != nil {
		return err
	}
	deployKey := filepath.Join(containerDeployPrefix, appname, entrypoint, container.Nodename, container.ID)
	opts := []clientv3.OpOption{}
	if ttl > 0 {
		lease, err := m.cliv3.Grant(ctx, ttl)
		if err != nil {
			return err
		}
		opts = append(opts, clientv3.WithLease(lease.ID))
	}
	if !force {
		// Only update when it exist
		_, err = m.Update(ctx, deployKey, string(data), opts...)
	} else {
		_, err = m.Create(ctx, deployKey, string(data), opts...)
	}
	return err
}

// ListContainers list containers
func (m *Mercury) ListContainers(ctx context.Context, appname, entrypoint, nodename string, limit int64) ([]*types.Container, error) {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// 这里显式加个 / 来保证 prefix 是唯一的
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename) + "/"
	resp, err := m.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithKeysOnly(), clientv3.WithLimit(limit))
	if err != nil {
		return []*types.Container{}, err
	}

	keys := []string{}
	for _, ev := range resp.Kvs {
		containerID := utils.Tail(string(ev.Key))
		keys = append(keys, fmt.Sprintf(containerInfoKey, containerID))
	}

	return m.doGetContainers(ctx, keys)
}

// ListNodeContainers list containers belong to one node
func (m *Mercury) ListNodeContainers(ctx context.Context, nodename string) ([]*types.Container, error) {
	key := fmt.Sprintf(nodeContainersKey, nodename, "")
	resp, err := m.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return []*types.Container{}, err
	}

	containers := []*types.Container{}
	for _, ev := range resp.Kvs {
		container := &types.Container{}
		if err := json.Unmarshal(ev.Value, container); err != nil {
			return []*types.Container{}, err
		}
		containers = append(containers, container)
	}

	return m.bindContainersAdditions(ctx, containers)
}

// ContainerStatusStream watch deployed status
func (m *Mercury) ContainerStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.ContainerStatus {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// 显式加个 / 保证 prefix 唯一
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename) + "/"
	ch := make(chan *types.ContainerStatus)
	go func() {
		defer close(ch)
		for resp := range m.Watch(ctx, key, clientv3.WithPrefix()) {
			if resp.Err() != nil {
				if !resp.Canceled {
					log.Errorf("[ContainerStatusStream] watch failed %v", resp.Err())
				}
				return
			}
			for _, ev := range resp.Events {
				_, _, _, ID := parseStatusKey(string(ev.Kv.Key))
				msg := &types.ContainerStatus{ID: ID}
				if ev.Type == clientv3.EventTypeDelete {
					msg.Delete = true
				} else if container, err := m.GetContainer(ctx, ID); err != nil {
					msg.Error = err
				} else if utils.FilterContainer(container.Labels, labels) {
					log.Debugf("[ContainerStatusStream] container %s status changed", container.ID)
					msg.Container = container
				} else {
					log.Warnf("[ContainerStatusStream] ignore container %s by labels", ID)
					continue
				}
				ch <- msg
			}
		}
	}()
	return ch
}

func (m *Mercury) cleanContainerData(ctx context.Context, ID, appname, entrypoint, nodename string) error {
	keys := []string{
		filepath.Join(containerDeployPrefix, appname, entrypoint, nodename, ID), // container deploy status
		fmt.Sprintf(containerInfoKey, ID),                                       // container info
		fmt.Sprintf(nodeContainersKey, nodename, ID),                            // node containers
	}
	_, err := m.batchDelete(ctx, keys)
	return err
}

func (m *Mercury) doGetContainers(ctx context.Context, keys []string) (containers []*types.Container, err error) {
	var kvs []*mvccpb.KeyValue
	if kvs, err = m.GetMulti(ctx, keys); err != nil {
		return
	}

	for _, kv := range kvs {
		container := &types.Container{}
		if err = json.Unmarshal(kv.Value, container); err != nil {
			log.Errorf("[doGetContainers] failed to unmarshal %v, err: %v", string(kv.Key), err)
			return
		}
		containers = append(containers, container)
	}

	return m.bindContainersAdditions(ctx, containers)
}

func (m *Mercury) bindContainersAdditions(ctx context.Context, containers []*types.Container) ([]*types.Container, error) {
	podNodes := map[string][]string{}
	isCached := map[string]struct{}{}
	deployKeys := []string{}
	deployStatus := map[string][]byte{}
	for _, container := range containers {
		if _, ok := podNodes[container.Podname]; !ok {
			podNodes[container.Podname] = []string{}
		}
		if _, ok := isCached[container.Nodename]; !ok {
			podNodes[container.Podname] = append(podNodes[container.Podname], container.Nodename)
			isCached[container.Nodename] = struct{}{}
		}
		appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
		if err != nil {
			return nil, err
		}
		deployKeys = append(deployKeys,
			filepath.Join(containerDeployPrefix, appname, entrypoint, container.Nodename, container.ID),
		)
	}

	// deal with container status
	if kvs, err := m.GetMulti(ctx, deployKeys); err == nil {
		for _, kv := range kvs {
			containerID := utils.Tail(string(kv.Key))
			deployStatus[containerID] = kv.Value
		}
	} else {
		log.Warnf("[bindContainersAdditions] get container status failed %v", err)
	}

	nodes, err := m.GetNodes(ctx, podNodes)
	if err != nil {
		return nil, err
	}

	for index, container := range containers {
		if _, ok := nodes[container.Nodename]; !ok {
			return nil, types.ErrBadMeta
		}
		containers[index].Engine = nodes[container.Nodename].Engine
		containers[index].StatusMeta.ID = containers[index].ID
		if _, ok := deployStatus[container.ID]; ok {
			if err := json.Unmarshal(deployStatus[container.ID], &containers[index].StatusMeta); err != nil {
				log.Warnf("[bindContainersAdditions] unmarshal %s status data failed %v", container.ID, err)
				log.Errorf("[bindContainersAdditions] status raw: %s", deployStatus[container.ID])
			}
		}
	}
	return containers, nil
}

func (m *Mercury) doOpsContainer(ctx context.Context, container *types.Container, create bool) error {
	var err error
	appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
	if err != nil {
		return err
	}

	// now everything is ok
	// we use full length id instead
	bytes, err := json.Marshal(container)
	if err != nil {
		return err
	}
	containerData := string(bytes)

	data := map[string]string{
		fmt.Sprintf(containerInfoKey, container.ID):                      containerData,
		fmt.Sprintf(nodeContainersKey, container.Nodename, container.ID): containerData,
	}

	if create {
		b, err := json.Marshal(container.StatusMeta)
		if err != nil {
			return err
		}
		data[filepath.Join(containerDeployPrefix, appname, entrypoint, container.Nodename, container.ID)] = string(b)
		_, err = m.BatchCreate(ctx, data)
	} else {
		_, err = m.BatchUpdate(ctx, data)
	}
	return err
}
