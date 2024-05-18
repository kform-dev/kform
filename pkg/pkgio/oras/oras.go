package oras

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform/pkg/pkgio/oci"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	credentials "github.com/oras-project/oras-credentials-go"
	"github.com/pkg/errors"

	//"oras.land/oras-go/pkg/auth"
	//dockerauth "oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"

	//"oras.land/oras-go/v2/oras"
	"github.com/henderiw/store"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const (
	//PackageMetaLayerMediaType  = "application/vnd.cncf.kform.package.meta.v1.tar+gzip"
	//PackageImageLayerMediaType = "application/vnd.cncf.kform.package.image.v1.tar+gzip"
	PackageLayerMediaType = "application/vnd.cncf.kform.package.v1.tar+gzip"
	ModuleMediaType       = "application/vnd.cncf.kform.module.v1+json"
	ProviderMediaType     = "application/vnd.cncf.kform.provider.v1+json"
)

func EmptyCredential(ctx context.Context, hostport string) (auth.Credential, error) {
	return auth.EmptyCredential, nil
}

func DefaultCredential(registry string) auth.CredentialFunc {
	s, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return EmptyCredential
	}
	return func(ctx context.Context, registry string) (auth.Credential, error) {
		registry = credentials.ServerAddressFromHostname(registry)
		if registry == "" {
			return auth.EmptyCredential, nil
		}
		return s.Get(ctx, registry)
	}
}

type Tags []string

func GetTags(ctx context.Context, ref string) (Tags, error) {
	tags := Tags{}
	fmt.Println("ref", ref)
	target, err := GetRepository(ctx, ref)
	if err != nil {
		return tags, errors.Wrap(err, "cannot get repository")
	}
	fmt.Println("got target", target)
	target.Tags(ctx, "", func(t []string) error {
		fmt.Println("got tags", t)
		tags = append(tags, t...)
		return nil
	})
	return tags, nil
}

func GetRepository(ctx context.Context, ref string) (registry.Repository, error) {
	parsedRef, err := registry.ParseReference(ref)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse reference")
	}
	reg, err := remote.NewRegistry(parsedRef.Registry)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create registry")
	}

	reg.Client = &auth.Client{
		Credential: DefaultCredential(parsedRef.Registry),
		Header: http.Header{
			"User-Agent": {"kform"},
		},
	}
	return reg.Repository(ctx, parsedRef.Repository)
}

func Push(ctx context.Context, kind string, ref string, pkgData []byte) error {
	log := log.FromContext(ctx).With("ref", ref)
	log.Info("pushing package")
	// parse the reference
	parsedRef, err := registry.ParseReference(ref)
	if err != nil {
		return errors.Wrap(err, "cannot parse reference")
	}
	// src -> memory
	src := memory.New()
	// dst -> registry
	dst, err := GetRepository(ctx, ref)
	if err != nil {
		return errors.Wrap(err, "cannot get remote registry")
	}

	// collect layer descriptors and artifact type based on package type (provider/module)
	layerDescriptors := []ocispecv1.Descriptor{}
	artifactType := ModuleMediaType
	pkgDescriptor, err := pushBlob(ctx, PackageLayerMediaType, pkgData, src)
	if err != nil {
		return err
	}
	log.Info("pkg descriptor", "data", len(pkgDescriptor.Data))
	layerDescriptors = append(layerDescriptors, pkgDescriptor)
	/*
		if kind == v1alpha1.PkgKindProvider {
			artifactType = ProviderMediaType
			//log.Info("image add layer", "data", len(imgData))
			imageDescriptor, err := pushBlobReader(ctx, PackageImageLayerMediaType, img, src)
			if err != nil {
				return err
			}
			log.Info("image descriptor", "data", len(imageDescriptor.Data))
			layerDescriptors = append(layerDescriptors, imageDescriptor)
		}
	*/
	// generate manifest and push from src (memory store) to dst (remote registry)
	manifestDesc, err := oras.PackManifest(
		ctx,
		src,
		oras.PackManifestVersion1_1_RC4,
		artifactType,
		oras.PackManifestOptions{
			Layers:              layerDescriptors,
			ManifestAnnotations: map[string]string{},
		},
	)
	if err != nil {
		return err
	}
	err = src.Tag(ctx, manifestDesc, parsedRef.Reference)
	if err != nil {
		panic(err)
	}
	log.Info("pushed package before", "digest", manifestDesc.Digest)

	desc, err := oras.Copy(ctx, src, parsedRef.Reference, dst, "", oras.DefaultCopyOptions)
	if err != nil {
		return err
	}
	log.Info("pushed package succeeded", "digest", desc.Digest)

	return nil
}

