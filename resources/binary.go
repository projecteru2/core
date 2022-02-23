package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"reflect"
	"strings"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
)

// BinaryPlugin .
type BinaryPlugin struct {
	path   string
	config coretypes.Config
}

func (bp *BinaryPlugin) getArgs(req interface{}) []string {
	t := reflect.TypeOf(req)
	if t.Kind() != reflect.Struct {
		return nil
	}
	v := reflect.ValueOf(req)
	args := []string{}

	for i := 0; i < t.NumField(); i++ {
		fieldType := t.Field(i).Type
		fieldValue := v.Field(i).Interface()
		jsonTag := t.Field(i).Tag.Get("json")

		switch {
		case fieldType.Kind() == reflect.Map:
			if v.Field(i).IsZero() {
				break
			}
			body, err := json.Marshal(fieldValue)
			if err != nil {
				break
			}
			args = append(args, "--"+jsonTag, string(body))
		case fieldType.Kind() == reflect.Slice:
			for j := 0; j < v.Field(i).Len(); j++ {
				if v.Field(i).Index(j).Kind() == reflect.Map {
					body, err := json.Marshal(v.Field(i).Index(j).Interface())
					if err != nil {
						break
					}
					args = append(args, "--"+jsonTag, string(body))
				} else {
					args = append(args, "--"+jsonTag, fmt.Sprintf("%v", v.Field(i).Index(j).Interface()))
				}
			}
		case fieldType.Kind() == reflect.Bool:
			if fieldValue.(bool) {
				args = append(args, "--"+jsonTag)
			}
		default:
			args = append(args, "--"+jsonTag, fmt.Sprintf("%v", fieldValue))
		}
	}
	return args
}

func (bp *BinaryPlugin) execCommand(cmd *exec.Cmd) (output, log string, err error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	output = stdout.String()
	log = stderr.String()
	if err != nil {
		err = fmt.Errorf("err: %v, output: %v, log: %v", err, output, log)
	}
	return output, log, err
}

// calls the plugin and gets json response
func (bp *BinaryPlugin) call(ctx context.Context, cmd string, req interface{}, resp interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, bp.config.ResourcePluginsTimeout)
	defer cancel()

	args := bp.getArgs(req)
	args = append([]string{cmd}, args...)
	command := exec.CommandContext(ctx, bp.path, args...)
	command.Dir = bp.config.ResourcePluginsDir
	log.Infof(ctx, "[callBinaryPlugin] command: %s %s", bp.path, strings.Join(args, " "))
	pluginOutput, pluginLog, err := bp.execCommand(command)

	defer log.Infof(ctx, "[callBinaryPlugin] log from plugin %s: %s", bp.path, pluginLog)
	defer log.Infof(ctx, "[callBinaryPlugin] output from plugin %s: %s", bp.path, pluginOutput)

	if err != nil {
		log.Errorf(ctx, "[callBinaryPlugin] failed to run plugin %s, command %v, err %s", bp.path, args, err)
		return err
	}

	if len(pluginOutput) == 0 {
		pluginOutput = "{}"
	}
	if err := json.Unmarshal([]byte(pluginOutput), resp); err != nil {
		log.Errorf(ctx, "[callBinaryPlugin] failed to unmarshal output of plugin %s, command %v, output %s, err %s", bp.path, args, pluginOutput, err)
		return err
	}
	return nil
}

// GetNodesDeployCapacity .
func (bp *BinaryPlugin) GetNodesDeployCapacity(ctx context.Context, nodes []string, resourceOpts coretypes.WorkloadResourceOpts) (resp *GetNodesDeployCapacityResponse, err error) {
	req := GetNodesDeployCapacityRequest{
		NodeNames:    nodes,
		ResourceOpts: resourceOpts,
	}
	resp = &GetNodesDeployCapacityResponse{}
	err = bp.call(ctx, getNodesCapacityCommand, req, resp)
	return resp, err
}

