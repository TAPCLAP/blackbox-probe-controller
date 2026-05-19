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

package probe

import (
	"strings"
	"testing"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/options"
)

func TestVMProbeName(t *testing.T) {
	name := VMProbeName("kube-prod-fsn", "internal")
	if name != "bb-kube-prod-fsn-internal" {
		t.Fatalf("unexpected name: %s", name)
	}
	if len(VMProbeName(strings.Repeat("a", 40), strings.Repeat("b", 40))) > 63 {
		t.Fatal("expected truncated name")
	}
}

func TestBuildVMProbe(t *testing.T) {
	opts := &options.Options{
		VMProbeNamespace:   "monitoring",
		ProbeInterval:      "20s",
		ProbeScrapeTimeout: "18s",
		ProbeModule:        "http_2xx",
		BlackboxProberURL:  "blackbox.monitoring.svc:9115",
	}
	vm := BuildVMProbe(opts, "kube-prod", "apps", []string{"https://example.com/ready"})
	if vm.Namespace != "monitoring" {
		t.Fatalf("namespace: %s", vm.Namespace)
	}
	if vm.Spec.Targets.StaticConfig == nil {
		t.Fatal("expected staticConfig")
	}
	if len(vm.Spec.Targets.StaticConfig.Targets) != 1 {
		t.Fatalf("targets: %v", vm.Spec.Targets.StaticConfig.Targets)
	}
	if vm.Spec.Targets.StaticConfig.RelabelConfigs == nil {
		t.Fatal("expected relabel configs")
	}
	if vm.Spec.JobName != vm.Name {
		t.Fatalf("jobName: got %q, want %q", vm.Spec.JobName, vm.Name)
	}
}

func TestBuildVMProbeJobNameOverride(t *testing.T) {
	opts := &options.Options{
		VMProbeNamespace: "monitoring",
		ProbeModule:      "http_2xx",
		ProbeJobName:     "blackbox-ingress",
	}
	vm := BuildVMProbe(opts, "kube-prod", "apps", nil)
	if vm.Spec.JobName != "blackbox-ingress" {
		t.Fatalf("jobName: got %q", vm.Spec.JobName)
	}
	if vm.Name == vm.Spec.JobName {
		t.Fatal("resource name should differ from overridden jobName")
	}
}
