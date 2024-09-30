/*
Copyright 2024 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkgio

import (
	"context"
	"path/filepath"

	"github.com/henderiw/store"
	memstore "github.com/henderiw/store/memory"
	configv1alpha1 "github.com/pkgserver-dev/pkgserver/apis/config/v1alpha1"
	pkgv1alpha1 "github.com/pkgserver-dev/pkgserver/apis/pkg/v1alpha1"
	"github.com/pkgserver-dev/pkgserver/apis/pkgrevid"
	"github.com/pkgserver-dev/pkgserver/pkg/auth/ui"
	"github.com/pkgserver-dev/pkgserver/pkg/git/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GitReader struct {
	// URL of the Git Repository
	URL string
	// Secret with crendentials to access the repo
	Secret string
	// Deployment determines if this is a catalog repo or not
	Deployment bool
	// Directory used in the git repository as an offset
	Directory string
	// PackageID identifies target, repo, realm, package, workspace and revision (if assigned)
	PkgRevID *pkgrevid.PackageRevID
	// PkgPath identifies the localation of the package in the local filesystem
	PkgPath string
	// allows the consumer to specify its own data store
	DataStore store.Storer[[]byte]
}

func (r *GitReader) Read(ctx context.Context) (store.Storer[[]byte], error) {
	if r.DataStore == nil {
		r.DataStore = memstore.NewStore[[]byte](nil)
	}
	datastore := r.DataStore

	pkgDir := filepath.Join(r.PkgPath, pkg.LocalGitDirectory)
	//os.MkdirAll(pkgDir, 0700)

	repo := configv1alpha1.BuildRepository(
		metav1.ObjectMeta{
			Namespace: "default",
			Name:      r.PkgRevID.Repository,
		},
		configv1alpha1.RepositorySpec{
			Type: configv1alpha1.RepositoryTypeGit,
			Git: &configv1alpha1.GitRepository{
				URL:         r.URL,
				Credentials: r.Secret,
				Directory:   r.Directory,
			},
			Deployment: r.Deployment,
		},
		configv1alpha1.RepositoryStatus{},
	)
	cachedRepo, err := pkg.OpenRepository(ctx, pkgDir, repo, &pkg.Options{
		//CredentialResolver: viper.NewCredentialResolver(),
		UserInfoProvider: &ui.ApiserverUserInfoProvider{},
	})
	if err != nil {
		return datastore, err
	}
	lifecycle := pkgv1alpha1.PackageRevisionLifecycleDraft
	if r.PkgRevID.Revision != "" {
		lifecycle = pkgv1alpha1.PackageRevisionLifecyclePublished
	}

	pkgRev := pkgv1alpha1.BuildPackageRevision(
		metav1.ObjectMeta{
			Namespace: "default",
			Name:      r.PkgRevID.PkgRevString(),
		},
		pkgv1alpha1.PackageRevisionSpec{
			PackageRevID: *r.PkgRevID,
			Lifecycle:    lifecycle,
		},
		pkgv1alpha1.PackageRevisionStatus{},
	)

	resources, err := cachedRepo.GetResources(ctx, pkgRev)
	if err != nil {
		return datastore, err
	}
	for fileName, data := range resources {
		datastore.Create(store.ToKey(fileName), []byte(data))
	}

	return datastore, nil
}
