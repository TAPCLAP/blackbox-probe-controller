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

package cluster

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	crconfig "sigs.k8s.io/controller-runtime/pkg/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/state"
	probesync "github.com/TAPCLAP/blackbox-probe-controller/internal/sync"
)

// RemoteSetupFunc registers controllers on a remote cluster manager.
type RemoteSetupFunc func(mgr ctrl.Manager, clusterName string) error

// Registry manages remote cluster connections.
type Registry struct {
	mu       sync.Mutex
	clusters map[string]*remoteCluster

	scheme  *runtime.Scheme
	store   *state.DesiredStateStore
	setup   RemoteSetupFunc
	trigger probesync.Trigger
}

type remoteCluster struct {
	cancel          context.CancelFunc
	resourceVersion string
}

// NewRegistry creates a cluster registry.
func NewRegistry(
	scheme *runtime.Scheme,
	store *state.DesiredStateStore,
	setup RemoteSetupFunc,
	trigger probesync.Trigger,
) *Registry {
	return &Registry{
		clusters: make(map[string]*remoteCluster),
		scheme:   scheme,
		store:    store,
		setup:    setup,
		trigger:  trigger,
	}
}

// StartCluster connects to a remote cluster and starts watching Ingress resources.
// secretResourceVersion is the Secret metadata.resourceVersion; when it matches an
// already-running connection, StartCluster is a no-op.
func (r *Registry) StartCluster(ctx context.Context, name string, cfg *rest.Config, secretResourceVersion string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.clusters[name]; ok {
		if existing.resourceVersion == secretResourceVersion {
			return nil
		}
		existing.cancel()
		delete(r.clusters, name)
	}

	clusterCtx, cancel := context.WithCancel(ctx)

	remoteMgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 r.scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		LeaderElection:         false,
		// Remote managers are created per cluster and may be restarted when the
		// Secret changes. Controller names are reused across manager lifetimes;
		// only one remote controller with a given name runs at a time.
		Controller: crconfig.Controller{
			SkipNameValidation: ptr.To(true),
		},
	})
	if err != nil {
		cancel()
		return fmt.Errorf("create remote manager for %q: %w", name, err)
	}

	if err := r.setup(remoteMgr, name); err != nil {
		cancel()
		return fmt.Errorf("setup remote controllers for %q: %w", name, err)
	}

	go func() {
		log := logf.FromContext(clusterCtx).WithValues("cluster", name)
		log.Info("Starting remote cluster manager")
		if err := remoteMgr.Start(clusterCtx); err != nil {
			log.Error(err, "Remote cluster manager stopped with error")
		}
	}()

	r.clusters[name] = &remoteCluster{
		cancel:          cancel,
		resourceVersion: secretResourceVersion,
	}
	return nil
}

// StopCluster disconnects from a remote cluster and clears its desired state.
func (r *Registry) StopCluster(name string) []state.GroupKey {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rc, ok := r.clusters[name]; ok {
		rc.cancel()
		delete(r.clusters, name)
	}

	return r.store.RemoveCluster(name)
}

// StopAll stops every remote cluster connection.
func (r *Registry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, rc := range r.clusters {
		rc.cancel()
		delete(r.clusters, name)
	}
}

// Running reports whether a cluster connection is active.
func (r *Registry) Running(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.clusters[name]
	return ok
}
