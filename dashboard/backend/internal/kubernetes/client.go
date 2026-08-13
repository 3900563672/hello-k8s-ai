package kubernetes

import (
	"fmt"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Clients struct {
	Kubernetes kubernetes.Interface
	Dynamic    dynamic.Interface
	RESTConfig *rest.Config
	Context    string
	Version    string
}

func NewClients(cfg config.KubernetesConfig) (*Clients, error) {
	restConfig, contextName, err := buildRESTConfig(cfg)
	if err != nil {
		return nil, err
	}
	restConfig.QPS = cfg.QPS
	restConfig.Burst = cfg.Burst
	restConfig.UserAgent = "hello-k8s-ai-dashboard-backend/1.0"

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create dynamic Kubernetes client: %w", err)
	}

	version := discoverServerVersion(restConfig)
	return &Clients{
		Kubernetes: kubeClient,
		Dynamic:    dynamicClient,
		RESTConfig: restConfig,
		Context:    contextName,
		Version:    version,
	}, nil
}

func discoverServerVersion(restConfig *rest.Config) string {
	discoveryConfig := rest.CopyConfig(restConfig)
	discoveryConfig.Timeout = 5 * time.Second
	discoveryClient, err := kubernetes.NewForConfig(discoveryConfig)
	if err != nil {
		return ""
	}
	info, err := discoveryClient.Discovery().ServerVersion()
	if err != nil {
		return ""
	}
	return info.GitVersion
}

func buildRESTConfig(cfg config.KubernetesConfig) (*rest.Config, string, error) {
	if strings.TrimSpace(cfg.Kubeconfig) == "" {
		if inCluster, err := rest.InClusterConfig(); err == nil {
			return inCluster, "in-cluster", nil
		}
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if cfg.Kubeconfig != "" {
		rules.ExplicitPath = cfg.Kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if cfg.Context != "" {
		overrides.CurrentContext = cfg.Context
	}
	deferred := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	restConfig, err := deferred.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("load Kubernetes client configuration: %w", err)
	}
	raw, err := deferred.RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("read Kubernetes context: %w", err)
	}
	return restConfig, raw.CurrentContext, nil
}
