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
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	vmv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/apiconst"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/options"
)

var dns1123Invalid = regexp.MustCompile(`[^a-z0-9-]+`)

// VMProbeName returns a DNS-1123 subdomain name for a VMProbe.
func VMProbeName(cluster, namespace string) string {
	base := dns1123Invalid.ReplaceAllString(
		strings.ToLower(fmt.Sprintf("bb-%s-%s", cluster, namespace)),
		"-",
	)
	base = strings.Trim(base, "-")
	for strings.Contains(base, "--") {
		base = strings.ReplaceAll(base, "--", "-")
	}
	if len(base) <= 63 && base != "" {
		return base
	}
	if base == "" {
		base = "bb"
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(cluster+"/"+namespace)))[:8]
	maxBase := max(63-len(hash)-1, 1)
	if len(base) > maxBase {
		base = strings.Trim(base[:maxBase], "-")
	}
	if base == "" {
		base = "bb"
	}
	return base + "-" + hash
}

// BuildVMProbe constructs the desired VMProbe object.
func BuildVMProbe(
	opts *options.Options,
	clusterName string,
	sourceNamespace string,
	targets []string,
) *vmv1beta1.VMProbe {
	name := VMProbeName(clusterName, sourceNamespace)
	jobName := name
	if opts.ProbeJobName != "" {
		jobName = opts.ProbeJobName
	}
	labels := map[string]string{
		"app.kubernetes.io/managed-by": apiconst.ManagedByValue,
		apiconst.LabelCluster:          clusterName,
		apiconst.LabelSourceNamespace:  sourceNamespace,
	}

	return &vmv1beta1.VMProbe{
		TypeMeta: metav1.TypeMeta{
			APIVersion: vmv1beta1.SchemeGroupVersion.String(),
			Kind:       "VMProbe",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: opts.VMProbeNamespace,
			Labels:    labels,
		},
		Spec: vmv1beta1.VMProbeSpec{
			JobName: jobName,
			Module:  opts.ProbeModule,
			EndpointScrapeParams: vmv1beta1.EndpointScrapeParams{
				Interval:      opts.ProbeInterval,
				ScrapeTimeout: opts.ProbeScrapeTimeout,
			},
			VMProberSpec: vmv1beta1.VMProberSpec{
				URL: opts.BlackboxProberURL,
			},
			Targets: vmv1beta1.VMProbeTargets{
				StaticConfig: &vmv1beta1.VMProbeTargetStaticConfig{
					Targets:        append([]string(nil), targets...),
					RelabelConfigs: relabelConfigs(clusterName, sourceNamespace),
				},
			},
		},
	}
}

func relabelConfigs(clusterName, sourceNamespace string) []*vmv1beta1.RelabelConfig {
	hostRegex := vmv1beta1.StringOrArray{`https://([^/]+).*`}
	return []*vmv1beta1.RelabelConfig{
		{
			SourceLabels: []string{"__param_target"},
			TargetLabel:  "target",
		},
		{
			TargetLabel: "exported_cluster",
			Replacement: &clusterName,
		},
		{
			TargetLabel: "namespace",
			Replacement: &sourceNamespace,
		},
		{
			Regex:        hostRegex,
			SourceLabels: []string{"__param_target"},
			TargetLabel:  "host",
			Replacement:  ptr("$1"),
		},
	}
}

func ptr(s string) *string {
	return &s
}

// SpecEqual compares VMProbe specs relevant to reconciliation.
func SpecEqual(a, b *vmv1beta1.VMProbe) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Spec.JobName != b.Spec.JobName ||
		a.Spec.Module != b.Spec.Module ||
		a.Spec.Interval != b.Spec.Interval ||
		a.Spec.ScrapeTimeout != b.Spec.ScrapeTimeout ||
		a.Spec.VMProberSpec.URL != b.Spec.VMProberSpec.URL {
		return false
	}
	ast := a.Spec.Targets.StaticConfig
	bst := b.Spec.Targets.StaticConfig
	if ast == nil || bst == nil {
		return ast == bst
	}
	if len(ast.Targets) != len(bst.Targets) {
		return false
	}
	for i := range ast.Targets {
		if ast.Targets[i] != bst.Targets[i] {
			return false
		}
	}
	return true
}
