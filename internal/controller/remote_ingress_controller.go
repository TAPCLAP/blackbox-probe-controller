/*
Copyright 2026.

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

package controller

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/ingress"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/state"
	probesync "github.com/TAPCLAP/blackbox-probe-controller/internal/sync"
)

// RemoteIngressReconciler watches Ingress objects in a remote cluster.
type RemoteIngressReconciler struct {
	client.Client
	ClusterName string
	Store       *state.DesiredStateStore
	Trigger     probesync.Trigger
}

// Reconcile updates desired probe state for a remote Ingress.
func (r *RemoteIngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("cluster", r.ClusterName, "ingress", req.NamespacedName)

	ing := &networkingv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, ing); err != nil {
		if apierrors.IsNotFound(err) {
			changed, gk := r.Store.RemoveIngress(r.ClusterName, req.Namespace, req.Name)
			if changed {
				r.Trigger.TriggerSync(gk)
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !ingress.ProbeEnabled(ing) {
		changed, gk := r.Store.RemoveIngress(r.ClusterName, req.Namespace, req.Name)
		if changed {
			r.Trigger.TriggerSync(gk)
		}
		return ctrl.Result{}, nil
	}

	targets := ingress.ProbeTargets(ing)
	changed, gk := r.Store.SetIngress(r.ClusterName, req.Namespace, req.Name, targets)
	if changed {
		log.Info("Updated probe targets", "targetCount", len(targets))
		r.Trigger.TriggerSync(gk)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler on a remote cluster manager.
func (r *RemoteIngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	clusterName := r.ClusterName
	store := r.Store
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			ing, ok := obj.(*networkingv1.Ingress)
			if !ok {
				return false
			}
			return ingress.ProbeEnabled(ing) || store.HasIngress(clusterName, ing.Namespace, ing.Name)
		})).
		WithOptions(controller.Options{MaxConcurrentReconciles: 2}).
		Named(safeControllerName("remote-ingress-" + clusterName)).
		Complete(r)
}

func safeControllerName(name string) string {
	out := make([]rune, 0, len(name))
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			out = append(out, c)
		} else if c >= 'A' && c <= 'Z' {
			out = append(out, c+'a'-'A')
		} else {
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "remote-ingress"
	}
	return string(out)
}
