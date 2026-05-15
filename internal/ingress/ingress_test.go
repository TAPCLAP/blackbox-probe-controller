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

package ingress

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/apiconst"
)

func TestProbeEnabled(t *testing.T) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				apiconst.AnnotationEnabled: "true",
			},
		},
	}
	if !ProbeEnabled(ing) {
		t.Fatal("expected probe enabled")
	}
	if ProbeEnabled(&networkingv1.Ingress{}) {
		t.Fatal("expected probe disabled")
	}
}

func TestProbeTargets(t *testing.T) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				apiconst.AnnotationEnabled:   "true",
				apiconst.AnnotationProbePath: "/healthz",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{Hosts: []string{"example.com"}}},
			Rules: []networkingv1.IngressRule{
				{Host: "example.com"},
				{Host: "api.example.com"},
			},
		},
	}

	targets := ProbeTargets(ing)
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d: %v", len(targets), targets)
	}
	if targets[0] != "https://api.example.com/healthz" && targets[0] != "https://example.com/healthz" {
		t.Fatalf("unexpected target: %s", targets[0])
	}
}

func TestProbeTargetsDefaultPath(t *testing.T) {
	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{Host: "svc.local"}},
		},
	}
	targets := ProbeTargets(ing)
	if len(targets) != 1 || targets[0] != "http://svc.local/ready" {
		t.Fatalf("unexpected targets: %v", targets)
	}
}
