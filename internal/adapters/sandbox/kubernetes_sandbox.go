package sandbox

import (
	"context"
	"dev.rubentxu.devops-platform/core/domain"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"net/url"
	"strconv"
)

type KubernetesConfig struct {
	Namespace string // Namespace de Kubernetes
	Proxy     string // URL del proxy (opcional)
	SkipSSL   bool   // Omitir verificación SSL (opcional)
}

type KubernetesContainerSandboxService struct {
	clientset *kubernetes.Clientset
	namespace string
}

func NewKubernetesContainerSandboxService(config KubernetesConfig) (*KubernetesContainerSandboxService, error) {
	// Configuración del cliente de Kubernetes
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %v", err)
	}

	// Configuración del proxy (si está presente)
	if config.Proxy != "" {
		proxy, err := url.Parse(config.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %v", err)
		}
		kubeConfig.Proxy = http.ProxyURL(proxy)
	}

	// Configuración de skip SSL (si está habilitado)
	if config.SkipSSL {
		kubeConfig.TLSClientConfig = rest.TLSClientConfig{
			Insecure: true, // Omitir verificación SSL
		}
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	return &KubernetesContainerSandboxService{
		clientset: clientset,
		namespace: config.Namespace,
	}, nil
}

func (k *KubernetesContainerSandboxService) CreateContainerSandbox(ctx context.Context, config *domain.SandboxConfig) (string, error) {

	cpuQuantity, err := parseCPU(config.Resources.CPU)
	if err != nil {
		return "", fmt.Errorf("failed to parse CPU: %v", err)
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "domain-",
			Labels: map[string]string{
				"app": "containerdomain",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:       "domain",
					Image:      config.Image,
					Env:        mapToEnvVarsK8s(config.EnvVars),
					WorkingDir: config.WorkingDir,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 50051,
						},
					},

					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"memory": parseMemoryK8s(config.Resources.Memory),
							"cpu":    cpuQuantity,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	createdPod, err := k.clientset.CoreV1().Pods(k.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create pod: %v", err)
	}

	return fmt.Sprintf("%s:50051", createdPod.Status.PodIP), nil
}

func mapToEnvVarsK8s(envVars map[string]string) []corev1.EnvVar {
	var env []corev1.EnvVar
	for key, value := range envVars {
		env = append(env, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}
	return env
}

func parseMemoryK8s(memory string) resource.Quantity {
	return resource.MustParse(memory)
}

func parseCPU(cpu string) (resource.Quantity, error) {
	cpuInt, err := strconv.ParseInt(cpu, 10, 64)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("failed to parse CPU: %v", err)
	}
	return *resource.NewQuantity(cpuInt, resource.DecimalSI), nil
}

func (k *KubernetesContainerSandboxService) DeleteContainerSandbox(ctx context.Context, sandboxID string) error {
	return k.clientset.CoreV1().Pods(k.namespace).Delete(ctx, sandboxID, metav1.DeleteOptions{})
}

func (k *KubernetesContainerSandboxService) ListContainerSandboxes(ctx context.Context) ([]string, error) {
	pods, err := k.clientset.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=containersandbox",
	})
	if err != nil {
		return nil, err
	}

	var sandboxAddresses []string
	for _, pod := range pods.Items {
		sandboxAddresses = append(sandboxAddresses, fmt.Sprintf("%s:50051", pod.Status.PodIP))
	}
	return sandboxAddresses, nil
}
