package cocoon

import (
	"cmp"
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// partsMedia is the layer type of a disk image published as split qcow2 parts, the Windows shape.
	partsMedia = "application/vnd.cocoonstack.disk.qcow2.part"

	// importScript reassembles a parts artifact once; cocoon imports split parts as one disk.
	importScript = `set -e
bin=$1; ref=$2
if "$bin" image inspect "$ref" >/dev/null 2>&1; then exit 0; fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
oras pull "$ref" -o "$tmp" >/dev/null
"$bin" image import "$ref" "$tmp"/*.part
`

	orasProbe = `command -v oras >/dev/null 2>&1`

	noImages = "No images"
)

// cocoonImage is the part of cocoon's image JSON the engine reads.
type cocoonImage struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type layer struct {
	MediaType string `json:"mediaType"`
}

type manifest struct {
	Layers []layer `json:"layers"`
}

func (m *manifest) parts() bool {
	return slices.ContainsFunc(m.Layers, func(l layer) bool { return l.MediaType == partsMedia })
}

func (e *Engine) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	res, err := e.run(ctx, e.cocoon.Binary, "image", "list", "--format", formatJSON)
	if err != nil {
		return nil, err
	}
	images := []*enginetypes.Image{}
	if strings.HasPrefix(strings.TrimSpace(res.Stdout), noImages) {
		return images, nil
	}
	stored := []cocoonImage{}
	if err = json.Unmarshal([]byte(res.Stdout), &stored); err != nil {
		return nil, err
	}
	for _, entry := range stored {
		if strings.HasPrefix(entry.Name, image) {
			images = append(images, &enginetypes.Image{ID: entry.ID, Tags: []string{entry.Name}})
		}
	}
	return images, nil
}

func (e *Engine) ImageRemove(ctx context.Context, image string, _, _ bool) ([]string, error) {
	if _, err := e.run(ctx, e.cocoon.Binary, "image", "rm", image); err != nil {
		return nil, err
	}
	return []string{image}, nil
}

func (e *Engine) ImagesPrune(context.Context) error {
	return coretypes.ErrEngineNotImplemented
}

// ImagePull hands a registry ref or a cloud-image url to cocoon; a parts artifact goes through oras and import.
func (e *Engine) ImagePull(ctx context.Context, ref string, _ bool) (io.ReadCloser, error) {
	argv := []string{e.cocoon.Binary, "image", "pull", ref}
	if !enginetypes.IsURL(ref) && e.partsArtifact(ctx, ref) {
		argv = sshrunner.Shell(importScript, e.cocoon.Binary, ref)
	}
	res, err := e.run(ctx, argv...)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(res.Stdout)), nil
}

func (e *Engine) ImagePush(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.Wrap(coretypes.ErrEngineNotImplemented, "cocoon keeps snapshots on the node, there is no registry push")
}

func (e *Engine) ImageBuild(context.Context, io.Reader, []string, string) (io.ReadCloser, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (e *Engine) ImageBuildCachePrune(context.Context, bool) (uint64, error) {
	return 0, nil
}

func (e *Engine) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	res, err := e.call(ctx, e.cocoon.Binary, "image", "inspect", image)
	if err != nil {
		return nil, err
	}
	if res.Code != 0 {
		return nil, nil
	}
	stored := cocoonImage{}
	if err = json.Unmarshal([]byte(res.Stdout), &stored); err != nil {
		return nil, err
	}
	return []string{enginetypes.ImageDigest(image, stored.ID)}, nil
}

// ImageRemoteDigest asks the registry through oras; a cloud image url is its own digest.
func (e *Engine) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	if enginetypes.IsURL(image) {
		return image, nil
	}
	if !e.orasPresent(ctx) {
		return "", nil
	}
	res, err := e.run(ctx, "oras", "manifest", "fetch", "--descriptor", image)
	if err != nil {
		return "", err
	}
	digest, err := enginetypes.ParseDescriptor(res.Stdout)
	if err != nil {
		return "", err
	}
	return enginetypes.ImageDigest(image, digest), nil
}

func (e *Engine) ImageBuildFromExist(context.Context, string, []string, string) (string, error) {
	return "", errors.Wrap(coretypes.ErrEngineNotImplemented, "cocoon has no snapshot push")
}

func (e *Engine) BuildRefs(_ context.Context, opts *enginetypes.BuildRefOptions) []string {
	return e.config.Registry.BuildRefs(opts.Name, opts.Tags)
}

func (e *Engine) BuildContent(context.Context, coresource.Source, *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, coretypes.ErrEngineNotImplemented
}

// partsArtifact reads the manifest through oras; when oras cannot answer, cocoon pulls the ref itself.
func (e *Engine) partsArtifact(ctx context.Context, ref string) bool {
	if !e.orasPresent(ctx) {
		return false
	}
	res, err := e.call(ctx, "oras", "manifest", "fetch", ref)
	if err != nil || res.Code != 0 {
		return false
	}
	m := &manifest{}
	return json.Unmarshal([]byte(res.Stdout), m) == nil && m.parts()
}

func (e *Engine) orasPresent(ctx context.Context) bool {
	if e.hasOras.Load() {
		return true
	}
	probed := e.probe.DoChan("oras", func() (any, error) {
		probeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cmp.Or(e.config.ConnectionTimeout, time.Minute))
		defer cancel()
		res, err := e.call(probeCtx, sshrunner.Shell(orasProbe)...)
		found := err == nil && res.Code == 0
		if found {
			e.hasOras.Store(true)
		}
		return found, nil
	})
	select {
	case result := <-probed:
		return result.Val.(bool)
	case <-ctx.Done():
		return false
	}
}
