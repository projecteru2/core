package common

import (
	"fmt"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/yaml.v2"
)

// app.yaml读进来之后就变这个了
// Entrypoints的key是entrypoint的名字
// Binds的key是HostPath, 宿主机对应的路径
type AppSpecs struct {
	Appname     string                `yaml:"appname,omitempty"`
	Entrypoints map[string]Entrypoint `yaml:"entrypoints,omitempty,flow"`
	Build       []string              `yaml:"build,omitempty,flow"`
	Volumes     []string              `yaml:"volumes,omitempty,flow"`
	Binds       map[string]Bind       `yaml:"binds,omitempty,flow"`
	Meta        map[string]string     `yaml:"meta,omitempty,flow"`
	Base        string                `yaml:"base"`
}

// 单个entrypoint
// Ports是端口列表, 形如5000/tcp, 5001/udp
// Exposes是对外暴露端口列表, 形如22:5000, 前面是容器内端口, 后面是宿主机端口
// Hosts是容器内会加入的/etc/hosts列表
type Entrypoint struct {
	Command       string        `yaml:"cmd,omitempty"`
	Ports         []docker.Port `yaml:"ports,omitempty,flow"`
	Exposes       []Expose      `yaml:"exposes,omitempty,flow"`
	NetworkMode   string        `yaml:"network_mode,omitempty"`
	MemoryLimit   int           `yaml:"mem_limit,omitempty"`
	RestartPolicy string        `yaml:"restart,omitempty"`
	HealthCheck   string        `yaml:"health_check,omitempty"`
	ExtraHosts    []string      `yaml:"hosts,omitempty,flow"`
	PermDir       bool          `yaml:"permdir,omitempty"`
	Privileged    string        `yaml:"privileged,omitempty"`
}

// 单个bind
// Binds对应的内容, 容器里的路径, 以及是否只读
type Bind struct {
	InContainerPath string `yaml:"bind,omitempty"`
	ReadOnly        bool   `yaml:"ro,omitempty"`
}

type Expose string

// suppose expose is like 80/tcp:46656/tcp
func (e Expose) ContainerPort() docker.Port {
	ports := strings.Split(string(e), ":")
	return docker.Port(ports[0])
}

func (e Expose) HostPort() docker.Port {
	ports := strings.Split(string(e), ":")
	return docker.Port(ports[1])
}

// 从content加载一个AppSpecs对象出来
// content, app.yaml的内容
// check, 是否要检查配置, 如果直接从etcd读不需要检查
func LoadAppSpecs(content string, check bool) (AppSpecs, error) {
	specs := AppSpecs{}
	err := yaml.Unmarshal([]byte(content), &specs)
	if err != nil {
		return specs, err
	}

	if check {
		err = verify(specs)
		if err != nil {
			return specs, err
		}
	}
	return specs, nil
}

// 基本的检查, 检查失败就挂了
// 一些对ports, exposes啥的检查还没做
func verify(a AppSpecs) error {
	if a.Appname == "" {
		return fmt.Errorf("No appname specified")
	}
	if len(a.Entrypoints) == 0 {
		return fmt.Errorf("No entrypoints specified")
	}
	if len(a.Build) == 0 {
		return fmt.Errorf("No build commands specified")
	}

	for name, _ := range a.Entrypoints {
		if strings.Contains(name, "_") {
			return fmt.Errorf("Sorry but we do not support `_` in entrypoint 눈_눈")
		}
	}
	return nil
}
