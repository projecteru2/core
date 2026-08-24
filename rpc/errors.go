package rpc

import "google.golang.org/grpc/codes"

const (
	WatchServiceStatus codes.Code = 1001

	ListNetworks      codes.Code = 1011
	ConnectNetwork    codes.Code = 1012
	DisconnectNetwork codes.Code = 1013

	AddPod      codes.Code = 1021
	RemovePod   codes.Code = 1022
	GetPod      codes.Code = 1023
	ListPods    codes.Code = 1024
	PodResource codes.Code = 1025

	AddNode         codes.Code = 1031
	RemoveNode      codes.Code = 1032
	ListPodNodes    codes.Code = 1033
	GetNode         codes.Code = 1034
	SetNode         codes.Code = 1035
	SetNodeStatus   codes.Code = 1036
	GetNodeStatus   codes.Code = 1038
	GetNodeResource codes.Code = 1037
	GetNodeEngine   codes.Code = 1038

	CalculateCapacity codes.Code = 1041

	GetWorkload        codes.Code = 1051
	GetWorkloads       codes.Code = 1052
	ListWorkloads      codes.Code = 1053
	ListNodeWorkloads  codes.Code = 1054
	GetWorkloadsStatus codes.Code = 1055
	SetWorkloadsStatus codes.Code = 1056
	RawEngineStatus    codes.Code = 1057

	Copy          codes.Code = 1061
	Send          codes.Code = 1062
	SendLargeFile codes.Code = 1063

	BuildImage  codes.Code = 1071
	CacheImage  codes.Code = 1072
	RemoveImage codes.Code = 1073

	CreateWorkload     codes.Code = 1074
	ReplaceWorkload    codes.Code = 1075
	RemoveWorkload     codes.Code = 1076
	DissociateWorkload codes.Code = 1077
	ControlWorkload    codes.Code = 1078
	ExecuteWorkload    codes.Code = 1079
	ReallocResource    codes.Code = 10710
	LogStream          codes.Code = 10711
	RunAndWait         codes.Code = 10712
	ListImage          codes.Code = 10713
)
