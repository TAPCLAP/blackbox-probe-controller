// Package clusterconfig holds RBAC markers for cluster configuration Secrets.
// Cluster Secrets live in the operator namespace (kustomize "system" placeholder).
package clusterconfig

// +kubebuilder:rbac:groups=core,namespace=system,resources=secrets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=events.k8s.io,namespace=system,resources=events,verbs=create;patch
