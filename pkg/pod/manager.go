package pod

import (
	"context"
	"fmt"

	fgtechv1 "github.com/fgtech/ia/cursor/api/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Manager manages Pods associated to Fgtech resources.
type Manager struct {
	client            client.Client
	scheme            *runtime.Scheme
	defaultTTLSeconds int64
	defaultSA         string
	defaultPort       int32
}

func NewManager(c client.Client, scheme *runtime.Scheme, defaultTTLSeconds int64, defaultSA string, defaultPort int32) *Manager {
	return &Manager{client: c, scheme: scheme, defaultTTLSeconds: defaultTTLSeconds, defaultSA: defaultSA, defaultPort: defaultPort}
}

// Ensure makes sure the Pod and Service backing the provided Fgtech exist and match its spec.
func (m *Manager) Ensure(ctx context.Context, fg *fgtechv1.Fgtech, log logr.Logger) (ctrl.Result, error) {
	podName := PodNameFor(fg)
	podKey := types.NamespacedName{Name: podName, Namespace: fg.Namespace}

	var existingPod corev1.Pod
	if err := m.client.Get(ctx, podKey, &existingPod); err != nil {
		if apierrors.IsNotFound(err) {
			newPod := buildPod(fg, podName, m.defaultTTLSeconds, m.defaultSA, m.defaultPort)
			if err := controllerutil.SetControllerReference(fg, newPod, m.scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := m.client.Create(ctx, newPod); err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Pod created for fgtech", "pod", podName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if podNeedsUpdate(&existingPod, fg, m.defaultTTLSeconds, m.defaultSA, m.defaultPort) {
		if err := m.client.Delete(ctx, &existingPod); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Pod deleted to refresh configuration", "pod", existingPod.Name)
		return ctrl.Result{Requeue: true}, nil
	}

	if err := m.ensureService(ctx, fg, log); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func podNameFor(fg *fgtechv1.Fgtech) string {
	return fmt.Sprintf("%s-pod", fg.Name)
}

// PodNameFor exposes the generated Pod name for a Fgtech instance.
func PodNameFor(fg *fgtechv1.Fgtech) string {
	return podNameFor(fg)
}

func buildPod(fg *fgtechv1.Fgtech, podName string, defaultTTLSeconds int64, defaultSA string, defaultPort int32) *corev1.Pod {
	activeDeadline := ttlPointer(fg, defaultTTLSeconds)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: fg.Namespace,
			Labels: map[string]string{
				"app":            "fgtech",
				"fgtech-name":    fg.Name,
				"fgtech-version": fg.Spec.Version,
			},
		},
		Spec: corev1.PodSpec{
			ActiveDeadlineSeconds: activeDeadline,
			ServiceAccountName:    resolveServiceAccount(fg, defaultSA),
			Volumes: []corev1.Volume{
				{
					Name: "kube-config",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "k8sconfig",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "fgtech",
					Image:           fg.Spec.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env: []corev1.EnvVar{
						{
							Name:  "FGTECH_VERSION",
							Value: fg.Spec.Version,
						},
					},
					Command: []string{"sh", "-c", "while true; do echo fgtech running; sleep 30; done"},
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: defaultPort,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "kube-config",
							MountPath: "/home/clovers/.kube",
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
}

func podNeedsUpdate(pod *corev1.Pod, fg *fgtechv1.Fgtech, defaultTTLSeconds int64, defaultSA string, defaultPort int32) bool {
	if len(pod.Spec.Containers) == 0 {
		return true
	}

	container := pod.Spec.Containers[0]
	if container.Image != fg.Spec.Image {
		return true
	}

	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != defaultPort {
		return true
	}

	hasVersionEnv := false
	for _, env := range container.Env {
		if env.Name == "FGTECH_VERSION" {
			hasVersionEnv = true
			if env.Value != fg.Spec.Version {
				return true
			}
		}
	}
	if !hasVersionEnv {
		return true
	}

	if pod.Labels["fgtech-version"] != fg.Spec.Version {
		return true
	}

	expectedTTL := ttlPointer(fg, defaultTTLSeconds)
	if (pod.Spec.ActiveDeadlineSeconds == nil) != (expectedTTL == nil) {
		return true
	}
	if expectedTTL != nil && pod.Spec.ActiveDeadlineSeconds != nil && *pod.Spec.ActiveDeadlineSeconds != *expectedTTL {
		return true
	}

	if pod.Spec.ServiceAccountName != resolveServiceAccount(fg, defaultSA) {
		return true
	}

	return false
}

func (m *Manager) ensureService(ctx context.Context, fg *fgtechv1.Fgtech, log logr.Logger) error {
	serviceName := ServiceNameFor(fg)
	serviceKey := types.NamespacedName{Name: serviceName, Namespace: fg.Namespace}
	var svc corev1.Service
	if err := m.client.Get(ctx, serviceKey, &svc); err != nil {
		if apierrors.IsNotFound(err) {
			newSvc := buildService(fg, serviceName, m.defaultPort)
			if err := controllerutil.SetControllerReference(fg, newSvc, m.scheme); err != nil {
				return err
			}
			if err := m.client.Create(ctx, newSvc); err != nil {
				return err
			}
			log.Info("Service created for fgtech", "service", serviceName)
			return nil
		}
		return err
	}

	if serviceNeedsUpdate(&svc, fg, m.defaultPort) {
		updatedSvc := svc.DeepCopy()
		updateServiceFields(updatedSvc, fg, m.defaultPort)
		if err := m.client.Update(ctx, updatedSvc); err != nil {
			return err
		}
		log.Info("Service updated for fgtech", "service", serviceName)
	}

	return nil
}

func ServiceNameFor(fg *fgtechv1.Fgtech) string {
	return fmt.Sprintf("%s-svc", fg.Name)
}

func buildService(fg *fgtechv1.Fgtech, serviceName string, defaultPort int32) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: fg.Namespace,
			Labels: map[string]string{
				"app":         "fgtech",
				"fgtech-name": fg.Name,
			},
		},
	}
	updateServiceFields(svc, fg, defaultPort)
	return svc
}

func updateServiceFields(svc *corev1.Service, fg *fgtechv1.Fgtech, defaultPort int32) {
	svc.Spec.Selector = map[string]string{
		"fgtech-name": fg.Name,
	}
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt(int(defaultPort)),
		},
	}
	svc.Spec.Type = corev1.ServiceTypeClusterIP
}

func serviceNeedsUpdate(svc *corev1.Service, fg *fgtechv1.Fgtech, defaultPort int32) bool {
	expectedSelector := map[string]string{
		"fgtech-name": fg.Name,
	}
	if !mapsEqual(svc.Spec.Selector, expectedSelector) {
		return true
	}

	if len(svc.Spec.Ports) != 1 {
		return true
	}
	port := svc.Spec.Ports[0]
	if port.Port != 80 || port.TargetPort.IntValue() != int(defaultPort) {
		return true
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		return true
	}

	return false
}

func ttlPointer(fg *fgtechv1.Fgtech, defaultTTLSeconds int64) *int64 {
	ttl := ResolveTTLSeconds(fg, defaultTTLSeconds)
	if ttl <= 0 {
		return nil
	}
	return int64Ptr(ttl)
}

// ResolveTTLSeconds returns the effective TTL in seconds for a Fgtech,
// preferring the spec override when positive, otherwise the provided default.
// Returns 0 when no TTL should be applied.
func ResolveTTLSeconds(fg *fgtechv1.Fgtech, defaultTTLSeconds int64) int64 {
	if fg.Spec.TTLSeconds != nil && *fg.Spec.TTLSeconds > 0 {
		return *fg.Spec.TTLSeconds
	}
	if defaultTTLSeconds > 0 {
		return defaultTTLSeconds
	}
	return 0
}

func resolveServiceAccount(fg *fgtechv1.Fgtech, defaultSA string) string {
	if fg.Spec.ServiceAccount != "" {
		return fg.Spec.ServiceAccount
	}
	if defaultSA != "" {
		return defaultSA
	}
	return "default"
}

func int64Ptr(v int64) *int64 {
	return &v
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
