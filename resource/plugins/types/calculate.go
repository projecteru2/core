package types

type CalculateDeployResponse struct {
	EnginesParams     []EngineParams     `json:"engines_params"`
	WorkloadsResource []WorkloadResource `json:"workloads_resource"`
}

type CalculateReallocResponse struct {
	EngineParams     EngineParams     `json:"engine_params"`
	DeltaResource    WorkloadResource `json:"delta_resource"`
	WorkloadResource WorkloadResource `json:"workload_resource"`
}

type CalculateRemapResponse struct {
	EngineParamsMap map[string]EngineParams `json:"engine_params_map"`
}
