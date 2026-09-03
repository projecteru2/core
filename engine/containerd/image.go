package containerd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/pkg/labels"
	"github.com/containerd/containerd/v2/pkg/rootfs"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
)

const commitAuthor = "eru-core"

var hubAliases = []string{"docker.io", "index.docker.io", "registry-1.docker.io", "https://index.docker.io/v1/"}

func (e *Engine) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	listed, err := e.client.ListImages(ctx)
	if err != nil {
		return nil, err
	}
	name, _ := enginetypes.SplitRef(normalizeRef(image))
	r := []*enginetypes.Image{}
	for _, item := range listed {
		if !strings.HasPrefix(item.Name(), name) {
			continue
		}
		r = append(r, &enginetypes.Image{ID: item.Target().Digest.String(), Tags: []string{item.Name()}})
	}
	return r, nil
}

func (e *Engine) ImageRemove(ctx context.Context, image string, _, _ bool) ([]string, error) {
	image = normalizeRef(image)
	if err := e.client.ImageService().Delete(ctx, image, images.SynchronousDelete()); err != nil {
		if cerrdefs.IsNotFound(err) {
			return []string{}, nil
		}
		return nil, err
	}
	return []string{image}, nil
}

// ImagesPrune drops every image no container is built on; containerd collects the blobs itself.
func (e *Engine) ImagesPrune(ctx context.Context) error {
	listed, err := e.client.ListImages(ctx)
	if err != nil {
		return err
	}
	used, err := e.client.Containers(ctx)
	if err != nil {
		return err
	}
	inUse := map[string]struct{}{}
	for _, found := range used {
		info, infoErr := found.Info(ctx, client.WithoutRefreshedMetadata)
		if infoErr != nil {
			return infoErr
		}
		inUse[info.Image] = struct{}{}
	}
	logger := log.WithFunc("engine.containerd.ImagesPrune")
	for _, item := range listed {
		if _, ok := inUse[item.Name()]; ok {
			continue
		}
		if err = e.client.ImageService().Delete(ctx, item.Name()); err != nil && !cerrdefs.IsNotFound(err) {
			logger.Errorf(ctx, err, "remove image %s", item.Name())
		}
	}
	return nil
}

func (e *Engine) ImagePull(ctx context.Context, ref string, _ bool) (io.ReadCloser, error) {
	if _, err := e.client.Pull(ctx, normalizeRef(ref), client.WithPullUnpack, client.WithResolver(e.resolver())); err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader("")), nil
}

// ImagePush has nothing to send after a solve: BuildKit's exporter already pushed the image.
func (e *Engine) ImagePush(ctx context.Context, ref string) (io.ReadCloser, error) {
	ref = normalizeRef(ref)
	image, err := e.client.GetImage(ctx, ref)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return io.NopCloser(strings.NewReader("")), nil
		}
		return nil, err
	}
	if err = e.client.Push(ctx, ref, image.Target(), client.WithResolver(e.resolver())); err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (e *Engine) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	image = normalizeRef(image)
	found, err := e.client.GetImage(ctx, image)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	name, _ := enginetypes.SplitRef(image)
	return []string{name + "@" + found.Target().Digest.String()}, nil
}

func (e *Engine) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	image = normalizeRef(image)
	_, desc, err := e.resolver().Resolve(ctx, image)
	if err != nil {
		return "", err
	}
	name, _ := enginetypes.SplitRef(image)
	return name + "@" + desc.Digest.String(), nil
}

// ImageBuildFromExist captures the workload's writable snapshot as one more layer on its image.
func (e *Engine) ImageBuildFromExist(ctx context.Context, ID string, refs []string, _ string) (string, error) {
	found, err := e.container(ctx, ID)
	if err != nil {
		return "", err
	}
	info, err := found.Info(ctx, client.WithoutRefreshedMetadata)
	if err != nil {
		return "", err
	}
	task, err := optionalTask(ctx, found)
	if err != nil {
		return "", err
	}
	if task != nil {
		if err = task.Pause(ctx); err != nil {
			return "", err
		}
		defer func() {
			if resumeErr := task.Resume(ctx); resumeErr != nil {
				log.WithFunc("engine.containerd.ImageBuildFromExist").Error(ctx, resumeErr, "resume the paused workload")
			}
		}()
	}

	ctx, release, err := e.client.WithLease(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		if releaseErr := release(ctx); releaseErr != nil {
			log.WithFunc("engine.containerd.ImageBuildFromExist").Error(ctx, releaseErr, "release the commit lease")
		}
	}()

	target, err := e.commit(ctx, info)
	if err != nil {
		return "", err
	}
	for _, ref := range refs {
		if err = createImageReference(ctx, e.client.ImageService(), images.Image{Name: ref, Target: target}); err != nil {
			return "", err
		}
	}
	return target.Digest.String(), nil
}

