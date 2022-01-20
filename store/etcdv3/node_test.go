package etcdv3

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func TestAddNode(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	nodename := "testnode"
	nodename2 := "testnode2"
	endpoint := "tcp://128.0.0.1:2376"
	podname := "testpod"
	_, err := m.AddPod(ctx, podname, "test")
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, "numapod", "test")
	assert.NoError(t, err)
	cpu := 1
	share := 100
	memory := int64(100)
	storage := int64(100)
	m.config.Scheduler.ShareBase = 100
	labels := map[string]string{"test": "1"}

	// wrong endpoint
	ctx1, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err = m.AddNode(ctx1, &types.AddNodeOptions{Nodename: nodename, Endpoint: "abc", Podname: podname, CPU: cpu, Share: share, Memory: memory, Storage: storage, Labels: labels})
	assert.Error(t, err)

	// wrong because engine not mocked
	ctx2, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err = m.AddNode(ctx2, &types.AddNodeOptions{Nodename: nodename, Endpoint: endpoint, Podname: podname, CPU: cpu, Share: share, Memory: memory, Storage: storage, Labels: labels})
	assert.Error(t, err)
	endpoint = "mock://fakeengine"
	// wrong no pod
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: endpoint, Podname: "abc", CPU: cpu, Share: share, Memory: memory, Storage: storage, Labels: labels})
	assert.Error(t, err)
	// AddNode
	node, err := m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: endpoint, Podname: podname, CPU: cpu, Share: share, Memory: memory, Storage: storage, Labels: labels})
	assert.NoError(t, err)
	assert.Equal(t, node.Name, nodename)
	assert.Equal(t, node.CPU["0"], int64(100))
	// add again and failed
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: endpoint, Podname: podname, CPU: cpu, Share: share, Memory: memory, Storage: storage, Labels: labels})
	assert.Error(t, err)
	// AddNode with numa
	nodeWithNuma, err := m.AddNode(ctx, &types.AddNodeOptions{Nodename: "nodewithnuma", Endpoint: endpoint, Podname: "numapod", CPU: cpu, Share: share, Memory: memory, Storage: storage, Labels: labels, Numa: types.NUMA{"1": "n1", "2": "n2"}})
	assert.NoError(t, err)
	assert.Equal(t, nodeWithNuma.Name, "nodewithnuma")
	assert.Equal(t, len(nodeWithNuma.NUMAMemory), 2)
	assert.Equal(t, nodeWithNuma.NUMAMemory["n1"], int64(50))
	// Addnode again will failed
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: endpoint, Podname: podname, CPU: cpu, Share: share, Memory: memory, Storage: storage, Labels: labels})
	assert.Error(t, err)
	// Check etcd has node data
	key := fmt.Sprintf(nodeInfoKey, nodename)
	_, err = m.GetOne(ctx, key)
	assert.NoError(t, err)
	// AddNode with mocked engine and default value
	node2, err := m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename2, Endpoint: endpoint, Podname: podname, Labels: labels})
	assert.NoError(t, err)
	assert.Equal(t, node2.CPU["0"], int64(100))
	assert.Equal(t, len(node2.CPU), 100)
	assert.Equal(t, node2.MemCap, int64(85899345920))
	// with tls
	ca := `-----BEGIN CERTIFICATE-----
MIIC7TCCAdWgAwIBAgIJAM8uLRZf9jttMA0GCSqGSIb3DQEBCwUAMA0xCzAJBgNV
BAYTAkNOMB4XDTE4MDYxODA5MTkwNloXDTI4MDYxNTA5MTkwNlowDTELMAkGA1UE
BhMCQ04wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDWMezGaddx5dmf
i3p28ZqBk2JHhAxlKqvKTIfaH3D/44roZD0kXgLNRYp3FxV4TUK4A2jxvOxfF/UZ
/fLjX66zsi/nKX0Uv3i+tk4MbU3H+A0N2YsE7Nx5ldDSw1gUe2LoA687Fn0iZHED
wWdUhkev3291wK2DY0BnSfTUFejhaUbDPgbTIaQwzxFCN26QpO5EBBFQPfdp9ilg
rfhYgoQ016BWM3Nuor3yeX9WPZdCdCq0/zM+Pn62A6EUBnC0oLskz5Sbdq3kAio2
FmQLEu2ZGV9BzfIPFLfI/K9y7kH+OvbaFlZxjqsYVqjpIwmeCINYcQGP6JVrPvcQ
5X4u0oHVAgMBAAGjUDBOMB0GA1UdDgQWBBRElDLtPaaMarh8xKkHsmHZmug6sDAf
BgNVHSMEGDAWgBRElDLtPaaMarh8xKkHsmHZmug6sDAMBgNVHRMEBTADAQH/MA0G
CSqGSIb3DQEBCwUAA4IBAQBGREhJnNwgZIPio/Dof1JUWSmD1uamNMNQSbluWs+5
fQrxGPMzh0maEfah21R2fXcrqXDH9qa7lGGPWc1wrn/pqTPQYYZyeCiEOZAHYMM7
orSraF6M9pwLn/MB5O9onTg1RkEABpw6K0YsKShTW/rM0o5JMGD/fnX4Vdr/vsTi
190TboLkQwFd+16x1C6/YVIBx2wM8b+shm5sZeXHQNDD4Cp8iQOPKwxot1vnefTl
+ksb5sqiWIx88LBRouXI65ORPHSejZrr3iVowu9EvtwaxQ8E9QXfb1eks0WWzPkX
lgCr4uFZk7z7nRQVUfLSkNYNGKY8P62xLtjigp4rYsLB
-----END CERTIFICATE-----`
	cert := `-----BEGIN CERTIFICATE-----
MIICmjCCAYICCQDeEJuzNfHgXzANBgkqhkiG9w0BAQsFADANMQswCQYDVQQGEwJD
TjAeFw0xODA2MTgwOTE5MDZaFw0yODA2MTUwOTE5MDZaMBExDzANBgNVBAMMBmNs
aWVudDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAPftUgiEEXkQK2Mj
3PNTNZsV6K8ouKyUlVAA/7w102VsP27oSbzjTc1ad9Hyfc9PxYe1l7sbTgKqlC9S
p5C1wTswtnbcVwdIUMtizupr9MCoBLXpEFTpNS0FxGKyltJTGOppUvlN9VKbrV4B
rUIGpTClbvfAc8iLymYn05sRw+JMBD5rStv/KnZPYeIYliMaS147fXGQDhgQ4gdx
6cguL22hfvWGov3Sot1FsANStVqYAEvd6Bk0KgZi2eupRigduboUth1VzRN4TNSi
Rzk2Vy6CBhMyYRj0UogJD9WfI//qej1/IOFA9hiOY6+WflFiaqCi4TbJd2D9IDof
hGE7bdsCAwEAATANBgkqhkiG9w0BAQsFAAOCAQEAKgW1Co/H34XrXS+ms+9X9xKJ
sv8HCrNUrsKF3/srwkNmLySJYqAaIcUmXKCzslYOF5mPi3XCcKNVWdH8FYwQxgJJ
mK9a460/0z9Znf0J1cfFELR4BTvFEp0s2HY38W5yojiJqo6zQIztD1hqONHJPeF8
jvPFvH2WREjVLvF8Y8U+qWcRhZRoPex2a6rr/33kgx1+cuNC9opBD0pIa5+HpnqW
M+P7/vNqjwaYS5HMKPs9bqrtYLKAIVSWa4Do0kDd2sX8/ngMPsxTJzZEqbXBnEJt
YsRamLe0Yu66g0ZhRzHpUxOio9+LyT1vs/hRGEtoEXZfZUF39+4J4C6xmNcE4A==
-----END CERTIFICATE-----`
	certkey := `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQD37VIIhBF5ECtj
I9zzUzWbFeivKLislJVQAP+8NdNlbD9u6Em8403NWnfR8n3PT8WHtZe7G04CqpQv
UqeQtcE7MLZ23FcHSFDLYs7qa/TAqAS16RBU6TUtBcRispbSUxjqaVL5TfVSm61e
Aa1CBqUwpW73wHPIi8pmJ9ObEcPiTAQ+a0rb/yp2T2HiGJYjGkteO31xkA4YEOIH
cenILi9toX71hqL90qLdRbADUrVamABL3egZNCoGYtnrqUYoHbm6FLYdVc0TeEzU
okc5NlcuggYTMmEY9FKICQ/VnyP/6no9fyDhQPYYjmOvln5RYmqgouE2yXdg/SA6
H4RhO23bAgMBAAECggEAY4jkoUScWzUxpgi04P9sCwo9s2yuz6KLW2Y7RY16hEJ0
KQua5vl+t831QtWOytck33n5I4YvyIRBH8qYOVGu9Rt2dbu6ONNAlJbjqVuUFHCg
C4Q5KU3DKoMhN9qpEGGKJDoKtMomjnavoIkdzN8sHJ6eMVsTYNU2edLNcnksYkIG
Oi+LZFaYqI3Gn5GfWznJi12CIcNjutipWHB2Q01r/THEUUDAm6f/vUonVbJKn0GQ
dHUNl52Z3Gg6i+R1zz/ilm0a6wbVhz0aKTyhGOru+z9dZzxktl51JUws42zbTp7e
xU8x+kA4Nvec228a5wcpS1uiO1ogme48grEK2NhqgQKBgQD+VD5WiGVGUuGRgsQ6
dpO+IcG5LZC0lrxGR3zY0fvtPyqqDmnmA80jefZUy5THsJ0FMg6lKdfIIO+W2nkk
+jG4RAv/AQiM2QT9jQMDpF6eJxSQYm4zcGhtrIy0i79upZM0sPoBxzNQVUWpRxPT
YJWwiTaPFc0KaQ40ZbU8s1xDwQKBgQD5jk8wFoSLSN/nTVY3m23SJBejGBxyUtP/
mIPBmSdRxbdB2MoTTg+zRiQj7McUu8E/SW6RB4ScQWBRjbgin0lTVYUn25xlrXDB
sFh/AWq4m98O5ju/fZGDRSheDkMUxyC0DFSin91sF7iBZqv/HJBITncTghhrLXpQ
umltothomwKBgQDxInysXMvw3jpCVYKpj63K0oSzhzExF83QsIz9ojJDIeXYsKvV
SvtfzI4ynYclwh1ORMS/8ilF9XxUQjYkSheEBvh8wcUSjdz+bYlTFbAkMRd9QeYM
XWKVwcjykaFiThiBF98ienT7kK3orpxsiKHEbIRPK7NpUGwIX/pzX/d1wQKBgQDF
/GtCwXqibkyE20xdjYhRQaUnFYfsA16B12Qggfs52tyK9w1Kx5GZLzqY7c772gF0
zjNUCFzjAtMBoKfHgAvSe3TKrGamHDXq1JdBG8SpdbA/x9T7FQoO1R0zkakSoPCH
J4k2BBLNIPyWXPhzyxuE4guChKIO1ePGjD38Z0e9pQKBgFHI//1VkRSZu49jrGOw
Dl0aR0+ZbHq5hv5feDdpeKxMxKkKnCu1cl47gKAyFet5nvK7htBUk9aIph8zDBZj
cmag5uyTcIogsd5GyOg06jDD1aCqz3FbTX1KQLeSFQUCTT+m100rohrc2rW5m4Al
RdCPRPt513WozkJZZAjUSP2U
-----END PRIVATE KEY-----`
	nodename3 := "nodename3"
	endpoint3 := "tcp://path"
	m.config.CertPath = "/tmp"
	node3, err := m.doAddNode(ctx, nodename3, endpoint3, podname, ca, cert, certkey, cpu, share, memory, storage, labels, nil, nil, nil)
	assert.NoError(t, err)
	_, err = m.makeClient(ctx, node3)
	assert.Error(t, err)
	// failed by get key
	node3.Name = "nokey"
	_, err = m.makeClient(ctx, node3)
	assert.Error(t, err)
}

