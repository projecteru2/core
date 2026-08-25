package process

import (
	"context"
	"io"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/distribution/reference"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	digestFile   = ".digest"
	bundleMedia  = "application/vnd.eru.process.bundle.v1+tar"
	existArchive = "exist.tar"

	listScript = `ls -1 "$1" 2>/dev/null || true`

	// oras leaves a bundle layer packed; an artifact pushed as a directory arrives unpacked
	unpackFunc = `unpack() {
for archive in "$1"/*.tar; do
[ -f "$archive" ] || continue
tar -C "$1" -xf "$archive"
rm -f "$archive"
done
}
`

	pullScript = "set -e\n" + unpackFunc + `ref=$1; dir=$2; shift 2
rm -rf "$dir"
mkdir -p "$dir"
oras pull "$ref" -o "$dir" "$@"
unpack "$dir"
oras manifest fetch --descriptor "$ref" "$@" > "$dir/` + digestFile + `"
`

	existScript = `set -e
unit=$1; dir=$2; ref=$3; layer=$4; shift 4
mounted=0
if ! mountpoint -q "$dir/merged"; then
mount -t overlay overlay -o "lowerdir=$dir/lower,upperdir=$dir/upper,workdir=$dir/work" "$dir/merged"
mounted=1
fi
cleanup() {
systemctl thaw "$unit" >/dev/null 2>&1 || true
if [ "$mounted" = 1 ]; then umount -l "$dir/merged" >/dev/null 2>&1 || true; fi
return 0
}
trap cleanup EXIT
systemctl freeze "$unit" >/dev/null 2>&1 || true
tar -C "$dir/merged" -cf "$layer" .
systemctl thaw "$unit" >/dev/null 2>&1 || true
oras push --disable-path-validation --artifact-type ` + bundleMedia + ` "$ref" "$layer:` + bundleMedia + `" "$@" >/dev/null
rm -f "$layer"
oras manifest fetch --descriptor "$ref" "$@"
`
)

func (e *Engine) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	res, err := e.run(ctx, sshrunner.Shell(listScript, filepath.Join(e.root, imageCache))...)
	if err != nil {
		return nil, err
	}
	name, _ := enginetypes.SplitRef(image)
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
	res, err := e.run(ctx, sshrunner.Shell(pullScript, slices.Concat([]string{ref, imageDir(e.root, ref)}, e.registryFlags(ref))...)...)
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
	digest, err := enginetypes.ParseDescriptor(res.Stdout)
	if err != nil {
		return nil, err
	}
	return []string{enginetypes.ImageDigest(image, digest)}, nil
}

func (e *Engine) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	res, err := e.run(ctx, slices.Concat([]string{"oras", "manifest", "fetch", "--descriptor", image}, e.registryFlags(image))...)
	if err != nil {
		return "", err
	}
	digest, err := enginetypes.ParseDescriptor(res.Stdout)
	if err != nil {
		return "", err
	}
	return enginetypes.ImageDigest(image, digest), nil
}

func (e *Engine) ImageBuildFromExist(ctx context.Context, ID string, refs []string, _ string) (string, error) {
	record, _, err := e.workloadMeta(ctx, ID)
	if err != nil {
		return "", err
	}
	if record.RootDirectory == "" {
		return "", errors.Wrapf(coretypes.ErrEngineNotImplemented, "workload %s has no overlay to capture", ID)
	}

	dir := workloadDir(e.root, ID)
	existArgs := slices.Concat([]string{unitName(ID), dir, refs[0], filepath.Join(dir, existArchive)}, e.registryFlags(refs[0]))
	res, err := e.run(ctx, sshrunner.Shell(existScript, existArgs...)...)
	if err != nil {
		return "", err
	}
	for _, ref := range refs[1:] {
		_, tag := enginetypes.SplitRef(ref)
		if _, err = e.run(ctx, slices.Concat([]string{"oras", "tag", refs[0], tag}, e.registryFlags(ref))...); err != nil {
			return "", err
		}
	}
	return enginetypes.ParseDescriptor(res.Stdout)
}

// registryFlags authenticates oras against the ref's registry.
func (e *Engine) registryFlags(ref string) []string {
	host := registryHost(ref)
	flags := []string{}
	if auth, ok := e.config.Registry.Auths[host]; ok {
		flags = append(flags, "--username", auth.Username, "--password", auth.Password)
	}
	if slices.Contains(e.config.Registry.PlainHTTP, host) {
		flags = append(flags, "--plain-http")
	}
	return flags
}

func registryHost(ref string) string {
	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return ""
	}
	return reference.Domain(named)
}
