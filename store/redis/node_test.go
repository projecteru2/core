package redis

import (
	"context"
	"path/filepath"
	"time"

	"github.com/projecteru2/core/types"
)

func (s *RediaronTestSuite) TestAddNode() {
	ctx := context.Background()
	podname := "testpod"
	_, err := s.rediaron.AddPod(ctx, podname, "test")
	s.NoError(err)
	_, err = s.rediaron.AddPod(ctx, "numapod", "test")
	s.NoError(err)
	s.rediaron.config.Scheduler.ShareBase = 100
	labels := map[string]string{"test": "1"}

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
	s.rediaron.config.CertPath = "/tmp"
	node3, err := s.rediaron.doAddNode(ctx, nodename3, endpoint3, podname, ca, cert, certkey, labels)
	s.NoError(err)
	_, err = s.rediaron.makeClient(ctx, node3)
	s.Error(err)
	// failed by get key
	node3.Name = "nokey"
	_, err = s.rediaron.makeClient(ctx, node3)
	s.Error(err)
}

func (s *RediaronTestSuite) TestRemoveNode() {
	ctx := context.Background()
	node, err := s.rediaron.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", nil)
	s.NoError(err)
	s.Equal(node.Name, "test")
	s.NoError(s.rediaron.RemoveNode(ctx, nil))
	s.NoError(s.rediaron.RemoveNode(ctx, node))
}

func (s *RediaronTestSuite) TestGetNode() {
	ctx := context.Background()
	node, err := s.rediaron.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", nil)
	s.NoError(err)
	s.Equal(node.Name, "test")
	_, err = s.rediaron.GetNode(ctx, "wtf")
	s.Error(err)
	n, err := s.rediaron.GetNode(ctx, "test")
	s.NoError(err)
	s.Equal(node.Name, n.Name)
}

func (s *RediaronTestSuite) TestGetNodesByPod() {
	ctx := context.Background()
	node, err := s.rediaron.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", map[string]string{"x": "y"})
	s.NoError(err)
	s.Equal(node.Name, "test")
	ns, err := s.rediaron.GetNodesByPod(ctx, "wtf", nil, false)
	s.NoError(err)
	s.Empty(ns)
	ns, err = s.rediaron.GetNodesByPod(ctx, "testpod", nil, true)
	s.NoError(err)
	s.NotEmpty(ns)
	_, err = s.rediaron.AddPod(ctx, "testpod", "")
	s.NoError(err)
	ns, err = s.rediaron.GetNodesByPod(ctx, "", nil, false)
	s.NoError(err)
	s.Empty(ns)
	ns, err = s.rediaron.GetNodesByPod(ctx, "", nil, true)
	s.NoError(err)
	s.NotEmpty(ns)
}

func (s *RediaronTestSuite) TestUpdateNode() {
	ctx := context.Background()
	node, err := s.rediaron.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", map[string]string{"x": "y"})
	s.NoError(err)
	s.Equal(node.Name, "test")
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
	s.NoError(s.rediaron.UpdateNodes(ctx, fakeNode))
	s.NoError(s.rediaron.UpdateNodes(ctx, node))
}

func (s *RediaronTestSuite) TestUpdateNodeResource() {
	ctx := context.Background()
	node, err := s.rediaron.doAddNode(ctx, "test", "mock://", "testpod", "", "", "", map[string]string{"x": "y"})
	s.NoError(err)
	s.Equal(node.Name, "test")
}

func (s *RediaronTestSuite) TestExtractNodename() {
	s.Equal(extractNodename("/nodestatus/testname"), "testname")
}

func (s *RediaronTestSuite) TestSetNodeStatus() {
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "ep",
			Podname:  "testpod",
		},
	}
	s.NoError(s.rediaron.SetNodeStatus(context.TODO(), node, 1))
	key := filepath.Join(nodeStatusPrefix, node.Name)

	// not expired yet
	_, err := s.rediaron.GetOne(context.TODO(), key)
	s.NoError(err)
	// expired
	time.Sleep(2 * time.Second)
	// fastforward
	s.rediserver.FastForward(2 * time.Second)
	_, err = s.rediaron.GetOne(context.TODO(), key)
	s.Error(err)
}

func (s *RediaronTestSuite) TestGetNodeStatus() {
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "ep",
			Podname:  "testpod",
		},
	}
	s.NoError(s.rediaron.SetNodeStatus(context.TODO(), node, 1))

	// not expired yet
	ns, err := s.rediaron.GetNodeStatus(context.TODO(), node.Name)
	s.NoError(err)
	s.Equal(ns.Nodename, node.Name)
	s.True(ns.Alive)
	// expired
	time.Sleep(2 * time.Second)
	// fastforward
	s.rediserver.FastForward(2 * time.Second)
	ns1, err := s.rediaron.GetNodeStatus(context.TODO(), node.Name)
	s.Error(err)
	s.Nil(ns1)
}

func (s *RediaronTestSuite) TestNodeStatusStream() {
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "ep",
			Podname:  "testpod",
		},
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			time.Sleep(500 * time.Millisecond)
			s.NoError(s.rediaron.SetNodeStatus(context.TODO(), node, 1))
			// manually trigger
			triggerMockedKeyspaceNotification(s.rediaron.cli, filepath.Join(nodeStatusPrefix, node.Name), actionSet)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	ch := s.rediaron.NodeStatusStream(ctx)
	go func() {
		time.Sleep(1500 * time.Millisecond)
		// manually trigger
		triggerMockedKeyspaceNotification(s.rediaron.cli, filepath.Join(nodeStatusPrefix, node.Name), actionExpired)
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	statuses := []*types.NodeStatus{}
	for m := range ch {
		statuses = append(statuses, m)
	}
	for _, m := range statuses[:len(statuses)-1] {
		s.True(m.Alive)
	}
	s.False(statuses[len(statuses)-1].Alive)
}
