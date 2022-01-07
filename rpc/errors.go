package rpc

import "google.golang.org/grpc/codes"

const (
	// WatchServiceStatus .
	WatchServiceStatus codes.Code = 1001

	// ListNetworks .
	ListNetworks codes.Code = 1011
	// ConnectNetwork .
	ConnectNetwork codes.Code = 1012
	// DisconnectNetwork .
	DisconnectNetwork codes.Code = 1013

	// AddPod .
	AddPod codes.Code = 1021
	// RemovePod .
	RemovePod codes.Code = 1022
	// GetPod .
	GetPod codes.Code = 1023
	// ListPods .
	ListPods codes.Code = 1024
	// PodResource .
	PodResource codes.Code = 1025

	// AddNode .
	AddNode codes.Code = 1031
	// RemoveNode .
	RemoveNode codes.Code = 1032
	// ListPodNodes .
	ListPodNodes codes.Code = 1033
	// GetNode .
	GetNode codes.Code = 1034
	// SetNode .
	SetNode codes.Code = 1035
	// SetNodeStatus .
	SetNodeStatus codes.Code = 1036
	// GetNodeStatus .
	GetNodeStatus codes.Code = 1038
	// GetNodeResource .
	GetNodeResource codes.Code = 1037

	// CalculateCapacity .
	CalculateCapacity codes.Code = 1041

	// GetWorkload .
	GetWorkload codes.Code = 1051
	// GetWorkloads .
	GetWorkloads codes.Code = 1052
	// ListWorkloads .
	ListWorkloads codes.Code = 1053
	// ListNodeWorkloads .
	ListNodeWorkloads codes.Code = 1054
	// GetWorkloadsStatus .
	GetWorkloadsStatus codes.Code = 1055
	// SetWorkloadsStatus .
	SetWorkloadsStatus codes.Code = 1056

	// Copy .
	Copy codes.Code = 1061
	// Send .
	Send codes.Code = 1062
	// SendLargeFile .
	SendLargeFile codes.Code = 1063

	// BuildImage .
	BuildImage codes.Code = 1071
	// CacheImage .
	CacheImage codes.Code = 1072
	// RemoveImage .
	RemoveImage codes.Code = 1073

	// CreateWorkload .
	CreateWorkload codes.Code = 1074
	// ReplaceWorkload .
	ReplaceWorkload codes.Code = 1075
	// RemoveWorkload .
	RemoveWorkload codes.Code = 1076
	// DissociateWorkload .
	DissociateWorkload codes.Code = 1077
	// ControlWorkload .
	ControlWorkload codes.Code = 1078
	// ExecuteWorkload .
	ExecuteWorkload codes.Code = 1079
	// ReallocResource .
	ReallocResource codes.Code = 10710
	// LogStream .
	LogStream codes.Code = 10711
	// RunAndWait .
	RunAndWait codes.Code = 10712
	// ListImage .
	ListImage codes.Code = 10713
)
