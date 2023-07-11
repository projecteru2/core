package types

// CalculateDeployResponse .
type CalculateDeployResponse struct {
	EnginesParams     []EngineParams     `json:"engines_params" mapstructure:"engines_params"`
	WorkloadsResource []WorkloadResource `json:"workloads_resource" mapstructure:"workloads_resource"`
}

// CalculateReallocResponse .
type CalculateReallocResponse struct {
	EngineParams     EngineParams     `json:"engine_params" mapstructure:"engine_params"`
	DeltaResource    WorkloadResource `json:"delta_resource" mapstructure:"delta_resource"`
	WorkloadResource WorkloadResource `json:"workload_resource" mapstructure:"workload_resource"`
}

// CalculateRemapResponse .
type CalculateRemapResponse struct {
	EngineParamsMap map[string]EngineParams `json:"engine_params_map" mapstructure:"engine_params_map"`
}