// commit writes the diff of the workload's snapshot against its image as a new manifest.
func (e *Engine) commit(ctx context.Context, info containers.Container) (ocispec.Descriptor, error) {
	empty := ocispec.Descriptor{}
	store := e.client.ContentStore()
	base, err := e.client.GetImage(ctx, info.Image)
	if err != nil {
		return empty, err
	}
	manifest, err := images.Manifest(ctx, store, base.Target(), platforms.Only(e.platform))
	if err != nil {
		return empty, err
	}
	config := ocispec.Image{}
	if err = readBlob(ctx, store, manifest.Config, &config); err != nil {
		return empty, err
	}

	layer, err := rootfs.CreateDiff(ctx, info.SnapshotKey, e.client.SnapshotService(info.Snapshotter), e.client.DiffService())
	if err != nil {
		return empty, err
	}
	diffID, err := uncompressedDigest(ctx, store, layer)
	if err != nil {
		return empty, err
	}
	config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, diffID)
	created := time.Now().UTC()
	config.History = append(config.History, ocispec.History{Created: &created, CreatedBy: commitAuthor})
	configDesc, err := writeBlob(ctx, store, ocispec.MediaTypeImageConfig, config, nil)
	if err != nil {
		return empty, err
	}

	manifest.Config = configDesc
	manifest.Layers = append(manifest.Layers, layer)
	manifest.MediaType = ocispec.MediaTypeImageManifest
	return writeBlob(ctx, store, ocispec.MediaTypeImageManifest, manifest, gcRefs(manifest))
}

// imageConfig reads what the image declares straight from its config blob.
func (e *Engine) imageConfig(ctx context.Context, image client.Image) (*ocispec.ImageConfig, error) {
	desc, err := image.Config(ctx)
	if err != nil {
		return nil, err
	}
	config := ocispec.Image{}
	if err = readBlob(ctx, image.ContentStore(), desc, &config); err != nil {
		return nil, err
	}
	return &config.Config, nil
}

// resolver authenticates every registry request against the configured credentials; its push tracker remembers blobs per digest, not per repository, so it must not outlive one call.
func (e *Engine) resolver() remotes.Resolver {
	auths := e.config.Registry.Auths
	plainHTTP := e.config.Registry.PlainHTTP
	return docker.NewResolver(docker.ResolverOptions{
		Hosts: docker.ConfigureDefaultRegistries(
			docker.WithAuthorizer(docker.NewDockerAuthorizer(docker.WithAuthCreds(func(host string) (string, string, error) {
				auth, ok := registryAuth(auths, host)
				if !ok {
					return "", "", nil
				}
				return auth.Username, auth.Password, nil
			}))),
			docker.WithPlainHTTP(func(host string) (bool, error) {
				return slices.Contains(plainHTTP, host), nil
			}),
		),
	})
}

func createImageReference(ctx context.Context, store images.Store, image images.Image) error {
	if _, err := store.Create(ctx, image); err != nil {
		if !cerrdefs.IsAlreadyExists(err) {
			return err
		}
		_, err = store.Update(ctx, image, "target")
		return err
	}
	return nil
}

func uncompressedDigest(ctx context.Context, store content.Store, layer ocispec.Descriptor) (digest.Digest, error) {
	info, err := store.Info(ctx, layer.Digest)
	if err != nil {
		return "", err
	}
	uncompressed, ok := info.Labels[labels.LabelUncompressed]
	if !ok {
		return layer.Digest, nil
	}
	return digest.Parse(uncompressed)
}

func gcRefs(manifest ocispec.Manifest) map[string]string {
	refs := map[string]string{"containerd.io/gc.ref.content.config": manifest.Config.Digest.String()}
	for i, layer := range manifest.Layers {
		refs["containerd.io/gc.ref.content.l."+strconv.Itoa(i)] = layer.Digest.String()
	}
	return refs
}

func registryAuth(auths map[string]coretypes.AuthConfig, host string) (coretypes.AuthConfig, bool) {
	if auth, ok := auths[host]; ok {
		return auth, true
	}
	if !slices.Contains(hubAliases, host) {
		return coretypes.AuthConfig{}, false
	}
	for _, alias := range hubAliases {
		if auth, ok := auths[alias]; ok {
			return auth, true
		}
	}
	return coretypes.AuthConfig{}, false
}

func readBlob(ctx context.Context, store content.Provider, desc ocispec.Descriptor, target any) error {
	blob, err := content.ReadBlob(ctx, store, desc)
	if err != nil {
		return err
	}
	return json.Unmarshal(blob, target)
}

func writeBlob(ctx context.Context, store content.Ingester, mediaType string, payload any, refs map[string]string) (ocispec.Descriptor, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	desc := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(body),
		Size:      int64(len(body)),
	}
	opts := []content.Opt{}
	if len(refs) > 0 {
		opts = append(opts, content.WithLabels(refs))
	}
	if err = content.WriteBlob(ctx, store, desc.Digest.String(), bytes.NewReader(body), desc, opts...); err != nil {
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}