func pushBlob(ctx context.Context, mediaType string, blob []byte, target oras.Target) (ocispecv1.Descriptor, error) {
	desc := content.NewDescriptorFromBytes(mediaType, blob)
	return desc, target.Push(ctx, desc, bytes.NewReader(blob)) // Push the blob to the registry target
}

/*
func pushBlobReader(ctx context.Context, mediaType string, blob io.Reader, target oras.Target) (ocispecv1.Descriptor, error) {
	desc := content.NewDescriptorFromBytes(mediaType, []byte{})
	return desc, target.Push(ctx, desc, blob) // Push the blob to the registry target
}
*/

func Pull(ctx context.Context, ref string, data store.Storer[[]byte]) error {
	log := log.FromContext(ctx).With("ref", ref)
	log.Info("pulling package")
	// dst -> memory
	dst := memory.New()
	// src -> registry
	src, err := GetRepository(ctx, ref)
	if err != nil {
		return errors.Wrap(err, "cannot get remote repo")
	}

	desc, err := oras.Copy(ctx, src, ref, dst, "", oras.DefaultCopyOptions)
	if err != nil {
		return errors.Wrap(err, "cannot copy")
	}
	if err := mem2file(ctx, ref, dst, desc, data); err != nil {
		return errors.Wrap(err, "cannot copy memrfile")
	}
	log.Info("pulled package successfully", "digest", desc.Digest)
	return nil
}

func mem2file(ctx context.Context, ref string, dst *memory.Store, desc ocispecv1.Descriptor, data store.Storer[[]byte]) error {
	log := log.FromContext(ctx).With("ref", ref)
	rc, err := dst.Fetch(ctx, desc)
	if err != nil {
		return errors.Wrap(err, "cannot fetch package from memory")
	}
	var manifest ocispecv1.Manifest
	if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
		return errors.Wrap(err, "cannot decode json")
	}
	defer rc.Close()

	log.Info("manifest", "mediaType", manifest.MediaType, "artifactType", manifest.ArtifactType)

	for _, layer := range manifest.Layers {
		log.Info("layer", "mediaType", layer.MediaType, "artifactType", layer.ArtifactType)
		rc, err := dst.Fetch(ctx, layer)
		if err != nil {
			return errors.Wrap(err, "cannot fetch layer")
		}
		if err = oci.TgzReader(ctx, rc, data); err != nil {
			log.Error("cannot read file", "err", err.Error())
			continue
			//return errors.Wrap(err, "cannot read tar.gz")
		}
		if err := rc.Close(); err != nil {
			log.Error("cannot close rc", "err", err.Error())
			continue
		}
	}
	return nil
}

//https://ghcr.io/kformdev/provider-resourcebackend/provider-resourcebackend/

