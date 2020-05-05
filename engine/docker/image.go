package docker

import (
	"context"
	"fmt"
	"io"

	dockertypes "github.com/docker/docker/api/types"
	dockerfilters "github.com/docker/docker/api/types/filters"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

// ImageList list image
func (e *Engine) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	image = normalizeImage(image)
	imgListFilter := dockerfilters.NewArgs()
	imgListFilter.Add("reference", image) // 相同 repo 的image

	images, err := e.client.ImageList(ctx, dockertypes.ImageListOptions{Filters: imgListFilter})
	if err != nil {
		return nil, err
	}

	r := []*enginetypes.Image{}
	for _, image := range images {
		i := &enginetypes.Image{
			ID:   image.ID,
			Tags: image.RepoTags,
		}
		r = append(r, i)
	}
	return r, nil
}

// ImageRemove remove a image
func (e *Engine) ImageRemove(ctx context.Context, image string, force, prune bool) ([]string, error) {
	opts := dockertypes.ImageRemoveOptions{
		Force:         force,
		PruneChildren: prune,
	}

	removed, err := e.client.ImageRemove(ctx, image, opts)
	r := []string{}
	if err != nil {
		return r, err
	}

	for _, item := range removed {
		if item.Untagged != "" {
			r = append(r, item.Untagged)
		}
		if item.Deleted != "" {
			r = append(r, item.Deleted)
		}
	}

	return r, nil
}

// ImagesPrune prune images
func (e *Engine) ImagesPrune(ctx context.Context) error {
	_, err := e.client.ImagesPrune(ctx, dockerfilters.NewArgs())
	return err
}

// ImagePull pull Image
func (e *Engine) ImagePull(ctx context.Context, ref string, all bool) (io.ReadCloser, error) {
	auth, err := makeEncodedAuthConfigFromRemote(e.config.Docker.AuthConfigs, ref)
	if err != nil {
		return nil, err
	}
	pullOptions := dockertypes.ImagePullOptions{All: all, RegistryAuth: auth}
	return e.client.ImagePull(ctx, ref, pullOptions)
}

// ImagePush push image
func (e *Engine) ImagePush(ctx context.Context, ref string) (io.ReadCloser, error) {
	auth, err := makeEncodedAuthConfigFromRemote(e.config.Docker.AuthConfigs, ref)
	if err != nil {
		return nil, err
	}
	pushOptions := dockertypes.ImagePushOptions{RegistryAuth: auth}
	return e.client.ImagePush(ctx, ref, pushOptions)
}

// ImageBuild build image
func (e *Engine) ImageBuild(ctx context.Context, input io.Reader, refs []string) (io.ReadCloser, error) {
	authConfigs := map[string]dockertypes.AuthConfig{}
	for domain, conf := range e.config.Docker.AuthConfigs {
		b64auth, err := encodeAuthToBase64(conf)
		if err != nil {
			return nil, err
		}
		if _, ok := authConfigs[domain]; !ok {
			authConfigs[domain] = dockertypes.AuthConfig{
				Username: conf.Username,
				Password: conf.Password,
				Auth:     b64auth,
			}
		}
	}
	buildOptions := dockertypes.ImageBuildOptions{
		Tags:           refs,
		SuppressOutput: false,
		NoCache:        true,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		AuthConfigs:    authConfigs,
	}
	resp, err := e.client.ImageBuild(ctx, input, buildOptions)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ImageBuildFromExist commits image from running container
func (e *Engine) ImageBuildFromExist(ctx context.Context, ID, name string) (imageID string, err error) {
	return "", types.ErrEngineNotImplemented
}

// ImageBuildCachePrune prune build cache
func (e *Engine) ImageBuildCachePrune(ctx context.Context, all bool) (uint64, error) {
	r, err := e.client.BuildCachePrune(ctx, dockertypes.BuildCachePruneOptions{All: all})
	if err != nil {
		return 0, err
	}
	return r.SpaceReclaimed, nil
}

// ImageLocalDigests return image digests
func (e *Engine) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	inspect, _, err := e.client.ImageInspectWithRaw(ctx, image)
	if err != nil {
		return nil, err
	}
	return inspect.RepoDigests, nil
}

// ImageRemoteDigest return image digest at remote
func (e *Engine) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	auth, err := makeEncodedAuthConfigFromRemote(e.config.Docker.AuthConfigs, image)
	if err != nil {
		return "", err
	}
	inspect, err := e.client.DistributionInspect(ctx, image, auth)
	if err != nil {
		return "", err
	}
	remoteDigest := fmt.Sprintf("%s@%s", normalizeImage(image), inspect.Descriptor.Digest.String())
	return remoteDigest, nil
}
