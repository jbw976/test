/*
Copyright 2021 The Crossplane Authors.

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

package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"

	pc "github.com/crossplane/test/apis/provider"
	"github.com/crossplane/test/test/framework/provider"
)

func TestProviderUpgrade(t *testing.T) {
	cases := map[string]struct {
		reason string
		body   func(t *testing.T, providerPackage string, upgradeVersion pc.UpgradeProviderVersion) error
	}{
		"UpgradeProviderStableToLatest": {
			reason: "Should be able to successfully update provider from latest stable to latest development build.",
			body: func(t *testing.T, providerPackage string, upgradeVersion pc.UpgradeProviderVersion) error {
				sl := strings.SplitAfter(providerPackage, "/")
				packageName := sl[len(sl)-1]
				initialProviderPackage := providerPackage + ":" + upgradeVersion.Initial
				upgradeProviderPackage := providerPackage + ":" + upgradeVersion.Final

				ctx := context.Background()
				s := runtime.NewScheme()
				if err := v1.AddToScheme(s); err != nil {
					return err
				}
				c, err := client.New(ctrl.GetConfigOrDie(), client.Options{
					Scheme: s,
				})
				if err != nil {
					return err
				}
				a := resource.NewAPIUpdatingApplicator(c)
				p := &v1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name: packageName,
					},
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package: initialProviderPackage,
						},
					},
				}
				// Create initial Provider.
				if err := a.Apply(ctx, p); err != nil {
					return err
				}

				// Wait for Provider to be successfully installed.
				if err := provider.WaitForAllProvidersInstalled(t, ctx, c, 5*time.Second, 2*time.Minute); err != nil {
					return err
				}

				// Update Provider package.
				p.Spec.Package = upgradeProviderPackage
				if err := a.Apply(ctx, p); err != nil {
					return err
				}

				// Wait for Provider to be successfully updated.
				if err := provider.WaitForRevisionTransition(ctx, c, upgradeProviderPackage, initialProviderPackage, 5*time.Second, 2*time.Minute); err != nil {
					return err
				}

				// Clean up Provider.
				if err := c.DeleteAllOf(ctx, p); err != nil {
					return err
				}

				// Wait for Provider to be successfully deleted.
				return provider.WaitForAllProvidersDeleted(ctx, c, 5*time.Second, 30*time.Second)
			},
		},
	}

	config := pc.GetConfiguration("../../../config/provider/conformance.yml")

	for _, pr := range config.Providers {
		for _, upgradeVersion := range pr.Upgrade {
			for name, tc := range cases {
				t.Run(name, func(t *testing.T) {
					if err := tc.body(t, pr.Package, upgradeVersion); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}

}
