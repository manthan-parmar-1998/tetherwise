package kubernetes

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClient builds a client-go Interface from kubeconfig and an optional context.
// An empty kubeconfig uses the default loading rules.
func NewClient(kubeconfig, contextName string) (kubernetes.Interface, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kube config: %w", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build kubernetes client: %w", err)
	}
	return client, nil
}
