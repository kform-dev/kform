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
	configv1alpha1 "github.com/kform-dev/pkg-server/apis/config/v1alpha1"
	pkgv1alpha1 "github.com/kform-dev/pkg-server/apis/pkg/v1alpha1"
	"github.com/kform-dev/pkg-server/apis/pkgid"
	"github.com/kform-dev/pkg-server/pkg/auth/ui"
	"github.com/kform-dev/pkg-server/pkg/auth/viper"
	"github.com/kform-dev/pkg-server/pkg/git/pkg"
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
	PkgID *pkgid.PackageID
	// PkgPath identifies the localation of the package in the local filesystem
	PkgPath string
	// allows the consumer to specify its own data store
	DataStore store.Storer[[]byte]
}

func (r *GitReader) Read(ctx context.Context) (store.Storer[[]byte], error) {
	if r.DataStore == nil {
		r.DataStore = memstore.NewStore[[]byte]()
	}
	datastore := r.DataStore

	/*
		var refName plumbing.ReferenceName
		if r.Branch != "" {
			refName = plumbing.NewBranchReferenceName(r.Branch)
		}
		if r.Tag != "" {
			refName = plumbing.NewTagReferenceName(r.Tag)
		}
	*/

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
		CredentialResolver: viper.NewCredentialResolver(),
		UserInfoProvider:   &ui.ApiserverUserInfoProvider{},
	})
	if err != nil {
		return datastore, err
	}
	lifecycle := pkgv1alpha1.PackageRevisionLifecycleDraft
	if r.PkgID.Revision != "" {
		lifecycle = pkgv1alpha1.PackageRevisionLifecyclePublished
	}

	pkgRev := pkgv1alpha1.BuildPackageRevision(
		metav1.ObjectMeta{
			Namespace: "default",
			Name:      r.PkgID.PkgRevString(),
		},
		pkgv1alpha1.PackageRevisionSpec{
			PackageID: *r.PkgID,
			Lifecycle: lifecycle,
		},
		pkgv1alpha1.PackageRevisionStatus{},
	)

	resources, err := cachedRepo.GetResources(ctx, pkgRev)
	if err != nil {
		return datastore, err
	}
	for fileName, data := range resources {
		datastore.Create(ctx, store.ToKey(fileName), []byte(data))
	}

	/*
		s := memory.NewStorage()
		cachedRepo, err := git.Clone(s, memfs.New(), &git.CloneOptions{
			URL:           r.URL,
			ReferenceName: refName,
			SingleBranch:  true,
			Depth:         1,
			Progress:      os.Stdout,
			// SubModule could be useful but we avoid it for now
		})
		if err != nil {
			return datastore, err
		}
	*/

	/*
		// Retrieve the commit of the branch
		ref, err := cachedRepo.Head()
		if err != nil {
			return datastore, err
		}

		commit, err := cachedRepo.CommitObject(ref.Hash())
		if err != nil {
			return datastore, err
		}
		// Iterate over the tree and read files
		rootTree, err := commit.Tree()
		if err != nil {
			return datastore, err
		}
		tree, err := rootTree.Tree(r.Path)
		if err != nil {
			if err == object.ErrDirectoryNotFound {
				// We treat the filter prefix as a filter, the path doesn't have to exist
				fmt.Println("could not find prefix in commit; returning no resources", "package", r.Path, "commit", commit.Hash.String())
				return nil, nil
			} else {
				return nil, fmt.Errorf("error getting tree %s: %w", r.Path, err)
			}
		}

		if err := r.processTree(ctx, cachedRepo, tree); err != nil {
			return datastore, err
		}
	*/
	return datastore, nil
}

/*
func (r *GitReader) processTree(ctx context.Context, repo *git.Repository, tree *object.Tree) error {
	datastore := r.DataStore
	for _, entry := range tree.Entries {
		switch entry.Mode {
		case filemode.Submodule:
		case filemode.Symlink:
		case filemode.Executable:
		case filemode.Regular:
			//fmt.Println("regular", filepath.Join(basePath, entry.Name))
			fileBlob, err := repo.BlobObject(entry.Hash)
			if err != nil {
				return err
			}
			content, err := fileBlob.Reader()
			if err != nil {
				return err
			}
			buf := &bytes.Buffer{}
			buffer := make([]byte, 1024*1024)

			for {
				n, err := content.Read(buffer)
				if err != nil {
					if err == io.EOF {
						break
					}
					return err
				}
				buf.Write(buffer[:n])
			}
			//fmt.Println(buf.String())
			//datastore.Create(ctx, store.ToKey(filepath.Join(path, entry.Name)), buf.Bytes())
			reader := ByteReader{
				Reader:    strings.NewReader(buf.String()),
				DataStore: datastore,
				Path:      entry.Name,
			}
			if _, err := reader.Read(ctx); err != nil {
				return err
			}
		case filemode.Dir:
			// we set the newBasePath equal to the main basePath
			// since we try to filter the paths that are irrelevant

			//	newBasePath := basePath
			//	if len(pathSlice) != 0 {
			//		if pathSlice[0] != entry.Name {
			//			continue
			//		}
			//		pathSlice = pathSlice[1:]
			//	} else {
			//		newBasePath = filepath.Join(basePath, entry.Name)
			//		//fmt.Println("dir", newBasePath)
			//	}
			//
			//	subTree, err := repo.TreeObject(entry.Hash)
			//	if err != nil {
			//		return err
			//	}
			//	if err := r.processTree(ctx, repo, subTree, newBasePath, pathSlice); err != nil {
			//		return err
			//	}

		default:
			//fmt.Println("default", filepath.Join(entry.Name))
		}
	}
	return nil
}
*/
