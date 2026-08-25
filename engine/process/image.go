package process

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	digestFile   = ".digest"
	bundleMedia  = "application/vnd.eru.process.bundle.v1+tar"
	existArchive = "exist.tar"

	listScript = `ls -1 "$1" 2>/dev/null || true`

	pullScript = `set -e
ref=$1; dir=$2
mkdir -p "$dir"
oras pull "$ref" -o "$dir"
oras manifest fetch --descriptor "$ref" > "$dir/.digest"
`
	existScript = `set -e
unit=$1; dir=$2; ref=$3; layer=$4
systemctl freeze "$unit"
tar -C "$dir/upper" -cf "$layer" . || { systemctl thaw "$unit"; exit 1; }
systemctl thaw "$unit"
oras push "$ref" "$layer:` + bundleMedia + `" >/dev/null
rm -f "$layer"
oras manifest fetch --descriptor "$ref"
`
)

func (e *Engine) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	res, err := e.run(ctx, shell(listScript, filepath.Join(e.root, imageCache))...)
	if err != nil {
		return nil, err
	}
	name, _ := splitRef(image)
	images := []*enginetypes.Image{}
	for entry := range strings.FieldsSeq(res.Stdout) {
		ref, unescapeErr := url.PathUnescape(entry)
		if unescapeErr != nil || !strings.HasPrefix(ref, name) {
			continue
		}
		images = append(images, &enginetypes.Image{ID: ref, Tags: []string{ref}})
	}
	return images, nil
}

func (e *Engine) ImageRemove(ctx context.Context, image string, _, _ bool) ([]string, error) {
	if _, err := e.run(ctx, "rm", "-rf", imageDir(e.root, image)); err != nil {
		return nil, err
	}
	return []string{image}, nil
}

func (e *Engine) ImagesPrune(ctx context.Context) error {
	_, err := e.run(ctx, "rm", "-rf", filepath.Join(e.root, imageCache))
	return err
}

func (e *Engine) ImagePull(ctx context.Context, ref string, _ bool) (io.ReadCloser, error) {
	res, err := e.run(ctx, shell(pullScript, ref, imageDir(e.root, ref))...)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(res.Stdout)), nil
}

// ImagePush has nothing left to send: ImageBuildFromExist pushes the artifact as it builds it.
func (e *Engine) ImagePush(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (e *Engine) ImageBuild(context.Context, io.Reader, []string, string) (io.ReadCloser, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (e *Engine) ImageBuildCachePrune(context.Context, bool) (uint64, error) {
	return 0, nil
}

func (e *Engine) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	res, err := e.call(ctx, "cat", filepath.Join(imageDir(e.root, image), digestFile))
	if err != nil {
		return nil, err
	}
	if res.Code != 0 {
		return nil, nil
	}
	digest, err := parseDescriptor(res.Stdout)
	if err != nil {
		return nil, err
	}
	return []string{imageDigest(image, digest)}, nil
}

func (e *Engine) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	res, err := e.run(ctx, "oras", "manifest", "fetch", "--descriptor", image)
	if err != nil {
		return "", err
	}
	digest, err := parseDescriptor(res.Stdout)
	if err != nil {
		return "", err
	}
	return imageDigest(image, digest), nil
}

func (e *Engine) ImageBuildFromExist(ctx context.Context, ID string, refs []string, _ string) (string, error) {
	record, err := e.workloadMeta(ctx, ID)
	if err != nil {
		return "", err
	}
	if record.RootDirectory == "" {
		return "", errors.Wrapf(coretypes.ErrEngineNotImplemented, "workload %s has no overlay to capture", ID)
	}

	dir := workloadDir(e.root, ID)
	res, err := e.run(ctx, shell(existScript, unitName(ID), dir, refs[0], filepath.Join(dir, existArchive))...)
	if err != nil {
		return "", err
	}
	for _, ref := range refs[1:] {
		_, tag := splitRef(ref)
		if _, err = e.run(ctx, "oras", "tag", refs[0], tag); err != nil {
			return "", err
		}
	}
	return parseDescriptor(res.Stdout)
}

func parseDescriptor(out string) (string, error) {
	descriptor := struct {
		Digest string `json:"digest"`
	}{}
	if err := json.Unmarshal([]byte(out), &descriptor); err != nil {
		return "", err
	}
	return descriptor.Digest, nil
}

func imageDigest(image, digest string) string {
	name, _ := splitRef(image)
	return name + "@" + digest
}
