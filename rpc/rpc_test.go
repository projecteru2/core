package rpc

import (
	"net"
	"testing"
	"time"

	"context"

	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/cluster/calcium"
	"gitlab.ricebook.net/platform/core/rpc/gen"
	"gitlab.ricebook.net/platform/core/store/mock"
	"gitlab.ricebook.net/platform/core/types"
	"google.golang.org/grpc"
)

// Test Pods functions
func TestPods(t *testing.T) {
	ctx := context.Background()
	store := &mockstore.MockStore{}
	config, v := initConfig(store)

	// startup the server
	grpcServer := startServer(config, v)
	defer stopServer(grpcServer, v)

	// create a client
	conn, err := grpc.Dial("localhost:5001", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Init gRPC conn error: %v", err)
	}
	defer conn.Close()
	clnt := pb.NewCoreRPCClient(conn)

	// Add a pod
	pod := &types.Pod{Name: "testpod", Desc: "desc", Favor: "MEM"}
	store.On("AddPod", "testpod", "", "desc").Return(pod, nil)
	addpodoptions := pb.AddPodOptions{
		Name:  "testpod",
		Favor: "",
		Desc:  "desc",
	}
	respAddPod, err := clnt.AddPod(ctx, &addpodoptions)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, pod.Name, respAddPod.GetName())

	// ListPods
	log.Infoln("testing list pods")
	p1 := &types.Pod{Name: "pod1", Desc: "this is pod-1", Favor: "MEM"}
	p2 := &types.Pod{Name: "pod2", Desc: "thie is pod-2", Favor: "MEM"}
	pods := []*types.Pod{p1, p2}
	store.On("GetAllPods").Return(pods, nil)

	listPods, err := clnt.ListPods(ctx, &pb.Empty{})
	if err != nil {
		t.Error(err)
		return
	}
	for i, p := range listPods.Pods {
		assert.Equal(t, pods[i].Name, p.Name)
		assert.Equal(t, pods[i].Desc, p.Desc)
	}

	// Get a pod
	log.Infoln("testing get pod")
	store.On("GetPod", "pod1").Return(p1, nil)
	getpodoptions := pb.GetPodOptions{Name: "pod1"}
	podGet, err := clnt.GetPod(ctx, &getpodoptions)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, p1.Name, podGet.Name)

}

// Test Nodes functions
func TestNodes(t *testing.T) {
	ctx := context.Background()
	store := &mockstore.MockStore{}
	config, v := initConfig(store)

	// startup the server
	grpcServer := startServer(config, v)
	defer stopServer(grpcServer, v)

	// create a client
	conn, err := grpc.Dial("localhost:5001", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Init gRPC conn error: %v", err)
	}
	defer conn.Close()
	clnt := pb.NewCoreRPCClient(conn)

	// test add node
	log.Infoln("testing add a node")
	var (
		nodeName  = "node"
		endpoint  = "xxx"
		podname   = "pod"
		cafile    = ""
		certfile  = ""
		keyfile   = ""
		public    = true
		available = true
	)
	tNode := &types.Node{
		Name:      nodeName,
		Endpoint:  endpoint,
		Podname:   podname,
		Public:    public,
		Available: available,
		CPU: types.CPUMap{
			"0": 1e9,
		},
		MemCap: 1024 * 1024 * 8,
		Engine: nil,
	}
	store.On("AddNode", nodeName, endpoint, podname, cafile, certfile, keyfile, public).Return(tNode, nil)
	addnodeoptions := pb.AddNodeOptions{
		Nodename: nodeName,
		Endpoint: endpoint,
		Podname:  podname,
		Public:   public,
		Cafile:   cafile,
		Certfile: certfile,
		Keyfile:  keyfile,
	}
	respAddPod, err := clnt.AddNode(ctx, &addnodeoptions)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, tNode.Endpoint, respAddPod.GetEndpoint())

	// test get node
	gnOpts := &pb.GetNodeOptions{
		Podname:  podname,
		Nodename: nodeName,
	}
	store.On("GetNode", podname, nodeName).Return(tNode, nil)
	store.On("DeleteNode", tNode).Return()
	gnResp, err := clnt.GetNode(ctx, gnOpts)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, gnResp.Name, tNode.Name)

	// test remove node
	rnOpts := &pb.RemoveNodeOptions{
		Nodename: nodeName,
		Podname:  podname,
	}
	pod := &types.Pod{Name: podname, Desc: "this is pod-1", Favor: "MEM"}
	store.On("GetPod", podname).Return(pod, nil)
	// 其实我很不理解为什么删除一个node的返回结果是它的pod
	rnResp, err := clnt.RemoveNode(ctx, rnOpts)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, rnResp.Name, pod.Name)

	// test ListPodNodes
	nodes := []*types.Node{tNode}
	store.On("GetNodesByPod", podname).Return(nodes, nil)
	lpnOpts := &pb.ListNodesOptions{
		Podname: podname,
		All:     true,
	}
	lpnResp, err := clnt.ListPodNodes(ctx, lpnOpts)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, lpnResp.GetNodes()[0].GetEndpoint(), nodes[0].Endpoint)

}

