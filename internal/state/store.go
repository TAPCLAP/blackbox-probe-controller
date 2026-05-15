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
	"fmt"
	"sort"
	"strings"
	"sync"
)

// GroupKey identifies a VMProbe aggregation group (cluster + ingress namespace).
type GroupKey struct {
	Cluster   string
	Namespace string
}

func (g GroupKey) String() string {
	return g.Cluster + "/" + g.Namespace
}

// IsZero reports whether the group key is unset.
func (g GroupKey) IsZero() bool {
	return g.Cluster == "" && g.Namespace == ""
}

func ingressKey(cluster, namespace, name string) string {
	return cluster + "/" + namespace + "/" + name
}

// DesiredStateStore holds desired probe targets keyed by remote Ingress.
type DesiredStateStore struct {
	mu        sync.RWMutex
	ingresses map[string][]string
}

// NewDesiredStateStore creates an empty store.
func NewDesiredStateStore() *DesiredStateStore {
	return &DesiredStateStore{
		ingresses: make(map[string][]string),
	}
}

// SetIngress records targets for an Ingress. Returns whether data changed and the affected group.
func (s *DesiredStateStore) SetIngress(cluster, namespace, name string, targets []string) (bool, GroupKey) {
	gk := GroupKey{Cluster: cluster, Namespace: namespace}
	key := ingressKey(cluster, namespace, name)
	normalized := dedupeSorted(targets)

	s.mu.Lock()
	defer s.mu.Unlock()

	prev, ok := s.ingresses[key]
	if ok && stringSliceEqual(prev, normalized) {
		return false, gk
	}
	if len(normalized) == 0 {
		delete(s.ingresses, key)
	} else {
		s.ingresses[key] = normalized
	}
	return true, gk
}

// HasIngress reports whether an Ingress entry exists in the store.
func (s *DesiredStateStore) HasIngress(cluster, namespace, name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.ingresses[ingressKey(cluster, namespace, name)]
	return ok
}

// RemoveIngress removes an Ingress entry. Returns whether data changed and the affected group.
func (s *DesiredStateStore) RemoveIngress(cluster, namespace, name string) (bool, GroupKey) {
	gk := GroupKey{Cluster: cluster, Namespace: namespace}
	key := ingressKey(cluster, namespace, name)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.ingresses[key]; !ok {
		return false, gk
	}
	delete(s.ingresses, key)
	return true, gk
}

// RemoveCluster removes all Ingress entries for a cluster and returns affected groups.
func (s *DesiredStateStore) RemoveCluster(cluster string) []GroupKey {
	prefix := cluster + "/"

	s.mu.Lock()
	defer s.mu.Unlock()

	groups := make(map[GroupKey]struct{})
	for key := range s.ingresses {
		if strings.HasPrefix(key, prefix) {
			parts := strings.SplitN(key, "/", 3)
			if len(parts) == 3 {
				groups[GroupKey{Cluster: parts[0], Namespace: parts[1]}] = struct{}{}
			}
			delete(s.ingresses, key)
		}
	}

	out := make([]GroupKey, 0, len(groups))
	for gk := range groups {
		out = append(out, gk)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].String() < out[j].String()
	})
	return out
}

// GroupTargets returns deduplicated sorted targets for a group.
func (s *DesiredStateStore) GroupTargets(cluster, namespace string) []string {
	prefix := fmt.Sprintf("%s/%s/", cluster, namespace)

	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]struct{})
	var targets []string
	for key, urls := range s.ingresses {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		for _, u := range urls {
			if _, ok := seen[u]; ok {
				continue
			}
			seen[u] = struct{}{}
			targets = append(targets, u)
		}
	}
	sort.Strings(targets)
	return targets
}

// AllGroups returns all non-empty groups present in the store.
func (s *DesiredStateStore) AllGroups() []GroupKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groups := make(map[GroupKey]struct{})
	for key := range s.ingresses {
		parts := strings.SplitN(key, "/", 3)
		if len(parts) != 3 {
			continue
		}
		groups[GroupKey{Cluster: parts[0], Namespace: parts[1]}] = struct{}{}
	}

	out := make([]GroupKey, 0, len(groups))
	for gk := range groups {
		if len(s.groupTargetsLocked(gk.Cluster, gk.Namespace)) > 0 {
			out = append(out, gk)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].String() < out[j].String()
	})
	return out
}

func (s *DesiredStateStore) groupTargetsLocked(cluster, namespace string) []string {
	prefix := fmt.Sprintf("%s/%s/", cluster, namespace)
	seen := make(map[string]struct{})
	var targets []string
	for key, urls := range s.ingresses {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		for _, u := range urls {
			if _, ok := seen[u]; ok {
				continue
			}
			seen[u] = struct{}{}
			targets = append(targets, u)
		}
	}
	sort.Strings(targets)
	return targets
}

func dedupeSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
