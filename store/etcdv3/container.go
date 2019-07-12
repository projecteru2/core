package etcdv3

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"context"

	"github.com/coreos/etcd/clientv3"
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

	return m.CleanContainerData(ctx, container.ID, appname, entrypoint, container.Nodename)
}

// CleanContainerData clean container data
func (m *Mercury) CleanContainerData(ctx context.Context, ID, appname, entrypoint, nodename string) error {
	keys := []string{
		fmt.Sprintf(containerInfoKey, ID),
		filepath.Join(containerDeployPrefix, appname, entrypoint, nodename, ID),
		fmt.Sprintf(nodeContainersKey, nodename, ID),
	}
	_, err := m.BatchDelete(ctx, keys)
	return err
}

// GetContainer get a container
// container if must be in full length, or we can't find it in etcd
// storage path in etcd is `/container/:containerid`
func (m *Mercury) GetContainer(ctx context.Context, ID string) (*types.Container, error) {
	if l := len(ID); l != 64 {
		return nil, types.NewDetailedErr(types.ErrBadContainerID,
			fmt.Sprintf("containerID: %s, length: %d", ID, l))
	}

	key := fmt.Sprintf(containerInfoKey, ID)
	ev, err := m.GetOne(ctx, key)
	if err != nil {
		return nil, err
	}

	c := &types.Container{}
	if err := json.Unmarshal(ev.Value, c); err != nil {
		return nil, err
	}

	if c, err = m.bindContainerAdditions(ctx, c); err != nil {
		return nil, err
	}

	return c, nil
}

// GetContainers get many containers
func (m *Mercury) GetContainers(ctx context.Context, IDs []string) (containers []*types.Container, err error) {
	keys := []string{}
	for _, ID := range IDs {
		keys = append(keys, fmt.Sprintf(containerInfoKey, ID))
	}
	txnResponse := &clientv3.TxnResponse{}
	if txnResponse, err = m.BatchGet(ctx, keys); err != nil {
		return
	}

	for idx, responseOp := range txnResponse.Responses {
		kvs := responseOp.GetResponseRange().Kvs
		if len(kvs) == 0 {
			return containers, types.ErrKeyNotExists
		}

		container := &types.Container{}
		if err = json.Unmarshal(kvs[0].Value, container); err != nil {
			log.Errorf("[Mercury] failed to unmarshal json to container: %s", IDs[idx])
			return
		}
		containers = append(containers, container)
	}
	return
}

// ContainerDeployed store deployed container info
func (m *Mercury) ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename string, data []byte, ttl int64) error {
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename, ID)
	opts := []clientv3.OpOption{}
	if ttl > 0 {
		lease, err := m.cliv3.Grant(ctx, ttl)
		if err != nil {
			return err
		}
		opts = append(opts, clientv3.WithLease(lease.ID))
	}
	//Only update when it exist
	_, err := m.Update(ctx, key, string(data), opts...)
	return err
}

// ListContainers list containers
func (m *Mercury) ListContainers(ctx context.Context, appname, entrypoint, nodename string) ([]*types.Container, error) {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// 这里显式加个 / 来保证 prefix 是唯一的
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename) + "/"
	resp, err := m.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return []*types.Container{}, err
	}

	IDs := []string{}
	for _, ev := range resp.Kvs {
		containerID := utils.Tail(string(ev.Key))
		IDs = append(IDs, containerID)
	}
	return m.GetContainers(ctx, IDs)
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
		c := &types.Container{}
		if err := json.Unmarshal(ev.Value, c); err != nil {
			return []*types.Container{}, err
		}

		if c, err = m.bindContainerAdditions(ctx, c); err != nil {
			return []*types.Container{}, err
		}

		containers = append(containers, c)
	}
	return containers, nil
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
				appname, entrypoint, nodename, id := parseStatusKey(string(ev.Kv.Key))
				msg.Data = string(ev.Kv.Value)
				msg.Action = ev.Type.String()
				msg.Appname = appname
				msg.Entrypoint = entrypoint
				msg.Nodename = nodename
				msg.ID = id
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

func (m *Mercury) bindContainerAdditions(ctx context.Context, container *types.Container) (*types.Container, error) {
	node, err := m.GetNode(ctx, container.Podname, container.Nodename)
	if err != nil {
		return nil, err
	}

	appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
	if err != nil {
		return nil, err
	}

	key := filepath.Join(containerDeployPrefix, appname, entrypoint, node.Name, container.ID)
	ev, err := m.GetOne(ctx, key)
	if err != nil {
		return nil, err
	}

	container.StatusData = ev.Value
	container.Engine = node.Engine
	return container, nil
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
