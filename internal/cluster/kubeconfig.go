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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/apiconst"
)

// ClusterNameFromSecret returns the cluster name annotation value.
func ClusterNameFromSecret(secret *corev1.Secret) (string, error) {
	if secret.Annotations == nil {
		return "", fmt.Errorf("secret %s/%s missing annotation %s", secret.Namespace, secret.Name, apiconst.AnnotationClusterName)
	}
	name := secret.Annotations[apiconst.AnnotationClusterName]
	if name == "" {
		return "", fmt.Errorf("secret %s/%s has empty annotation %s", secret.Namespace, secret.Name, apiconst.AnnotationClusterName)
	}
	return name, nil
}

// RESTConfigFromSecret builds a REST config from the kubeconfig data in a Secret.
func RESTConfigFromSecret(secret *corev1.Secret) (*rest.Config, error) {
	data, ok := secret.Data[apiconst.KubeconfigKey]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s missing data key %q", secret.Namespace, secret.Name, apiconst.KubeconfigKey)
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig from secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	return cfg, nil
}
