/*
Copyright 2023 The Perses Authors.

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

package subreconciler

import (
	"context"
	"time"

	"github.com/perses/perses-operator/internal/perses/common"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Fn = func(context.Context) (*ctrl.Result, error)

type FnWithRequest = func(context.Context, ctrl.Request) (*ctrl.Result, error)

// Evaluate returns the actual reconcile struct and error. Wrap helpers in
// this when returning from within the top-level Reconciler.
func Evaluate(r *reconcile.Result, e error) (reconcile.Result, error) {
	return *r, e
}

// ContinueReconciling indicates that the reconciliation block should continue by
// returning a nil result and a nil error
func ContinueReconciling() (*reconcile.Result, error) { return nil, nil }

// DoNotRequeue returns a controller result pairing specifying not to requeue.
func DoNotRequeue() (*reconcile.Result, error) { return &ctrl.Result{}, nil }

// RequeueWithError returns a controller result pairing specifying to
// requeue with an error message.
func RequeueWithError(e error) (*reconcile.Result, error) { return &ctrl.Result{}, e }

func RequeueWithErrorAndReason(e error, reason common.ConditionStatusReason) (*reconcile.Result, common.ConditionStatusReason, error) {
	res, e := RequeueWithError(e)
	return res, reason, e
}

// RequeueWithDelay returns a controller result pairing specifying to
// requeue after a delay. This returns no error.
func RequeueWithDelay(dur time.Duration) (*reconcile.Result, error) {
	return &ctrl.Result{RequeueAfter: dur}, nil
}

// ShouldRequeue returns true if the reconciler result indicates
// a requeue is required, or the error is not nil.
func ShouldRequeue(r *ctrl.Result, err error) bool {
	// if we get a nil value for result, we need to
	// fill it with an empty value which would not trigger
	// a requeue.

	res := r
	if r.IsZero() {
		res = &ctrl.Result{}
	}
	return res.RequeueAfter > 0 || (err != nil)
}

// ShouldHaltOrRequeue returns true if reconciler result is not nil
// or the err is not nil. In theory, the error evaluation
// is not needed because ShouldRequeue handles it, but
// it's included in case ShouldHaltOrRequeue is called directly.
func ShouldHaltOrRequeue(r *ctrl.Result, err error) bool {
	return (r != nil) || ShouldRequeue(r, err)
}
