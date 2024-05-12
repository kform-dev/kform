package address

import (
	"context"
	"fmt"
	"strings"

	"github.com/henderiw/logger/log"
)

func (r Releases) GetRelease(version string) *Release {
	for _, release := range r {
		if strings.Contains(release.TagName, version) {
			return &release
		}
	}
	return nil
}

type Images []Image

type Image struct {
	Name    string
	Version string
	Platform
	URL string
}

func (r *Release) GetImageData(ctx context.Context) (Images, error) {
	log := log.FromContext(ctx)
	images := Images{}
	for _, asset := range r.Assets {
		log.Info("asset info", "name", asset.Name, "contentType", asset.ContentType, "state", asset.State)
		if asset.ContentType == "application/gzip" && asset.State == "uploaded" {
			rawAssetName := strings.TrimSuffix(asset.Name, ".tar.gz")
			split := strings.Split(rawAssetName, "_")
			if len(split) != 3 {
				log.Error("wrong release name: expecting <name>_<os>_<arch>", "got", rawAssetName)
				return images, fmt.Errorf("wrong release name: expecting <name>_<os>_<arch>, got: %s", rawAssetName)
			}
			images = append(images, Image{
				Name: asset.Name,
				Platform: Platform{
					OS:   split[1],
					Arch: split[2],
				},
				URL: asset.BrowserDownloadURL,
			})
		}
	}
	return images, nil
}
