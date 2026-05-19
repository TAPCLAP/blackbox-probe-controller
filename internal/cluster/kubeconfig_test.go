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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/apiconst"
)

const minimalKubeconfig = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:65535
    insecure-skip-tls-verify: true
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
users:
- name: test
  user: {}
`

func TestKubeconfigDataFromSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    map[string][]byte
		wantKey string
		wantErr bool
	}{
		{
			name:    "kubeconfig",
			data:    map[string][]byte{"kubeconfig": []byte("x")},
			wantKey: "kubeconfig",
		},
		{
			name:    "kubeconfig.yaml",
			data:    map[string][]byte{"kubeconfig.yaml": []byte("x")},
			wantKey: "kubeconfig.yaml",
		},
		{
			name:    "kubeconfig.yml",
			data:    map[string][]byte{"kubeconfig.yml": []byte("x")},
			wantKey: "kubeconfig.yml",
		},
		{
			name: "prefers kubeconfig over extensions",
			data: map[string][]byte{
				"kubeconfig":      []byte("primary"),
				"kubeconfig.yaml": []byte("yaml"),
			},
			wantKey: "kubeconfig",
		},
		{
			name:    "missing",
			data:    map[string][]byte{"other": []byte("x")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
				Data:       tt.data,
			}

			got, err := kubeconfigDataFromSecret(secret)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				for _, key := range apiconst.KubeconfigKeys {
					if !strings.Contains(err.Error(), key) {
						t.Fatalf("error %q should mention key %q", err, key)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != string(tt.data[tt.wantKey]) {
				t.Fatalf("got %q, want data from key %q", got, tt.wantKey)
			}
		})
	}
}

func TestRESTConfigFromSecretKubeconfigKeys(t *testing.T) {
	t.Parallel()

	for _, key := range apiconst.KubeconfigKeys {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cluster"},
				Data:       map[string][]byte{key: []byte(minimalKubeconfig)},
			}

			cfg, err := RESTConfigFromSecret(secret)
			if err != nil {
				t.Fatalf("RESTConfigFromSecret: %v", err)
			}
			if cfg.Host != "https://127.0.0.1:65535" {
				t.Fatalf("Host = %q, want https://127.0.0.1:65535", cfg.Host)
			}
		})
	}
}
