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
	client client.Client
	scheme *runtime.Scheme
}

func NewManager(c client.Client, scheme *runtime.Scheme) *Manager {
	return &Manager{client: c, scheme: scheme}
}

// Ensure makes sure the Pod and Service backing the provided Fgtech exist and match its spec.
func (m *Manager) Ensure(ctx context.Context, fg *fgtechv1.Fgtech, log logr.Logger) (ctrl.Result, error) {
	podName := podNameFor(fg)
	podKey := types.NamespacedName{Name: podName, Namespace: fg.Namespace}

	var existingPod corev1.Pod
	if err := m.client.Get(ctx, podKey, &existingPod); err != nil {
		if apierrors.IsNotFound(err) {
			newPod := buildPod(fg, podName)
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

	if podNeedsUpdate(&existingPod, fg) {
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

func buildPod(fg *fgtechv1.Fgtech, podName string) *corev1.Pod {
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
							ContainerPort: 80,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
		},
	}
}

func podNeedsUpdate(pod *corev1.Pod, fg *fgtechv1.Fgtech) bool {
	if len(pod.Spec.Containers) == 0 {
		return true
	}

	container := pod.Spec.Containers[0]
	if container.Image != fg.Spec.Image {
		return true
	}

	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 80 {
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

	return false
}

func (m *Manager) ensureService(ctx context.Context, fg *fgtechv1.Fgtech, log logr.Logger) error {
	serviceName := ServiceNameFor(fg)
	serviceKey := types.NamespacedName{Name: serviceName, Namespace: fg.Namespace}
	var svc corev1.Service
	if err := m.client.Get(ctx, serviceKey, &svc); err != nil {
		if apierrors.IsNotFound(err) {
			newSvc := buildService(fg, serviceName)
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

	if serviceNeedsUpdate(&svc, fg) {
		updatedSvc := svc.DeepCopy()
		updateServiceFields(updatedSvc, fg)
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

func buildService(fg *fgtechv1.Fgtech, serviceName string) *corev1.Service {
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
	updateServiceFields(svc, fg)
	return svc
}

func updateServiceFields(svc *corev1.Service, fg *fgtechv1.Fgtech) {
	svc.Spec.Selector = map[string]string{
		"fgtech-name": fg.Name,
	}
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			TargetPort: intstr.FromInt(80),
		},
	}
	svc.Spec.Type = corev1.ServiceTypeClusterIP
}

func serviceNeedsUpdate(svc *corev1.Service, fg *fgtechv1.Fgtech) bool {
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
	if port.Port != 80 || port.TargetPort.IntValue() != 80 {
		return true
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		return true
	}

	return false
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
