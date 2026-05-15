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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	vmv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/options"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/state"
)

var _ = Describe("VMProbeSyncReconciler", func() {
	const vmprobeNS = "monitoring"

	var (
		store   *state.DesiredStateStore
		syncRec *VMProbeSyncReconciler
	)

	BeforeEach(func() {
		store = state.NewDesiredStateStore()
		syncRec = NewVMProbeSyncReconciler(k8sClient, testScheme, store, &options.Options{
			VMProbeNamespace:   vmprobeNS,
			ProbeInterval:      "20s",
			ProbeScrapeTimeout: "18s",
			ProbeModule:        "http_2xx",
			BlackboxProberURL:  "blackbox.monitoring.svc:9115",
		})
	})

	It("creates VMProbe for a desired group", func(ctx SpecContext) {
		store.SetIngress("kube-prod", "internal", "app", []string{"https://example.com/ready"})

		_, err := syncRec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: vmprobeNS,
			Name:      encodeGroupKey(state.GroupKey{Cluster: "kube-prod", Namespace: "internal"}),
		}})
		Expect(err).NotTo(HaveOccurred())

		vm := &vmv1beta1.VMProbe{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Namespace: vmprobeNS,
			Name:      "bb-kube-prod-internal",
		}, vm)).To(Succeed())

		Expect(vm.Spec.Targets.StaticConfig.Targets).To(ContainElement("https://example.com/ready"))
		Expect(vm.Labels).To(HaveKeyWithValue("blackbox-probe-controller.tapclap.com/cluster", "kube-prod"))
	})

	It("deletes VMProbe when group has no targets", func(ctx SpecContext) {
		gk := state.GroupKey{Cluster: "kube-prod", Namespace: "empty"}
		vm := &vmv1beta1.VMProbe{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bb-kube-prod-empty",
				Namespace: vmprobeNS,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "blackbox-probe-controller",
				},
			},
			Spec: vmv1beta1.VMProbeSpec{
				VMProberSpec: vmv1beta1.VMProberSpec{URL: "blackbox.monitoring.svc:9115"},
				Targets: vmv1beta1.VMProbeTargets{
					StaticConfig: &vmv1beta1.VMProbeTargetStaticConfig{
						Targets: []string{"https://gone.example/ready"},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, vm)).To(Succeed())

		_, err := syncRec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: vmprobeNS,
			Name:      encodeGroupKey(gk),
		}})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Get(ctx, types.NamespacedName{Namespace: vmprobeNS, Name: vm.Name}, &vmv1beta1.VMProbe{})
		Expect(err).To(HaveOccurred())
	})
})
