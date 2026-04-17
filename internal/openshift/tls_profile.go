// Copyright The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package openshift

import (
	"context"
	"strings"
	"sync"

	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FetchTLSProfile reads the TLS security profile from the cluster's
// config.openshift.io/v1 APIServer resource and returns the min TLS
// version and cipher suites as strings suitable for CLI flags.
func FetchTLSProfile(ctx context.Context, c client.Client) (minVersion string, cipherSuites string, profileSpec configv1.TLSProfileSpec, err error) {
	profileSpec, err = openshifttls.FetchAPIServerTLSProfile(ctx, c)
	if err != nil {
		return "", "", configv1.TLSProfileSpec{}, err
	}

	minVersion = string(profileSpec.MinTLSVersion)

	if len(profileSpec.Ciphers) > 0 {
		cipherNames := make([]string, len(profileSpec.Ciphers))
		for i, c := range profileSpec.Ciphers {
			cipherNames[i] = string(c)
		}
		cipherSuites = strings.Join(cipherNames, ",")
	}

	return minVersion, cipherSuites, profileSpec, nil
}

// SetupProfileWatcher registers a controller that watches the OpenShift
// APIServer resource for TLS profile changes. When a change is detected,
// it calls cancelFunc to trigger a graceful operator restart. The cancel
// function is guarded by sync.Once to prevent repeated calls from rapid
// profile changes. Must be called before mgr.Start().
func SetupProfileWatcher(mgr ctrl.Manager, initialProfileSpec configv1.TLSProfileSpec, cancelFunc context.CancelFunc) error {
	var cancelOnce sync.Once
	log := ctrl.Log.WithName("setup")

	watcher := &openshifttls.SecurityProfileWatcher{
		Client:                mgr.GetClient(),
		InitialTLSProfileSpec: initialProfileSpec,
		OnProfileChange: func(_ context.Context, _, _ configv1.TLSProfileSpec) {
			cancelOnce.Do(func() {
				log.Info("TLS security profile changed, triggering graceful restart")
				cancelFunc()
			})
		},
	}

	return watcher.SetupWithManager(mgr)
}