/*
func (c *Client) Pull(ctx context.Context, ref string) (*Result, error) {
	parsedRef, err := registry.ParseReference(ref)
	if err != nil {
		return nil, err
	}
	memoryStore := content.NewMemory()
	allowedMediaTypes := []string{
		PackageMetaLayerMediaType,
		PackageImageLayerMediaType,
		ProviderMediaType,
		ModuleMediaType,
	}
	minNumDescriptors := 2

	descriptors := []ocispecv1.Descriptor{}
	layers := []ocispecv1.Descriptor{}
	remotesResolver, err := c.resolver(parsedRef)
	if err != nil {
		return nil, err
	}
	registryStore := content.Registry{Resolver: remotesResolver}

	manifest, err := oras.Copy(context.Background(), registryStore, parsedRef.String(), memoryStore, "",
		oras.WithPullEmptyNameAllowed(),
		oras.WithAllowedMediaTypes(allowedMediaTypes),
		oras.WithLayerDescriptors(func(l []ocispecv1.Descriptor) {
			layers = l
		}))
	if err != nil {
		return nil, err
	}

	descriptors = append(descriptors, manifest)
	descriptors = append(descriptors, layers...)

	fmt.Println("descriptor length", len(descriptors))
	if len(descriptors) < minNumDescriptors {
		return nil, fmt.Errorf("manifest does not contain minimum number of descriptors (%d), descriptors found: %d",
			minNumDescriptors, len(descriptors))
	}
	var configDescriptor *ocispecv1.Descriptor
	var pkgMetaDescriptor *ocispecv1.Descriptor
	var imageDescriptor *ocispecv1.Descriptor
	for _, descriptor := range descriptors {
		d := descriptor
		switch d.MediaType {
		case ProviderMediaType, ModuleMediaType:
			configDescriptor = &d
		case PackageMetaLayerMediaType:
			pkgMetaDescriptor = &d
		case PackageImageLayerMediaType:
			imageDescriptor = &d
		case manifest.MediaType:
		default:
			fmt.Println("unexpected descriptor", d.MediaType, d.Digest.String(), d.Size)
			if _, data, ok := memoryStore.Get(d); !ok {
				return nil, fmt.Errorf("unable to retrieve config with digest %s", d.Digest.String())
			} else {
				fmt.Println("unexpected data", string(data))
			}
		}
	}
	fmt.Println("ArtifactType:", manifest.ArtifactType)
	result := &Result{
		Manifest: &descriptorSummary{
			Digest: manifest.Digest.String(),
			Size:   manifest.Size,
		},
		Config: &descriptorSummary{
			Digest: configDescriptor.Digest.String(),
			Size:   configDescriptor.Size,
		},
		PkgMeta: &descriptorSummary{
			Digest: pkgMetaDescriptor.Digest.String(),
			Size:   pkgMetaDescriptor.Size,
		},
		Image: &descriptorSummary{
			Digest: imageDescriptor.Digest.String(),
			Size:   imageDescriptor.Size,
		},
		Ref: parsedRef.String(),
	}
	fmt.Fprintf(c.out, "Pulled: %s\n", result.Ref)
	fmt.Fprintf(c.out, "Digest: %s\n", result.Manifest.Digest)

	if _, manifestData, ok := memoryStore.Get(manifest); !ok {
		return nil, fmt.Errorf("unable to retrieve manifest blob with digest %s", manifest.Digest)
	} else {
		result.Manifest.Data = manifestData
	}
	if _, configData, ok := memoryStore.Get(*configDescriptor); !ok {
		return nil, fmt.Errorf("unable to retrieve config with digest %s", configDescriptor.Digest)
	} else {
		result.Config.Data = configData
	}
	if _, pkgMetaData, ok := memoryStore.Get(*pkgMetaDescriptor); !ok {
		return nil, fmt.Errorf("unable to retrieve pkgMetaData with digest %s", pkgMetaDescriptor.Digest)
	} else {
		result.PkgMeta.Data = pkgMetaData
	}
	if _, imageData, ok := memoryStore.Get(*imageDescriptor); !ok {
		return nil, fmt.Errorf("unable to retrieve image with digest %s", imageDescriptor.Digest)
	} else {
		result.Image.Data = imageData
	}

	fmt.Println("ref", result.Ref)
	fmt.Println("manifest", string(result.Manifest.Data))
	fmt.Println("config", string(result.Config.Data))
	//fmt.Println("schemas", string(result.Schemas.Data))
	fmt.Println("image", string(result.Image.Data))


	//	if _, err := oci.ReadTgz(result.Schemas.Data); err != nil {
	//		return nil, err
	//	}

	return result, nil
}
*/
