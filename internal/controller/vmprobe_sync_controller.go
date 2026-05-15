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
	"strings"
	"time"

	vmv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/apiconst"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/options"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/probe"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/state"
)

const vmprobeSyncFullName = "sync-all"

// VMProbeSyncReconciler synchronizes VMProbe objects in the home cluster.
type VMProbeSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Store   *state.DesiredStateStore
	Options *options.Options

	eventCh chan event.TypedGenericEvent[state.GroupKey]
}

// NewVMProbeSyncReconciler creates a reconciler with an internal sync trigger channel.
func NewVMProbeSyncReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	store *state.DesiredStateStore,
	opts *options.Options,
) *VMProbeSyncReconciler {
	return &VMProbeSyncReconciler{
		Client:  c,
		Scheme:  scheme,
		Store:   store,
		Options: opts,
		eventCh: make(chan event.TypedGenericEvent[state.GroupKey], 256),
	}
}

// TriggerSync enqueues synchronization for the given groups.
func (r *VMProbeSyncReconciler) TriggerSync(keys ...state.GroupKey) {
	for _, k := range keys {
		select {
		case r.eventCh <- event.TypedGenericEvent[state.GroupKey]{Object: k}:
		default:
			go func(gk state.GroupKey) {
				r.eventCh <- event.TypedGenericEvent[state.GroupKey]{Object: gk}
			}(k)
		}
	}
}

// TriggerFullSync enqueues a full synchronization.
func (r *VMProbeSyncReconciler) TriggerFullSync() {
	r.TriggerSync(state.GroupKey{})
}

// Reconcile applies desired VMProbe state.
func (r *VMProbeSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if req.Name == vmprobeSyncFullName {
		if err := r.syncAll(ctx); err != nil {
			return ctrl.Result{}, err
		}
		log.V(1).Info("Completed full VMProbe sync")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	gk, ok := decodeGroupKey(req.Name)
	if !ok {
		if err := r.syncAll(ctx); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	if err := r.syncGroup(ctx, gk); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler on the home cluster manager.
func (r *VMProbeSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ns := r.Options.VMProbeNamespace
	syncHandler := handler.TypedFuncs[state.GroupKey, reconcile.Request]{
		GenericFunc: func(_ context.Context, e event.TypedGenericEvent[state.GroupKey], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			for _, req := range requestsForGroup(ns, e.Object) {
				q.Add(req)
			}
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("vmprobe-sync").
		WatchesRawSource(source.Channel(r.eventCh, syncHandler)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 4}).
		Complete(r)
}

func requestsForGroup(ns string, gk state.GroupKey) []reconcile.Request {
	if gk.IsZero() {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Namespace: ns,
				Name:      vmprobeSyncFullName,
			},
		}}
	}
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Namespace: ns,
			Name:      encodeGroupKey(gk),
		},
	}}
}

func (r *VMProbeSyncReconciler) syncAll(ctx context.Context) error {
	for _, gk := range r.Store.AllGroups() {
		if err := r.syncGroup(ctx, gk); err != nil {
			return err
		}
	}
	return r.pruneOrphaned(ctx)
}

func (r *VMProbeSyncReconciler) syncGroup(ctx context.Context, gk state.GroupKey) error {
	log := logf.FromContext(ctx).WithValues("cluster", gk.Cluster, "namespace", gk.Namespace)

	targets := r.Store.GroupTargets(gk.Cluster, gk.Namespace)
	name := probe.VMProbeName(gk.Cluster, gk.Namespace)
	nn := types.NamespacedName{Namespace: r.Options.VMProbeNamespace, Name: name}

	if len(targets) == 0 {
		existing := &vmv1beta1.VMProbe{}
		if err := r.Get(ctx, nn, existing); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if err := r.Delete(ctx, existing); err != nil {
			return err
		}
		log.Info("Deleted VMProbe")
		return nil
	}

	desired := probe.BuildVMProbe(r.Options, gk.Cluster, gk.Namespace, targets)

	existing := &vmv1beta1.VMProbe{}
	err := r.Get(ctx, nn, existing)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return err
		}
		log.Info("Created VMProbe", "name", name, "targets", len(targets))
		return nil
	}
	if err != nil {
		return err
	}

	if probe.SpecEqual(existing, desired) && labelsEqual(existing.Labels, desired.Labels) {
		return nil
	}

	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	if err := r.Update(ctx, existing); err != nil {
		return err
	}
	log.Info("Updated VMProbe", "name", name, "targets", len(targets))
	return nil
}

func (r *VMProbeSyncReconciler) pruneOrphaned(ctx context.Context) error {
	list := &vmv1beta1.VMProbeList{}
	if err := r.List(ctx, list,
		client.InNamespace(r.Options.VMProbeNamespace),
		client.MatchingLabels{"app.kubernetes.io/managed-by": apiconst.ManagedByValue},
	); err != nil {
		return err
	}

	active := make(map[string]struct{})
	for _, gk := range r.Store.AllGroups() {
		active[probe.VMProbeName(gk.Cluster, gk.Namespace)] = struct{}{}
	}

	for i := range list.Items {
		item := &list.Items[i]
		if _, ok := active[item.Name]; ok {
			continue
		}
		if err := r.Delete(ctx, item); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// DeleteClusterProbes removes all VMProbes for a remote cluster.
func (r *VMProbeSyncReconciler) DeleteClusterProbes(ctx context.Context, clusterName string) error {
	list := &vmv1beta1.VMProbeList{}
	if err := r.List(ctx, list,
		client.InNamespace(r.Options.VMProbeNamespace),
		client.MatchingLabels{apiconst.LabelCluster: clusterName},
	); err != nil {
		return err
	}
	for i := range list.Items {
		if err := r.Delete(ctx, &list.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func encodeGroupKey(gk state.GroupKey) string {
	return "g-" + strings.ReplaceAll(gk.Cluster, "/", "-") + "--" + strings.ReplaceAll(gk.Namespace, "/", "-")
}

func decodeGroupKey(name string) (state.GroupKey, bool) {
	if !strings.HasPrefix(name, "g-") {
		return state.GroupKey{}, false
	}
	body := strings.TrimPrefix(name, "g-")
	parts := strings.SplitN(body, "--", 2)
	if len(parts) != 2 {
		return state.GroupKey{}, false
	}
	return state.GroupKey{Cluster: parts[0], Namespace: parts[1]}, true
}

func labelsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
