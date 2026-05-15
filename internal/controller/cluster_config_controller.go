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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/apiconst"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/cluster"
)

// ClusterConfigReconciler reconciles cluster configuration Secrets.
type ClusterConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Registry               *cluster.Registry
	VMProbeSync            *VMProbeSyncReconciler
	ClusterSecretNamespace string
}

// Reconcile starts or stops remote cluster connections based on Secrets.
func (r *ClusterConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if secret.Namespace != r.ClusterSecretNamespace {
		return ctrl.Result{}, nil
	}
	if secret.Labels == nil || secret.Labels[apiconst.LabelClusterConfig] != "true" {
		return ctrl.Result{}, nil
	}

	clusterName, err := cluster.ClusterNameFromSecret(secret)
	if err != nil {
		log.Error(err, "Invalid cluster config secret")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if !secret.DeletionTimestamp.IsZero() {
		if err := r.handleDelete(ctx, secret, clusterName); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	cfg, err := cluster.RESTConfigFromSecret(secret)
	if err != nil {
		log.Error(err, "Failed to parse kubeconfig")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if err := r.Registry.StartCluster(ctx, clusterName, cfg, secret.ResourceVersion); err != nil {
		log.Error(err, "Failed to start remote cluster")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if !controllerutil.ContainsFinalizer(secret, apiconst.FinalizerName) {
		controllerutil.AddFinalizer(secret, apiconst.FinalizerName)
		if err := r.Update(ctx, secret); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("Remote cluster is running")
	return ctrl.Result{}, nil
}

func (r *ClusterConfigReconciler) handleDelete(ctx context.Context, secret *corev1.Secret, clusterName string) error {
	log := logf.FromContext(ctx).WithValues("cluster", clusterName)

	groups := r.Registry.StopCluster(clusterName)
	if err := r.VMProbeSync.DeleteClusterProbes(ctx, clusterName); err != nil {
		return err
	}
	r.VMProbeSync.TriggerSync(groups...)
	r.VMProbeSync.TriggerFullSync()

	if controllerutil.ContainsFinalizer(secret, apiconst.FinalizerName) {
		controllerutil.RemoveFinalizer(secret, apiconst.FinalizerName)
		if err := r.Update(ctx, secret); err != nil {
			return err
		}
	}

	log.Info("Stopped remote cluster")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				return false
			}
			if secret.Namespace != r.ClusterSecretNamespace {
				return false
			}
			return secret.Labels != nil && secret.Labels[apiconst.LabelClusterConfig] == "true"
		})).
		Named("cluster-config").
		Complete(r)
}
