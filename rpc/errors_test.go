package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
)

func TestStatusCodesAreUnique(t *testing.T) {
	seen := map[codes.Code]string{}
	for name, code := range allStatusCodes() {
		if other, dup := seen[code]; dup {
			t.Errorf("code %d shared by %s and %s", code, other, name)
		}
		seen[code] = name
	}
	assert.Len(t, seen, len(allStatusCodes()))
}

func allStatusCodes() map[string]codes.Code {
	return map[string]codes.Code{
		"WatchServiceStatus": WatchServiceStatus,
		"ListNetworks":       ListNetworks,
		"ConnectNetwork":     ConnectNetwork,
		"DisconnectNetwork":  DisconnectNetwork,
		"AddPod":             AddPod,
		"RemovePod":          RemovePod,
		"GetPod":             GetPod,
		"ListPods":           ListPods,
		"PodResource":        PodResource,
		"AddNode":            AddNode,
		"RemoveNode":         RemoveNode,
		"ListPodNodes":       ListPodNodes,
		"GetNode":            GetNode,
		"SetNode":            SetNode,
		"SetNodeStatus":      SetNodeStatus,
		"GetNodeResource":    GetNodeResource,
		"GetNodeStatus":      GetNodeStatus,
		"GetNodeEngine":      GetNodeEngine,
		"CalculateCapacity":  CalculateCapacity,
		"GetWorkload":        GetWorkload,
		"GetWorkloads":       GetWorkloads,
		"ListWorkloads":      ListWorkloads,
		"ListNodeWorkloads":  ListNodeWorkloads,
		"GetWorkloadsStatus": GetWorkloadsStatus,
		"SetWorkloadsStatus": SetWorkloadsStatus,
		"RawEngineStatus":    RawEngineStatus,
		"Copy":               Copy,
		"Send":               Send,
		"SendLargeFile":      SendLargeFile,
		"BuildImage":         BuildImage,
		"CacheImage":         CacheImage,
		"RemoveImage":        RemoveImage,
		"CreateWorkload":     CreateWorkload,
		"ReplaceWorkload":    ReplaceWorkload,
		"RemoveWorkload":     RemoveWorkload,
		"DissociateWorkload": DissociateWorkload,
		"ControlWorkload":    ControlWorkload,
		"ExecuteWorkload":    ExecuteWorkload,
		"ReallocResource":    ReallocResource,
		"LogStream":          LogStream,
		"RunAndWait":         RunAndWait,
		"ListImage":          ListImage,
	}
}
