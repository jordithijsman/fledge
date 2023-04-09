package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types/manifest"
	"github.com/regclient/regclient/types/ref"
	ociv1ext "gitlab.ilabt.imec.be/fledge/service/pkg/oci/v1/ext"
	"path"
	"regexp"
)

func ImagesPath() string {
	return path.Join(RootPath(), "images")
}

func ImagePath(name string) string {
	name = regexp.MustCompile(":[0-9]{1,5}").ReplaceAllString(name, "")
	return path.Join(ImagesPath(), name)
}

//func ImagePull(ctx context.Context, name string) (ref.Ref, error) {
//	// Parse image source
//	src, err := ref.New(name)
//	if err != nil {
//		return ref.Ref{}, err
//	}
//	if src.Registry == "" {
//		return ref.Ref{}, fmt.Errorf("reference %s does not contain a valid registry", src.CommonName())
//	}
//	// Construct image target
//	tgt, err := ensureDirRef(src)
//	if err != nil {
//		return ref.Ref{}, err
//	}
//	// Check if image should be pulled
//	if !shouldPullImage(ctx, tgt) {
//		log.G(ctx).Infof("Image %s is present\n", tgt.CommonName())
//		return tgt, nil
//	}
//	// Copy image locally
//	client := regclient.New(regclient.WithDockerCreds())
//	imageOpts := []regclient.ImageOpts{
//		regclient.ImageWithPlatforms([]string{platforms.DefaultString()}),
//	}
//	if err = client.ImageCopy(ctx, src, tgt, imageOpts...); err != nil {
//		return ref.Ref{}, err
//	}
//	log.G(ctx).Infof("Pulled image %s to %s\n", src.CommonName(), tgt.CommonName())
//	return tgt, nil
//}

func ImageGetConfig(ctx context.Context, name string) (ociv1ext.Image, error) {
	// Parse image source
	src, err := ref.New(name)
	if err != nil {
		return ociv1ext.Image{}, err
	}
	if src.Registry == "" {
		return ociv1ext.Image{}, fmt.Errorf("reference %s does not contain a valid registry", src.CommonName())
	}
	// Retrieve manifest of the image
	client := regclient.New(regclient.WithDockerCreds())
	manifestDesc, err := client.ManifestGet(ctx, src)
	if err != nil {
		return ociv1ext.Image{}, err
	}
	// Retrieve config from the manifest
	if img, ok := manifestDesc.(manifest.Imager); ok {
		configDesc, err := img.GetConfig()
		if err != nil {
			return ociv1ext.Image{}, err
		}
		configBlob, err := client.BlobGet(ctx, src, configDesc)
		if err != nil {
			return ociv1ext.Image{}, err
		}
		var config ociv1ext.Image
		if err = json.NewDecoder(configBlob).Decode(&config); err != nil {
			return ociv1ext.Image{}, err
		}
		return config, nil
	}
	return ociv1ext.Image{}, fmt.Errorf("reference %s does not represent a valid image", src.CommonName())
}
