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
	"testing"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/state"
)

func TestRegistry_StartClusterIdempotent(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = networkingv1.AddToScheme(scheme)

	var setups int
	registry := NewRegistry(scheme, state.NewDesiredStateStore(), func(_ ctrl.Manager, _ string) error {
		setups++
		return nil
	}, nil)

	cfg := &rest.Config{
		Host: "https://127.0.0.1:65535",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	defer registry.StopAll()

	if err := registry.StartCluster(t.Context(), "test-cluster", cfg, "rv-1"); err != nil {
		t.Fatalf("first StartCluster: %v", err)
	}
	if err := registry.StartCluster(t.Context(), "test-cluster", cfg, "rv-1"); err != nil {
		t.Fatalf("second StartCluster: %v", err)
	}

	if setups != 1 {
		t.Fatalf("setup calls = %d, want 1", setups)
	}
	if !registry.Running("test-cluster") {
		t.Fatal("expected cluster to be running")
	}

	time.Sleep(10 * time.Millisecond)
}

func TestRegistry_StartClusterRestartsOnResourceVersionChange(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = networkingv1.AddToScheme(scheme)

	var setups int
	registry := NewRegistry(scheme, state.NewDesiredStateStore(), func(_ ctrl.Manager, _ string) error {
		setups++
		return nil
	}, nil)

	cfg := &rest.Config{
		Host: "https://127.0.0.1:65535",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	defer registry.StopAll()

	if err := registry.StartCluster(t.Context(), "test-cluster", cfg, "rv-1"); err != nil {
		t.Fatalf("first StartCluster: %v", err)
	}
	if err := registry.StartCluster(t.Context(), "test-cluster", cfg, "rv-2"); err != nil {
		t.Fatalf("restart StartCluster: %v", err)
	}

	if setups != 2 {
		t.Fatalf("setup calls = %d, want 2", setups)
	}
}
