package docker

import (
	"context"
	"fmt"
	"io"

	registrytypes "github.com/moby/moby/api/types/registry"
	dockerapi "github.com/moby/moby/client"

	enginetypes "github.com/projecteru2/core/engine/types"
)

func (e *Engine) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	image = normalizeImage(image)
	imgListFilter := dockerapi.Filters{}
	imgListFilter.Add("reference", image)

	images, err := e.client.ImageList(ctx, dockerapi.ImageListOptions{Filters: imgListFilter})
	if err != nil {
		return nil, err
	}

	r := []*enginetypes.Image{}
	for _, image := range images.Items {
		i := &enginetypes.Image{
			ID:   image.ID,
			Tags: image.RepoTags,
		}
		r = append(r, i)
	}
	return r, nil
}

func (e *Engine) ImageRemove(ctx context.Context, image string, force, prune bool) ([]string, error) {
	opts := dockerapi.ImageRemoveOptions{
		Force:         force,
		PruneChildren: prune,
	}

	removed, err := e.client.ImageRemove(ctx, image, opts)
	r := []string{}
	if err != nil {
		return r, err
	}

	for _, item := range removed.Items {
		if item.Untagged != "" {
			r = append(r, item.Untagged)
		}
		if item.Deleted != "" {
			r = append(r, item.Deleted)
		}
	}

	return r, nil
}

func (e *Engine) ImagesPrune(ctx context.Context) error {
	_, err := e.client.ImagePrune(ctx, dockerapi.ImagePruneOptions{})
	return err
}

func (e *Engine) ImagePull(ctx context.Context, ref string, all bool) (io.ReadCloser, error) {
	auth, err := makeEncodedAuthConfigFromRemote(e.config.Docker.AuthConfigs, ref)
	if err != nil {
		return nil, err
	}
	pullOptions := dockerapi.ImagePullOptions{All: all, RegistryAuth: auth}
	return e.client.ImagePull(ctx, ref, pullOptions)
}

func (e *Engine) ImagePush(ctx context.Context, ref string) (io.ReadCloser, error) {
	auth, err := makeEncodedAuthConfigFromRemote(e.config.Docker.AuthConfigs, ref)
	if err != nil {
		return nil, err
	}
	pushOptions := dockerapi.ImagePushOptions{RegistryAuth: auth}
	return e.client.ImagePush(ctx, ref, pushOptions)
}

func (e *Engine) ImageBuild(ctx context.Context, input io.Reader, refs []string, platform string) (io.ReadCloser, error) {
	authConfigs := make(map[string]registrytypes.AuthConfig, len(e.config.Docker.AuthConfigs))
	for domain, conf := range e.config.Docker.AuthConfigs {
		b64auth, err := encodeAuthToBase64(conf)
		if err != nil {
			return nil, err
		}
		authConfigs[domain] = registrytypes.AuthConfig{
			Username: conf.Username,
			Password: conf.Password,
			Auth:     b64auth,
		}
	}
	buildOptions := dockerapi.ImageBuildOptions{
		Tags:           refs,
		SuppressOutput: false,
		NoCache:        true,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Platforms:      parsePlatform(platform),
		AuthConfigs:    authConfigs,
	}
	resp, err := e.client.ImageBuild(ctx, input, buildOptions)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (e *Engine) ImageBuildFromExist(ctx context.Context, ID string, refs []string, _ string) (imageID string, err error) {
	opts := dockerapi.ContainerCommitOptions{
		Reference: refs[0],
		Author:    "eru-core",
	}
	resp, err := e.client.ContainerCommit(ctx, ID, opts)
	if err != nil {
		return "", err
	}
	for _, ref := range refs[1:] {
		if _, err = e.client.ImageTag(ctx, dockerapi.ImageTagOptions{Source: resp.ID, Target: ref}); err != nil {
			return "", err
		}
	}
	return resp.ID, err
}

func (e *Engine) ImageBuildCachePrune(ctx context.Context, all bool) (uint64, error) {
	r, err := e.client.BuildCachePrune(ctx, dockerapi.BuildCachePruneOptions{All: all})
	if err != nil {
		return 0, err
	}
	return r.Report.SpaceReclaimed, nil
}

func (e *Engine) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	inspect, err := e.client.ImageInspect(ctx, image)
	if err != nil {
		return nil, err
	}
	return inspect.RepoDigests, nil
}

func (e *Engine) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	auth, err := makeEncodedAuthConfigFromRemote(e.config.Docker.AuthConfigs, image)
	if err != nil {
		return "", err
	}
	inspect, err := e.client.DistributionInspect(ctx, image, dockerapi.DistributionInspectOptions{EncodedRegistryAuth: auth})
	if err != nil {
		return "", err
	}
	remoteDigest := fmt.Sprintf("%s@%s", normalizeImage(image), inspect.Descriptor.Digest.String())
	return remoteDigest, nil
}
