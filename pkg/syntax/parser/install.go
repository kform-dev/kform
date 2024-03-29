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

package parser

import (
	"context"
)

// validateAndOrInstallProviders looks at the provider requirements
// 1. convert provider requirements to packages
// 2. get the releases per provider (based in source <hostname>/<namespace>)
func (r *KformParser) validateAndOrInstallProviders(ctx context.Context) {
	// TODO
	/*
		for providerName, providerReq := range r.listProviderRequirements(ctx) {
				// convert provider requirements to a package
				// the source was validated to be aligned before so we can just pick the first one.
				pkg, err := address.GetPackage(providerName, providerReq[0].Source)
				if err != nil {
					r.recorder.Record(diag.DiagFromErr(err))
					return
				}
				// retrieve the available releases/versions for this provider
				if !pkg.IsLocal() {
					tags, err := oras.GetTags(ctx, pkg.GetRef())
					if err != nil {
						return
					}
					for _, tag := range tags {
						v, err := versions.ParseVersion(strings.ReplaceAll(tag, "v", ""))
						if err != nil {
							r.recorder.Record(diag.DiagFromErr(err))
							continue
						}
						pkg.AvailableVersions = append(pkg.AvailableVersions, v)
					}
					// append the requirements together
					for _, req := range reqs {
						pkg.AddConstraints(req.Version)
					}

					// generate the candidate versions by looking at the available
					// versions and applying the constraints on them
					if err := pkg.GenerateCandidates(); err != nil {
						r.recorder.Record(diag.DiagFromErr(err))
						return
					}
				}
				r.providers.Add(ctx, nsn, pkg)
		}

			pkgrw := pkgio.NewPkgProviderReadWriter(r.rootModulePath, r.providers)
			pl := pkgio.Pipeline{
				Inputs:     []pkgio.Reader{pkgrw},
				Processors: []pkgio.Process{pkgrw},
				Outputs:    []pkgio.Writer{pkgrw},
			}
			if !init {
				// when doing kform apply we just want to validate the provider pkg(s)
				pl = pkgio.Pipeline{
					Inputs:     []pkgio.Reader{pkgrw},
					Processors: []pkgio.Process{pkgrw},
					Outputs:    []pkgio.Writer{pkgio.NewPkgValidator()},
				}
			}

			if err := pl.Execute(ctx); err != nil {
				r.recorder.Record(diag.DiagFromErr(err))
				return
			}
	*/
}