func (bp *BinaryPlugin) getNodeResourceInfo(ctx context.Context, nodeName string, workloads []*coretypes.Workload, fix bool) (resp *GetNodeResourceInfoResponse, err error) {
	workloadMap := map[string]coretypes.WorkloadResourceArgs{}
	for _, workload := range workloads {
		workloadMap[workload.ID] = workload.ResourceArgs[bp.Name()]
	}

	req := GetNodeResourceInfoRequest{
		NodeName:    nodeName,
		WorkloadMap: workloadMap,
		Fix:         fix,
	}
	resp = &GetNodeResourceInfoResponse{}
	if err = bp.call(ctx, getNodeResourceInfoCommand, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetNodeResourceInfo .
func (bp *BinaryPlugin) GetNodeResourceInfo(ctx context.Context, nodeName string, workloads []*coretypes.Workload) (resp *GetNodeResourceInfoResponse, err error) {
	return bp.getNodeResourceInfo(ctx, nodeName, workloads, false)
}

// FixNodeResource .
func (bp *BinaryPlugin) FixNodeResource(ctx context.Context, nodeName string, workloads []*coretypes.Workload) (resp *GetNodeResourceInfoResponse, err error) {
	return bp.getNodeResourceInfo(ctx, nodeName, workloads, true)
}

// SetNodeResourceInfo .
func (bp *BinaryPlugin) SetNodeResourceInfo(ctx context.Context, nodeName string, resourceCapacity coretypes.NodeResourceArgs, resourceUsage coretypes.NodeResourceArgs) (*SetNodeResourceInfoResponse, error) {
	req := SetNodeResourceInfoRequest{
		NodeName: nodeName,
		Capacity: resourceCapacity,
		Usage:    resourceUsage,
	}
	resp := &SetNodeResourceInfoResponse{}
	return resp, bp.call(ctx, setNodeResourceInfoCommand, req, resp)
}

// GetDeployArgs .
func (bp *BinaryPlugin) GetDeployArgs(ctx context.Context, nodeName string, deployCount int, resourceOpts coretypes.WorkloadResourceOpts) (resp *GetDeployArgsResponse, err error) {
	req := GetDeployArgsRequest{
		NodeName:     nodeName,
		DeployCount:  deployCount,
		ResourceOpts: resourceOpts,
	}
	resp = &GetDeployArgsResponse{}
	if err := bp.call(ctx, getDeployArgsCommand, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetReallocArgs .
func (bp *BinaryPlugin) GetReallocArgs(ctx context.Context, nodeName string, originResourceArgs coretypes.WorkloadResourceArgs, resourceOpts coretypes.WorkloadResourceOpts) (resp *GetReallocArgsResponse, err error) {
	req := GetReallocArgsRequest{
		NodeName:     nodeName,
		Old:          originResourceArgs,
		ResourceOpts: resourceOpts,
	}
	resp = &GetReallocArgsResponse{}
	if err := bp.call(ctx, getReallocArgsCommand, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetRemapArgs .
func (bp *BinaryPlugin) GetRemapArgs(ctx context.Context, nodeName string, workloadMap map[string]*coretypes.Workload) (*GetRemapArgsResponse, error) {
	workloadResourceArgsMap := map[string]coretypes.WorkloadResourceArgs{}
	for workloadID, workload := range workloadMap {
		workloadResourceArgsMap[workloadID] = workload.ResourceArgs[bp.Name()]
	}

	req := GetRemapArgsRequest{
		NodeName:    nodeName,
		WorkloadMap: workloadResourceArgsMap,
	}
	resp := &GetRemapArgsResponse{}
	if err := bp.call(ctx, getRemapArgsCommand, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (bp *BinaryPlugin) SetNodeResourceUsage(ctx context.Context, nodeName string, nodeResourceOpts coretypes.NodeResourceOpts, nodeResourceArgs coretypes.NodeResourceArgs, workloadResourceArgs []coretypes.WorkloadResourceArgs, delta bool, incr bool) (*SetNodeResourceUsageResponse, error) {
	req := SetNodeResourceUsageRequest{
		NodeName:             nodeName,
		WorkloadResourceArgs: workloadResourceArgs,
		NodeResourceOpts:     nodeResourceOpts,
		NodeResourceArgs:     nodeResourceArgs,
		Delta:                delta,
		Decr:                 !incr,
	}

	resp := &SetNodeResourceUsageResponse{}
	if err := bp.call(ctx, setNodeResourceUsageCommand, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (bp *BinaryPlugin) SetNodeResourceCapacity(ctx context.Context, nodeName string, nodeResourceOpts coretypes.NodeResourceOpts, nodeResourceArgs coretypes.NodeResourceArgs, delta bool, incr bool) (*SetNodeResourceCapacityResponse, error) {
	req := SetNodeResourceCapacityRequest{
		NodeName:         nodeName,
		NodeResourceOpts: nodeResourceOpts,
		NodeResourceArgs: nodeResourceArgs,
		Delta:            delta,
		Decr:             !incr,
	}

	resp := &SetNodeResourceCapacityResponse{}
	if err := bp.call(ctx, setNodeResourceCapacityCommand, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// AddNode .
func (bp *BinaryPlugin) AddNode(ctx context.Context, nodeName string, resourceOpts coretypes.NodeResourceOpts, nodeInfo *enginetypes.Info) (resp *AddNodeResponse, err error) {
	req := AddNodeRequest{
		NodeName:     nodeName,
		ResourceOpts: resourceOpts,
	}
	resp = &AddNodeResponse{}
	if err := bp.call(ctx, addNodeCommand, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// RemoveNode .
func (bp *BinaryPlugin) RemoveNode(ctx context.Context, nodeName string) (*RemoveNodeResponse, error) {
	req := RemoveNodeRequest{
		NodeName: nodeName,
	}
	resp := &RemoveNodeResponse{}
	return resp, bp.call(ctx, removeNodeCommand, req, resp)
}

// GetMostIdleNode .
func (bp *BinaryPlugin) GetMostIdleNode(ctx context.Context, nodeNames []string) (*GetMostIdleNodeResponse, error) {
	req := GetMostIdleNodeRequest{
		NodeNames: nodeNames,
	}
	resp := &GetMostIdleNodeResponse{}
	return resp, bp.call(ctx, getMostIdleNodeCommand, req, resp)
}

// Name .
func (bp *BinaryPlugin) Name() string {
	return path.Base(bp.path)
}
