package types

import resourcetypes "github.com/projecteru2/core/resource/types"

// WorkloadResourceRequest carries the request params, keepbind included.
type WorkloadResourceRequest = resourcetypes.RawParams

// WorkloadResource carries the allocated params, keepbind excluded.
type WorkloadResource = resourcetypes.RawParams
