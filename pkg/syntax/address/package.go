package address

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apparentlymart/go-versions/versions"
	"github.com/henderiw/store"
	"github.com/pkg/errors"
)

/*
https://github.com/kform/releases/download/v0.0.1/provider-kubernetes_0.0.1_darwin_amd64
europe-docker.pkg.dev/srlinux/eu.gcr.io/provider-xxxx
github.com/kform/provider-xxxx
*/

//ghcr.io/kform-tools/kformpkg-action/kformpkg-action:main

func GetPackageFromRef(ref string) (*Package, error) {
	pkg := &Package{}
	versionSplit := strings.Split(ref, ":")
	if len(versionSplit) != 2 {
		return nil, fmt.Errorf("unexpected ref semantics, want: <hostname>/<namespace>/<name>:<version>, got: %s", ref)
	}
	pkg.SelectedVersion = strings.ReplaceAll(versionSplit[1], "v", "")

	split := strings.Split(versionSplit[0], "/")
	if len(split) < 3 {
		return nil, fmt.Errorf("unexpected ref semantics, want: <hostname>/<namespace>/<name>, got: %s", versionSplit[0])
	}
	pkg.Address = &Address{
		HostName:  split[0],
		Namespace: filepath.Join(split[1:(len(split) - 1)]...),
		Name:      split[len(split)-1],
	}
	return pkg, nil
}

// address -> hostname, namespace, name
func GetPackage(nsn store.Key, source string) (*Package, error) {
	// TODO handle multiple requirements
	hostname, namespace, err := ParseSource(source)
	if err != nil {
		return nil, err
	}
	pkg := &Package{
		Address: &Address{
			HostName:  hostname,
			Namespace: namespace,
			Name:      nsn.Name,
		},
		Platform: &Platform{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		VersionConstraints: "",
	}
	return pkg, nil
}

// GetReleases returns the avilable releases/versions of the package
func (r *Package) GetReleases(ctx context.Context) (Releases, error) {
	url := r.ReleasesURL()
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get versions url %s, status code: %d", url, resp.StatusCode)
	}

	availableReleases := Releases{}
	if err := json.NewDecoder(resp.Body).Decode(&availableReleases); err != nil {
		return nil, err
	}
	for _, availableRelease := range availableReleases {
		v, err := versions.ParseVersion(strings.ReplaceAll(availableRelease.TagName, "v", ""))
		if err != nil {
			return nil, fmt.Errorf("cannot parse version: %s, err %s", availableRelease.TagName, err.Error())
		}
		r.AvailableVersions = append(r.AvailableVersions, v)
	}

	for _, availableRelease := range availableReleases {
		fmt.Println("availableRelease", availableRelease.TagName)
		for _, asset := range availableRelease.Assets {
			fmt.Printf("  Name: %s, State: %s, type: %s DownloadURL: %s\n", asset.Name, asset.State, asset.ContentType, asset.BrowserDownloadURL)

		}
	}
	return availableReleases, nil
}

type Releases []Release

type Release struct {
	Name    string  `json:"name"`
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets,omitempty"`
}

type Asset struct {
	Name               string     `json:"name"`
	ContentType        string     `json:"content_type"`
	State              AssetState `json:"state"`
	BrowserDownloadURL string     `json:"browser_download_url"`
}

type AssetState string

const (
	AssetStateUpLoaded AssetState = "uploaded"
)

func (r *Package) AddConstraints(constraint string) {
	if r.VersionConstraints == "" {
		r.VersionConstraints = constraint
	} else {
		r.VersionConstraints = fmt.Sprintf("%s, %s", r.VersionConstraints, constraint)
	}
	fmt.Println("constraints", r.VersionConstraints)
}

func (r *Package) GenerateCandidates() error {
	allowed, err := versions.MeetingConstraintsStringRuby(r.VersionConstraints)
	if err != nil {
		return errors.Wrap(err, "invalid version constraint")
	}
	fmt.Println("allowed versions", allowed)
	fmt.Println("available versions", r.AvailableVersions)
	r.CandidateVersions = r.AvailableVersions.Filter(allowed)
	fmt.Println("candidate versions", r.CandidateVersions)
	return nil
}

func (r *Package) GetRemoteChecksum(ctx context.Context, version string) (string, error) {
	resp, err := http.Get(r.ChecksumURL(version))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download checksum file %s, status code: %d", r.ChecksumURL(version), resp.StatusCode)
	}

	s := bufio.NewScanner(resp.Body)
	for s.Scan() {
		line := s.Text()
		if strings.HasSuffix(line, r.Filename()) {
			return strings.TrimSpace(strings.TrimSuffix(line, r.Filename())), nil
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %s not found in the url %s", r.BasePath(), r.ChecksumURL(version))
}

func (r *Package) HasVersion(version string) bool {
	return r.CandidateVersions.Set().Has(versions.MustParseVersion(version))
}

func (r *Package) Newest() string {
	fmt.Println("candidiate versions", r.CandidateVersions)
	return r.CandidateVersions.Newest().String()
}

func (r *Package) UpdateSelectedVersion(version string) {
	r.SelectedVersion = version
}

func (r *Package) GetSelectedVersion() string {
	return r.SelectedVersion
}
