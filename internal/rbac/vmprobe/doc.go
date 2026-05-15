// Package vmprobe holds RBAC markers for VMProbe management.
// VMProbe objects are managed in --vmprobe-namespace (default "monitoring").
package vmprobe

// +kubebuilder:rbac:groups=operator.victoriametrics.com,namespace=monitoring,resources=vmprobes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,namespace=monitoring,resources=events,verbs=create;patch
