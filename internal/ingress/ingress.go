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
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/apiconst"
)

// ProbeEnabled reports whether the Ingress should be probed.
func ProbeEnabled(ing *networkingv1.Ingress) bool {
	if ing == nil || ing.Annotations == nil {
		return false
	}
	return ing.Annotations[apiconst.AnnotationEnabled] == "true"
}

// ProbeTargets returns blackbox probe URLs for the given Ingress.
func ProbeTargets(ing *networkingv1.Ingress) []string {
	if ing == nil {
		return nil
	}

	path := apiconst.DefaultProbePath
	if ing.Annotations != nil {
		if p := strings.TrimSpace(ing.Annotations[apiconst.AnnotationProbePath]); p != "" {
			path = p
		}
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	scheme := "http"
	if len(ing.Spec.TLS) > 0 {
		scheme = "https"
	}

	hosts := collectHosts(ing)
	if len(hosts) == 0 {
		return nil
	}

	targets := make([]string, 0, len(hosts))
	for _, host := range hosts {
		targets = append(targets, fmt.Sprintf("%s://%s%s", scheme, host, path))
	}
	return targets
}

func collectHosts(ing *networkingv1.Ingress) []string {
	seen := make(map[string]struct{})
	var hosts []string

	add := func(host string) {
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}

	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			add(rule.Host)
		}
	}

	if len(hosts) == 0 && ing.Status.LoadBalancer.Ingress != nil {
		for _, lb := range ing.Status.LoadBalancer.Ingress {
			if lb.Hostname != "" {
				add(lb.Hostname)
			} else if lb.IP != "" {
				add(lb.IP)
			}
		}
	}

	return hosts
}
