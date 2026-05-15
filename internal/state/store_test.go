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

package state

import (
	"reflect"
	"testing"
)

func TestDesiredStateStore(t *testing.T) {
	s := NewDesiredStateStore()

	changed, gk := s.SetIngress("c1", "ns1", "ing-a", []string{"https://a.example/ready"})
	if !changed || gk != (GroupKey{Cluster: "c1", Namespace: "ns1"}) {
		t.Fatalf("unexpected set result: %v %v", changed, gk)
	}

	changed, _ = s.SetIngress("c1", "ns1", "ing-a", []string{"https://a.example/ready"})
	if changed {
		t.Fatal("expected no change on identical set")
	}

	changed, _ = s.SetIngress("c1", "ns1", "ing-b", []string{"https://b.example/ready"})
	if !changed {
		t.Fatal("expected change on second ingress")
	}

	targets := s.GroupTargets("c1", "ns1")
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %v", targets)
	}

	if !s.HasIngress("c1", "ns1", "ing-a") {
		t.Fatal("expected ingress in store")
	}

	changed, _ = s.RemoveIngress("c1", "ns1", "ing-a")
	if !changed {
		t.Fatal("expected change on remove")
	}
	targets = s.GroupTargets("c1", "ns1")
	if len(targets) != 1 || targets[0] != "https://b.example/ready" {
		t.Fatalf("unexpected targets after remove: %v", targets)
	}

	groups := s.RemoveCluster("c1")
	if len(groups) != 1 {
		t.Fatalf("expected one group, got %v", groups)
	}
	if len(s.AllGroups()) != 0 {
		t.Fatal("expected empty store")
	}
}

func TestAllGroups(t *testing.T) {
	s := NewDesiredStateStore()
	s.SetIngress("c1", "ns1", "a", []string{"https://x/ready"})
	s.SetIngress("c1", "ns2", "b", []string{"https://y/ready"})

	groups := s.AllGroups()
	if len(groups) != 2 {
		t.Fatalf("groups: %v", groups)
	}
}

func TestDedupeSorted(t *testing.T) {
	got := dedupeSorted([]string{"b", "a", "b", ""})
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