// initConfig returns config and vibranium(about gRPC)
func initConfig(mStore *mockstore.MockStore) (types.Config, *vibranium) {

	config := types.Config{
		Bind:           ":5001",           // HTTP API address
		AppDir:         "/tmp",            // App directory inside container
		PermDir:        "/tmp",            // Permanent dir on host
		BackupDir:      "/tmp",            // Backup dir on host
		EtcdMachines:   []string{"MOCK"},  // etcd cluster addresses
		EtcdLockPrefix: "/eru-core/_lock", // etcd lock prefix, all locks will be created under this dir
		ResourceAlloc:  "cpu-period",      // scheduler or cpu-period TODO give it a good name
		Statsd:         "localhost:1080",  // Statsd host and port
		Zone:           "c1",              // zone for core, e.g. C1, C2

		Git: types.GitConfig{
			SCMType: "gitlab",
		},
		Scheduler: types.SchedConfig{
			LockKey: "_scheduler_lock",
			LockTTL: 10,
			Type:    "complex",
		},
		Syslog: types.SyslogConfig{
			Address:  "udp://localhost:5111",
			Facility: "daemon",
			Format:   "rfc5424",
		},
		Docker: types.DockerConfig{
			APIVersion: "v1.23",
			LogDriver:  "none",
			BuildPod:   "dev",
		},
	}

	cluster, err := calcium.New(config)
	if err != nil {
		log.Fatal(err)
	}
	cluster.SetStore(mStore)
	vibranium := New(cluster, config)

	return config, vibranium

}

// startServer start a gRPC server and return it
func startServer(config types.Config, v *vibranium) *grpc.Server {
	s, err := net.Listen("tcp", config.Bind)
	if err != nil {
		log.Fatal(err)
	}

	opts := []grpc.ServerOption{grpc.MaxConcurrentStreams(100)}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterCoreRPCServer(grpcServer, v)
	go grpcServer.Serve(s)
	log.Info("Cluster started successfully.")
	return grpcServer
}

// stopServer stops the gRPC server with 30s timeout if there is some unfinished goroutines
// otherwise the gRPC server will stop gracefully
func stopServer(grpcServer *grpc.Server, v *vibranium) error {
	time.Sleep(1 * time.Second)       // to aviod "read: connection reset by peer"
	defer time.Sleep(1 * time.Second) // to aviod "bind error"
	grpcServer.GracefulStop()
	log.Info("gRPC server stopped gracefully.")

	log.Info("Now check if cluster still have running tasks...")
	wait := make(chan interface{})
	go func() {
		v.Wait()
		wait <- ""
	}()
	timer := time.NewTimer(time.Second * 30)
	select {
	case <-timer.C:
		// force quit(terminate all running tasks/goroutines)
		for {
			if v.TaskNum == 0 {
				break
			}
			v.taskDone("", false)
		}
		log.Info("Cluster stopped FORCEFULLY")
	case <-wait:
		log.Info("Cluster stopped gracefully")
	}
	return nil
}

// Test Container functions (test the error)
func TestContainers(t *testing.T) {
	ctx := context.Background()
	store := &mockstore.MockStore{}
	config, v := initConfig(store)

	// startup the server
	grpcServer := startServer(config, v)
	defer stopServer(grpcServer, v)

	// create a client
	conn, err := grpc.Dial("localhost:5001", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Init gRPC conn error: %v", err)
	}
	defer conn.Close()
	clnt := pb.NewCoreRPCClient(conn)

	// test GetContainer error
	store.On("GetContainer", "012").Return(&types.Container{}, nil)
	_, err = clnt.GetContainer(ctx, &pb.ContainerID{Id: "012"})
	assert.Contains(t, err.Error(), "Container ID must be length of 64")

	ID := "586f906185de3ed755d0db1c0f37149c1eae1ba557a26adccbe1f51c500d07d1"
	store.On("GetContainer", ID).Return(&types.Container{}, nil)
	_, err = clnt.GetContainer(ctx, &pb.ContainerID{Id: ID})
	assert.Contains(t, err.Error(), "Engine is nil")

	// test GetContainers
	container := types.Container{
		ID: ID,
	}
	store.On("GetContainers", []string{ID}).Return([]*types.Container{&container}, nil)
	gcResp, err := clnt.GetContainers(ctx, &pb.ContainerIDs{Ids: []*pb.ContainerID{&pb.ContainerID{Id: ID}}})
	assert.NoError(t, err)
	// 因为Container的engine是nil，所以获取不到信息，调用返回为空
	log.Info(gcResp)
	assert.Nil(t, gcResp.GetContainers())
}

