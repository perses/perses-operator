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

package perses

import (
	"context"
	"fmt"
	"time"

	logger "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/perses/perses-operator/internal/subreconciler"
)

// recreateWorkload deletes a Deployment or StatefulSet whose immutable
// spec.selector differs from the desired one, so that the next reconciliation
// recreates it. Without this, the update would be rejected on every reconcile
// until the workload was deleted by hand.
//
// The delete orphans the dependents (ReplicaSets of a Deployment, pods and
// ControllerRevisions of a StatefulSet) so the recreated workload adopts them:
// their labels still match the new, narrower selector, so the pods keep
// running and any template change rolls out as a normal update. The UID
// precondition makes sure only the workload that was inspected is deleted,
// never one that replaced it in the meantime.
func (r *PersesReconciler) recreateWorkload(ctx context.Context, log *logger.Entry, found, desired client.Object, oldSelector *metav1.LabelSelector) (*ctrl.Result, error) {
	// TypeMeta is usually empty on objects returned by the client, so resolve
	// the Kind from the scheme for log messages.
	kind := fmt.Sprintf("%T", found)
	if gvk, err := apiutil.GVKForObject(found, r.Scheme); err == nil {
		kind = gvk.Kind
	}

	// Make sure the desired workload is accepted by the API server before
	// removing the existing one; otherwise an invalid spec would leave the
	// instance without a workload until the spec is fixed.
	if err := r.validateCreate(ctx, desired); err != nil {
		log.WithError(err).Errorf("Desired %s is invalid, keeping the existing one", kind)
		return subreconciler.RequeueWithError(err)
	}

	log.WithField("oldSelector", oldSelector.MatchLabels).
		Infof("Recreating %s %s/%s because its immutable spec.selector changed", kind, found.GetNamespace(), found.GetName())

	uid := found.GetUID()
	if err := r.Delete(ctx, found,
		client.PropagationPolicy(metav1.DeletePropagationOrphan),
		client.Preconditions{UID: &uid},
	); err != nil && !apierrors.IsNotFound(err) {
		log.WithError(err).Errorf("Failed to delete %s for recreation", kind)
		return subreconciler.RequeueWithError(err)
	}

	return subreconciler.RequeueWithDelay(time.Second)
}

// validateCreate asks the API server to validate obj as a new object without
// persisting it. The object is copied and given a generated name so the dry
// run is not rejected with AlreadyExists while the object it replaces still
// exists; the returned error is wrapped with the real name, since the API
// server's message names the probe.
func (r *PersesReconciler) validateCreate(ctx context.Context, obj client.Object) error {
	probe := obj.DeepCopyObject().(client.Object)
	probe.SetName("")
	probe.SetGenerateName(generateNamePrefix(obj.GetName()))
	if err := r.Create(ctx, probe, client.DryRunAll); err != nil {
		return fmt.Errorf("desired %s/%s would be rejected by the API server: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}

// generateNamePrefix returns name followed by "-", clamped to the prefix
// length the API server itself allows for metadata.generateName, so a name
// already at the DNS label limit still produces a valid generated name.
func generateNamePrefix(name string) string {
	if max := names.MaxGeneratedNameLength - 1; len(name) > max {
		name = name[:max]
	}
	return name + "-"
}
