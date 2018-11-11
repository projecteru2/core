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
	data := string(bytes)

	// store container info
	key := fmt.Sprintf(containerInfoKey, container.ID)
	_, err = m.Create(ctx, key, data)
	if err != nil {
		return err
	}

	// store container finished so clean if err is not nil
	defer func() {
		if err != nil {
			m.CleanContainerData(context.Background(), container.ID, appname, entrypoint, container.Nodename)
		}
	}()

	// store deploy status
	key = filepath.Join(containerDeployPrefix, appname, entrypoint, container.Nodename, container.ID)
	_, err = m.Create(ctx, key, "")
	if err != nil {
		return err
	}

	// store node-container data
	key = fmt.Sprintf(nodeContainersKey, container.Nodename, container.ID)
	_, err = m.Create(ctx, key, data)
	return err
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
	key := fmt.Sprintf(containerInfoKey, ID)
	if _, err := m.Delete(ctx, key); err != nil {
		return err
	}

	// remove deploy status by core
	key = filepath.Join(containerDeployPrefix, appname, entrypoint, nodename, ID)
	if _, err := m.Delete(ctx, key); err != nil {
		return err
	}

	// remove node-containers data
	key = fmt.Sprintf(nodeContainersKey, nodename, ID)
	if _, err := m.Delete(ctx, key); err != nil {
		return err
	}
	return nil
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

	if c, err = m.bindNodeEngine(ctx, c); err != nil {
		return nil, err
	}

	return c, nil
}

// GetContainers get many containers
// TODO merge etcd ops
func (m *Mercury) GetContainers(ctx context.Context, IDs []string) (containers []*types.Container, err error) {
	for _, ID := range IDs {
		container, err := m.GetContainer(ctx, ID)
		if err != nil {
			return containers, err
		}
		containers = append(containers, container)
	}
	return containers, err
}

// ContainerDeployed store deployed container info
func (m *Mercury) ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename, data string) error {
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename, ID)
	//Only update when it exist
	_, err := m.Update(ctx, key, data)
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
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename)
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

		if c, err = m.bindNodeEngine(ctx, c); err != nil {
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
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename)
	ch := make(chan *types.DeployStatus)
	go func() {
		defer close(ch)
		for resp := range m.Watch(ctx, key, clientv3.WithPrefix()) {
			log.Debugf("[WatchDeployStatus] recv %d events", len(resp.Events))
			msg := &types.DeployStatus{}
			if resp.Err() != nil {
				if resp.Err() != context.Canceled {
					msg.Err = resp.Err()
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
				ch <- msg
			}
		}
	}()
	return ch
}

func (m *Mercury) bindNodeEngine(ctx context.Context, container *types.Container) (*types.Container, error) {
	node, err := m.GetNode(ctx, container.Podname, container.Nodename)
	if err != nil {
		return nil, err
	}

	container.Engine = node.Engine
	container.Node = node
	return container, nil
}