func TestRemoveNode(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	node, err := m.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", 100, 100, 100000, 100000, nil, nil, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, node.Name, "test")
	assert.NoError(t, m.RemoveNode(ctx, nil))
	assert.NoError(t, m.RemoveNode(ctx, node))
}

func TestGetNode(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	node, err := m.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", 100, 100, 100000, 100000, nil, nil, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, node.Name, "test")
	_, err = m.GetNode(ctx, "wtf")
	assert.Error(t, err)
	n, err := m.GetNode(ctx, "test")
	assert.NoError(t, err)
	assert.Equal(t, node.Name, n.Name)
}

func TestGetNodesByPod(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	node, err := m.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", 100, 100, 100000, 100000, map[string]string{"x": "y"}, nil, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, node.Name, "test")
	ns, err := m.GetNodesByPod(ctx, "wtf", nil, false)
	assert.NoError(t, err)
	assert.Empty(t, ns)
	ns, err = m.GetNodesByPod(ctx, "testpod", nil, true)
	assert.NoError(t, err)
	assert.NotEmpty(t, ns)
	_, err = m.AddPod(ctx, "testpod", "")
	assert.NoError(t, err)
	ns, err = m.GetNodesByPod(ctx, "", nil, false)
	assert.NoError(t, err)
	assert.Empty(t, ns)
	ns, err = m.GetNodesByPod(ctx, "", nil, true)
	assert.NoError(t, err)
	assert.NotEmpty(t, ns)
}

