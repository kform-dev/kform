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
	"strings"

	"github.com/henderiw/store"
	configv1alpha1 "github.com/pkgserver-dev/pkgserver/apis/config/v1alpha1"
	pkgv1alpha1 "github.com/pkgserver-dev/pkgserver/apis/pkg/v1alpha1"
	"github.com/pkgserver-dev/pkgserver/apis/pkgid"
	"github.com/pkgserver-dev/pkgserver/pkg/auth/ui"
	"github.com/pkgserver-dev/pkgserver/pkg/git/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GitWriter struct {
	// URL of the Git Repository
	URL string
	// Secret with crendentials to access the repo
	Secret string
	// Deployment determines if this is a catalog repo or not
	Deployment bool
	// Directory used in the git repository as an offset
	Directory string
	// PackageID identifies target, repo, realm, package, workspace and revision (if assigned)
	PkgID *pkgid.PackageID
	// PkgPath identifies the localation of the package in the local filesystem
	PkgPath string
}

func (r *GitWriter) Write(ctx context.Context, datastore store.Storer[[]byte]) error {

	resources := map[string]string{}
	datastore.List(ctx, func(ctx context.Context, k store.Key, b []byte) {
		parts := strings.Split(k.Name, ".")
		filename := strings.Join(parts[:len(parts)-1], ".")
		resources[filename] = string(b)
	})

	pkgDir := filepath.Join(r.PkgPath, pkg.LocalGitDirectory)
	//os.MkdirAll(pkgDir, 0700)

	repo := configv1alpha1.BuildRepository(
		metav1.ObjectMeta{
			Namespace: "default",
			Name:      r.PkgID.Repository,
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
		return err
	}

	pkgRev := pkgv1alpha1.BuildPackageRevision(
		metav1.ObjectMeta{
			Namespace: "default",
			Name:      r.PkgID.PkgRevString(),
		},
		pkgv1alpha1.PackageRevisionSpec{
			PackageID:    *r.PkgID,
			Lifecycle:    pkgv1alpha1.PackageRevisionLifecycleDraft,
			UpdatePolicy: pkgv1alpha1.UpdatePolicy_Strict,
		},
		pkgv1alpha1.PackageRevisionStatus{},
	)
	if err := cachedRepo.UpsertPackageRevision(ctx, pkgRev, resources); err != nil {
		return err
	}

	return nil
}
