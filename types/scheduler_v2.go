package types

type SchedulerV2 func([]NodeInfo) (ResourcePlans, int, error)

type ResourceRequest interface {
	Type() ResourceType
	MakeScheduler() SchedulerV2
}

type ResourcePlans interface {
	Type() ResourceType
	ApplyChangesOnNode(NodeInfo, *Node) error
}
