package containerd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	bkclient "github.com/moby/buildkit/client"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

var errSolveFailed = errors.New("solve failed")

func TestFrontendAttrsPinThePlatformOnlyWhenAsked(t *testing.T) {
	attrs := frontendAttrs("linux/arm64")
	if attrs["platform"] != "linux/arm64" || attrs["filename"] != dockerfileName {
		t.Errorf("got %+v, want the platform and the Dockerfile name", attrs)
	}
	if _, ok := frontendAttrs("")["platform"]; ok {
		t.Error("an empty platform must not pin the solve")
	}
}

func TestVertexStatusReadsTheGraph(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		vertex *bkclient.Vertex
		want   string
	}{
		{"pending", &bkclient.Vertex{}, "pending"},
		{"running", &bkclient.Vertex{Started: &now}, "running"},
		{"finished", &bkclient.Vertex{Started: &now, Completed: &now}, "finished"},
		{"cached", &bkclient.Vertex{Cached: true, Completed: &now}, "cached"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vertexStatus(tt.vertex); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStreamSolveStatusRendersBuildMessages(t *testing.T) {
	status := make(chan *bkclient.SolveStatus, 2)
	status <- &bkclient.SolveStatus{Vertexes: []*bkclient.Vertex{{Name: "[1/2] FROM alpine"}}}
	status <- &bkclient.SolveStatus{Logs: []*bkclient.VertexLog{{Data: []byte("hello\n")}}}
	close(status)

	out := &strings.Builder{}
	if err := streamSolveStatus(status, out); err != nil {
		t.Fatalf("stream: %v", err)
	}

	messages := decodeBuildMessages(t, out.String())
	if len(messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(messages))
	}
	if messages[0].Stream != "[1/2] FROM alpine\n" || messages[0].Status != "pending" {
		t.Errorf("got %+v, want the vertex name and its status", messages[0])
	}
	if messages[1].Stream != "hello\n" {
		t.Errorf("got %q, want the vertex log", messages[1].Stream)
	}
}

func TestWriteSolveErrorEndsTheStream(t *testing.T) {
	out := &strings.Builder{}
	if err := writeSolveError(out, errSolveFailed); err != nil {
		t.Fatalf("write: %v", err)
	}

	messages := decodeBuildMessages(t, out.String())
	if len(messages) != 1 || messages[0].Error != errSolveFailed.Error() {
		t.Fatalf("got %+v, want the solve error", messages)
	}
	if messages[0].ErrorDetail.Code != -1 {
		t.Errorf("got %d, want -1", messages[0].ErrorDetail.Code)
	}
}

func TestWriteSolveErrorSaysNothingOnSuccess(t *testing.T) {
	out := &strings.Builder{}
	if err := writeSolveError(out, nil); err != nil || out.Len() != 0 {
		t.Errorf("got %q %v, want an empty tail", out.String(), err)
	}
}

func TestMakeMainPartRendersOneStage(t *testing.T) {
	build := &enginetypes.Build{
		Base:     "alpine:3.20",
		Dir:      "/srv",
		Envs:     map[string]string{"MODE": "prod"},
		Commands: []string{"make"},
	}

	got, err := makeMainPart(build, "FROM alpine:3.20 as build", []string{"RUN make"}, []string{"COPY --from=deps /a /b"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{"FROM alpine:3.20 as build", `ENV MODE "prod"`, "WORKDIR /srv", "COPY --from=deps /a /b", "RUN make"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, want it to contain %q", got, want)
		}
	}
}

func TestMakeUserPartCreatesTheDeployUser(t *testing.T) {
	got, err := makeUserPart(&enginetypes.BuildContentOptions{User: "eru", UID: 1023})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(got, "USER eru") || !strings.Contains(got, "eru::1023:1023:") {
		t.Errorf("got %q, want the user added and selected", got)
	}
}

func decodeBuildMessages(t *testing.T, body string) []*coretypes.BuildImageMessage {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(body))
	messages := []*coretypes.BuildImageMessage{}
	for decoder.More() {
		message := &coretypes.BuildImageMessage{}
		if err := decoder.Decode(message); err != nil {
			t.Fatalf("decode: %v", err)
		}
		messages = append(messages, message)
	}
	return messages
}