func TestUpdateNode(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	node, err := m.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", 100, 100, 100000, 100000, map[string]string{"x": "y"}, nil, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, node.Name, "test")
	fakeNode := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "nil",
			Podname:  "wtf",
			Endpoint: "mock://hh",
			Ca:       "hh",
			Cert:     "hh",
			Key:      "hh",
		},
	}
	assert.NoError(t, m.UpdateNodes(ctx, fakeNode))
	assert.NoError(t, m.UpdateNodes(ctx, node))
	node.Available = false
	assert.NoError(t, m.UpdateNodes(ctx, node))
}

func TestUpdateNodeResource(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	node, err := m.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", 1, 100, 100000, 100000, map[string]string{"x": "y"}, map[string]string{"0": "0"}, map[string]int64{"0": 100}, nil)
	assert.NoError(t, err)
	assert.Equal(t, node.Name, "test")
	assert.Error(t, m.UpdateNodeResource(ctx, node, nil, "wtf"))
	assert.NoError(t, m.UpdateNodeResource(ctx, node, &types.ResourceMeta{CPU: map[string]int64{"0": 100}}, types.ActionIncr))
	assert.NoError(t, m.UpdateNodeResource(ctx, node, &types.ResourceMeta{CPU: map[string]int64{"0": 100}}, types.ActionDecr))
}

