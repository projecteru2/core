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

// ContainerDeployed store deployed container info
func (m *Mercury) ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename string, data []byte) error {
	deployKey := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename, ID)
	containerKey := fmt.Sprintf(containerInfoKey, ID)                // container info
	nodeContainerKey := fmt.Sprintf(nodeContainersKey, nodename, ID) // node containers
	kvs, err := m.GetOne(ctx, containerKey)
	if err != nil {
		return err
	}
	meta := &types.Meta{}
	if err = json.Unmarshal(data, meta); err != nil {
		return err
	}
	container := &types.Container{}
	if err = json.Unmarshal(kvs.Value, container); err != nil {
		return err
	}
	container.Running = meta.Running
	container.Networks = meta.Networks
	b, err := json.Marshal(container)
	if err != nil {
		return err
	}
	containerData := string(b)
	//Only update when it exist
	_, err = m.BatchUpdate(
		ctx,
		map[string]string{
			deployKey:        string(data),
			containerKey:     containerData,
			nodeContainerKey: containerData,
		},
	)
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

// WatchDeployStatus watch deployed status
func (m *Mercury) WatchDeployStatus(ctx context.Context, appname, entrypoint, nodename string) chan *types.DeployStatus {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// 显式加个 / 保证 prefix 唯一
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename) + "/"
	ch := make(chan *types.DeployStatus)
	go func() {
		defer close(ch)
		for resp := range m.Watch(ctx, key, clientv3.WithPrefix()) {
			msg := &types.DeployStatus{}
			if resp.Err() != nil {
				if !resp.Canceled {
					msg.Error = resp.Err()
					ch <- msg
				}
				return
			}
			for _, ev := range resp.Events {
				appname, entrypoint, nodename, ID := parseStatusKey(string(ev.Kv.Key))
				msg.ID = ID
				msg.Appname = appname
				msg.Entrypoint = entrypoint
				msg.Nodename = nodename
				msg.Data = string(ev.Kv.Value)
				msg.Action = ev.Type.String()
				log.Debugf("[WatchDeployStatus] app %s_%s event, id %s, action %s", appname, entrypoint, utils.ShortID(msg.ID), msg.Action)
				if msg.Data != "" {
					log.Debugf("[WatchDeployStatus] data %s", msg.Data)
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
			log.Errorf("[doGetContainers] failed to unmarshal %v, err: %v", kv.Key, err)
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
	kvs, err := m.GetMulti(ctx, deployKeys)
	if err != nil {
		return nil, err
	}
	for _, kv := range kvs {
		containerID := utils.Tail(string(kv.Key))
		deployStatus[containerID] = kv.Value
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
		if _, ok := deployStatus[container.ID]; !ok {
			return nil, types.ErrBadMeta
		}
		if len(deployStatus[container.ID]) > 0 {
			if err := json.Unmarshal(deployStatus[container.ID], &containers[index].Meta); err != nil {
				log.Warnf("[bindContainersAdditions] unmarshal %s status data failed %v", container.ID, err)
				log.Errorf("%s", deployStatus[container.ID])
			}
		} else {
			log.Warnf("[bindContainersAdditions] %s no deploy status", container.ID)
		}
		containers[index].StatusData = deployStatus[container.ID]
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
		data[filepath.Join(containerDeployPrefix, appname, entrypoint, container.Nodename, container.ID)] = ""
		_, err = m.BatchCreate(ctx, data)
	} else {
		_, err = m.BatchUpdate(ctx, data)
	}
	return err
}
