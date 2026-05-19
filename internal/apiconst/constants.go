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

package apiconst

const (
	Domain = "blackbox-probe-controller.tapclap.com"

	AnnotationEnabled     = Domain + "/enabled"
	AnnotationProbePath   = Domain + "/probe-path"
	AnnotationClusterName = Domain + "/cluster-name"

	LabelClusterConfig   = Domain + "/cluster-config"
	LabelCluster         = Domain + "/cluster"
	LabelSourceNamespace = Domain + "/source-namespace"

	ManagedByValue = "blackbox-probe-controller"

	DefaultProbePath = "/ready"
	// KubeconfigKey is the preferred Secret data key for cluster kubeconfig.
	KubeconfigKey = "kubeconfig"
	FinalizerName = Domain + "/finalizer"
)

// KubeconfigKeys lists Secret data keys tried in order (first match wins).
var KubeconfigKeys = []string{KubeconfigKey, "kubeconfig.yaml", "kubeconfig.yml"}