func TestExtractNodename(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(extractNodename("/nodestatus/testname"), "testname")
}

func TestSetNodeStatus(t *testing.T) {
	assert := assert.New(t)
	m := NewMercury(t)

	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "mock://",
			Podname:  "testpod",
		},
	}
	_, err := m.AddPod(context.TODO(), node.Podname, "")
	assert.NoError(err)
	_, err = m.AddNode(context.TODO(), &types.AddNodeOptions{
		Nodename: node.Name,
		Endpoint: node.Endpoint,
		Podname:  node.Podname,
	})
	assert.NoError(err)
	assert.NoError(m.SetNodeStatus(context.TODO(), node, 1))
	key := filepath.Join(nodeStatusPrefix, node.Name)

	// not expired yet
	_, err = m.GetOne(context.TODO(), key)
	assert.NoError(err)
	// expired
	time.Sleep(2000 * time.Millisecond)
	_, err = m.GetOne(context.TODO(), key)
	assert.Error(err)
}

func TestGetNodeStatus(t *testing.T) {
	assert := assert.New(t)
	m := NewMercury(t)

	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "mock://",
			Podname:  "testpod",
		},
	}
	_, err := m.AddPod(context.TODO(), node.Podname, "")
	assert.NoError(err)
	_, err = m.AddNode(context.TODO(), &types.AddNodeOptions{
		Nodename: node.Name,
		Endpoint: node.Endpoint,
		Podname:  node.Podname,
	})
	assert.NoError(err)
	assert.NoError(m.SetNodeStatus(context.TODO(), node, 1))

	// not expired yet
	ns, err := m.GetNodeStatus(context.TODO(), node.Name)
	assert.NoError(err)
	assert.Equal(ns.Nodename, node.Name)
	assert.True(ns.Alive)
	// expired
	time.Sleep(2 * time.Second)
	ns1, err := m.GetNodeStatus(context.TODO(), node.Name)
	assert.Error(err)
	assert.Nil(ns1)
}

func TestNodeStatusStream(t *testing.T) {
	assert := assert.New(t)
	m := NewMercury(t)

	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "mock://",
			Podname:  "testpod",
		},
	}

	_, err := m.AddPod(context.TODO(), node.Podname, "")
	assert.NoError(err)
	_, err = m.AddNode(context.TODO(), &types.AddNodeOptions{
		Nodename: node.Name,
		Endpoint: node.Endpoint,
		Podname:  node.Podname,
	})
	assert.NoError(err)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			time.Sleep(500 * time.Millisecond)
			assert.NoError(m.SetNodeStatus(context.TODO(), node, 1))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	ch := m.NodeStatusStream(ctx)
	go func() {
		time.Sleep(3000 * time.Millisecond)
		cancel()
	}()

	statuses := []*types.NodeStatus{}
	for s := range ch {
		statuses = append(statuses, s)
	}
	for _, s := range statuses[:len(statuses)-1] {
		assert.True(s.Alive)
	}
	assert.False(statuses[len(statuses)-1].Alive)
}