func TestNetwork(t *testing.T) {
	// 由于测试的时候没有 dockerEngine，不能 docker network ls，所以并不能得到真是结果，只能进行错误测试
	ctx := context.Background()
	store := &mockstore.MockStore{}
	config, v := initConfig(store)

	// startup the server
	grpcServer := startServer(config, v)
	defer stopServer(grpcServer, v)

	// create a client
	conn, err := grpc.Dial("localhost:5001", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Init gRPC conn error: %v", err)
	}
	defer conn.Close()
	clnt := pb.NewCoreRPCClient(conn)

	// test GetContainer error
	store.On("GetNodesByPod", "podname").Return([]*types.Node{}, nil)
	_, err = clnt.ListNetworks(ctx, &pb.GetPodOptions{Name: "podname"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no nodes")
}

func TestBuildImage(t *testing.T) {
	ctx := context.Background()
	store := &mockstore.MockStore{}
	config, v := initConfig(store)

	// startup the server
	grpcServer := startServer(config, v)
	defer stopServer(grpcServer, v)

	// create a client
	conn, err := grpc.Dial("localhost:5001", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Init gRPC conn error: %v", err)
	}
	defer conn.Close()
	clnt := pb.NewCoreRPCClient(conn)

	tNode := &types.Node{
		Name:      "nodename",
		Endpoint:  "endpoint",
		Podname:   "dev",
		Public:    true,
		Available: true,
		CPU: types.CPUMap{
			"0": 1e9,
		},
		MemCap: 1024 * 1024 * 8,
		Engine: nil,
	}
	store.On("GetNodesByPod", "dev").Return([]*types.Node{tNode}, nil)
	store.On("GetNode", "dev", "nodename").Return(tNode, nil)

	biOpts := pb.BuildImageOptions{
		Repo:     "noexistrepo",
		Version:  "0",
		Uid:      "-1",
		Artifact: "",
	}
	resp, err := clnt.BuildImage(ctx, &biOpts) // 这个err是RPC调用的error，比如连接断开，连接失败
	assert.NoError(t, err)

	recv, err := resp.Recv() // 这个err是 BuildImage 产生的error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Bad git url format")
	assert.Nil(t, recv)
}

func TestCreateContainer(t *testing.T) {
	ctx := context.Background()
	store := &mockstore.MockStore{}
	config, v := initConfig(store)

	// startup the server
	grpcServer := startServer(config, v)
	defer stopServer(grpcServer, v)

	// create a client
	conn, err := grpc.Dial("localhost:5001", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Init gRPC conn error: %v", err)
	}
	defer conn.Close()
	clnt := pb.NewCoreRPCClient(conn)

	// configure mock store
	nodename := "nodename"
	podname := "dev"
	tNode := &types.Node{
		Name:      nodename,
		Endpoint:  "endpoint",
		Podname:   podname,
		Public:    true,
		Available: true,
		CPU: types.CPUMap{
			"0": 1e9,
		},
		MemCap: 1024 * 1024 * 8,
		Engine: nil,
	}
	pod := &types.Pod{Name: podname, Desc: "this is pod-1", Favor: "MEM"}
	store.On("GetPod", podname).Return(pod, nil)
	store.On("GetNodesByPod", podname).Return([]*types.Node{tNode}, nil)
	store.On("GetNode", podname, nodename).Return(tNode, nil)

	depOpts := pb.DeployOptions{
		Specs: `appname: testapp
entrypoints: {"0": {run_and_wait_timeout: 0}}
base: ""`, // string
		Appname:    "testapp", // string
		Image:      "",        // string
		Podname:    "dev",     // string
		Nodename:   "",        // string
		Entrypoint: "",        // string
		ExtraArgs:  "",        // string
		CpuQuota:   1024,      // float64
		Count:      -1,        // int
		Memory:     666,       // int64
	}
	resp, err := clnt.CreateContainer(ctx, &depOpts)
	assert.NoError(t, err)
	recv, err := resp.Recv()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Minimum memory limit allowed is 4MB")
	assert.Nil(t, recv)

	depOpts = pb.DeployOptions{
		Specs: `appname: testapp
entrypoints: {"0": {run_and_wait_timeout: 0}}
base: ""`, // string
		Appname:    "testapp", // string
		Image:      "",        // string
		Podname:    "dev",     // string
		Nodename:   "",        // string
		Entrypoint: "",        // string
		ExtraArgs:  "",        // string
		CpuQuota:   1024,      // float64
		Count:      -1,        // int
		Memory:     6666666,   // int64
	}
	resp, err = clnt.CreateContainer(ctx, &depOpts)
	assert.NoError(t, err)
	recv, err = resp.Recv()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Count must be positive")
	assert.Nil(t, recv)

}

func TestRunAndWait(t *testing.T) {
	ctx := context.Background()
	store := &mockstore.MockStore{}
	config, v := initConfig(store)

	// startup the server
	grpcServer := startServer(config, v)
	defer stopServer(grpcServer, v)

	// create a client
	conn, err := grpc.Dial("localhost:5001", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Init gRPC conn error: %v", err)
	}
	defer conn.Close()
	clnt := pb.NewCoreRPCClient(conn)

	// configure mock store
	podname := "dev"
	pod := &types.Pod{Name: podname, Desc: "this is pod-1", Favor: "MEM"}
	store.On("GetPod", podname).Return(pod, nil)

	depOpts := pb.DeployOptions{
		Specs: `appname: testapp
entrypoints: {"0": {run_and_wait_timeout: 0}}
base: ""`, // string
		Appname:    "testapp", // string
		Image:      "",        // string
		Podname:    "dev",     // string
		Nodename:   "",        // string
		Entrypoint: "",        // string
		ExtraArgs:  "",        // string
		CpuQuota:   1024,      // float64
		Count:      -1,        // int
		Memory:     666,       // int64
	}
	runAndWaitOpts := pb.RunAndWaitOptions{
		DeployOptions: &depOpts,
	}
	stream, err := clnt.RunAndWait(ctx)
	assert.NoError(t, err)
	assert.NoError(t, stream.Send(&runAndWaitOpts))
	_, err = stream.Recv()
	assert.Contains(t, err.Error(), "Minimum memory limit allowed is 4MB")

	depOpts = pb.DeployOptions{
		Specs: `appname: testapp
entrypoints: {"0": {run_and_wait_timeout: 0}}
base: ""`, // string
		Appname:    "testapp", // string
		Image:      "",        // string
		Podname:    "dev",     // string
		Nodename:   "",        // string
		Entrypoint: "",        // string
		ExtraArgs:  "",        // string
		CpuQuota:   1024,      // float64
		Count:      -1,        // int
		Memory:     666666,    // int64
		OpenStdin:  true,
	}
	runAndWaitOpts = pb.RunAndWaitOptions{
		DeployOptions: &depOpts,
	}
	stream, err = clnt.RunAndWait(ctx)
	assert.NoError(t, err)
	assert.NoError(t, stream.Send(&runAndWaitOpts))
	_, err = stream.Recv()
	assert.Contains(t, err.Error(), "[RunAndWait] Count must be 1 if OpenStdin is true")

}

func TestOthers(t *testing.T) {
	ctx := context.Background()
	store := &mockstore.MockStore{}
	config, v := initConfig(store)

	// startup the server
	grpcServer := startServer(config, v)
	defer stopServer(grpcServer, v)

	// create a client
	conn, err := grpc.Dial("localhost:5001", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Init gRPC conn error: %v", err)
	}
	defer conn.Close()
	clnt := pb.NewCoreRPCClient(conn)

	// mock store
	ID := "586f906185de3ed755d0db1c0f37149c1eae1ba557a26adccbe1f51c500d07d1"
	podname := "podname"
	nodename := "nodename"
	store.On("GetContainer", ID).Return(&types.Container{}, nil)
	container := types.Container{
		ID:       ID,
		Podname:  podname,
		Nodename: nodename,
	}
	store.On("GetContainers", []string{ID}).Return([]*types.Container{&container}, nil)
	tNode := &types.Node{
		Name:      nodename,
		Endpoint:  "endpoint",
		Podname:   podname,
		Public:    true,
		Available: true,
		CPU: types.CPUMap{
			"0": 1e9,
		},
		MemCap: 1024 * 1024 * 8,
		Engine: nil,
	}
	store.On("GetNode", podname, nodename).Return(tNode, nil)

	// RemoveContainer
	rmContainerResp, _ := clnt.RemoveContainer(ctx, &pb.ContainerIDs{Ids: []*pb.ContainerID{&pb.ContainerID{Id: ID}}})
	r, err := rmContainerResp.Recv() // 同理这个err也是gRPC调用的error，而不是执行动作的error
	assert.Nil(t, err)
	assert.Contains(t, r.GetMessage(), "Engine is nil")

	// RemoveImage 和 Backup 无法测试，因为 node Engine is nil

}
