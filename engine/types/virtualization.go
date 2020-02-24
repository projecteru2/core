package types

// VirtualizationResource define resources
type VirtualizationResource struct {
	CPU           map[string]int64            // 通过 StartPre 设置 cgroup cpuset.cpus
	Quota         float64                     // systemd 能够设置百分比, 自己通过总核数换算
	Memory        int64                       // StartPre 设置 cgroup memory.limit_in_bytes
	Storage       int64                       // 不支持
	SoftLimit     bool                        // 不支持... 其实可以有, 不过只是对 memory
	NUMANode      string                      // 不支持, 其实也可以有, 以后再说
	Volumes       []string                    // 不支持, 其实也可以有, 再说
	VolumePlan    map[string]map[string]int64 // 不支持
	VolumeChanged bool                        // 不支持
}

// VirtualizationCreateOptions use for create virtualization target
type VirtualizationCreateOptions struct {
	VirtualizationResource
	Seq        int               // for count
	Name       string            // 会作为 service name 的一部分
	User       string            // 不支持了, 先都 root 吧
	Image      string            // 不支持
	WorkingDir string            // 没问题, 配置一下
	Stdin      bool              // 这个只有在 lambda 交互式的时候才有用, 技术上可以做
	Privileged bool              // 不管了, 都是 root
	Cmd        []string          // 没问题
	Env        []string          // 没问题
	DNS        []string          // 不支持, 都没有网络 namespace..
	Hosts      []string          // 不支持, 都没有网络 namespace..
	Publish    []string          // 不支持, 都没有网络 namespace..
	Sysctl     map[string]string // 不支持... 其实可以支持
	Labels     map[string]string // 支持, 写在 service 的 description 里吧, json 格式

	Debug bool // 呃... 不支持

	RestartPolicy string // 支持, systemd 可以指定多种策略

	Network  string            // 先不支持了, 其实可以支持
	Networks map[string]string // 不支持

	Volumes []string // 暂不

	LogType   string            // 支持, 就写 journal 吧默认
	LogConfig map[string]string // 不...

	RawArgs []byte // 无视
	Lambda  bool   // 暂不
}

// VirtualizationCreated use for store name and ID
type VirtualizationCreated struct {
	ID   string // id 不能用 pid 啦, 会变的, 记录 service 的名字吧, 会用上面的Name再拼上一段随机字符
	Name string // 就是用户传来的 Name
}

// VirtualizationInfo store virtualization info
type VirtualizationInfo struct {
	ID       string
	User     string
	Image    string
	Running  bool
	Env      []string
	Labels   map[string]string
	Networks map[string]string
	// TODO other infomation like cpu memory
}

// VirtualizationWaitResult store exit result
type VirtualizationWaitResult struct {
	Message string
	Code    int64
}
