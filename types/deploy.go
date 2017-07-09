package types

type DeployStatus struct {
	Name string
	// 可以部署几个
	Capacity int
	// 上面有几个了
	Count int
	// 最终部署几个
	Deploy int
	// 其他需要 filter 的字段
}
