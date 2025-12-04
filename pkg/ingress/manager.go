package ingress

import (
	"context"
	"fmt"
	"sort"
	"strings"

	fgtechv1 "github.com/fgtech/ia/cursor/api/v1"
	"github.com/fgtech/ia/cursor/pkg/pod"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ingressName             = "fgtech-global-ingress"
	defaultBackendName      = "fgtech-fake-backend"
	defaultBackendImage     = "nginxdemos/hello"
	defaultBackendContainer = "backend"
)

// Manager ensures a single ingress per namespace aggregates all Fgtech routes.
type Manager struct {
	client           client.Client
	host             string
	tlsSecret        string
	ingressClassName string
}

func NewManager(c client.Client, host, tlsSecret, ingressClassName string) *Manager {
	return &Manager{client: c, host: host, tlsSecret: tlsSecret, ingressClassName: ingressClassName}
}

// SyncNamespace reconciles the ingress for the provided namespace.
func (m *Manager) SyncNamespace(ctx context.Context, namespace string, log logr.Logger) error {
	if m.host == "" {
		return fmt.Errorf("FGTECH_INGRESS_FQDN env not set")
	}

	if err := m.ensureDefaultBackend(ctx, namespace); err != nil {
		return err
	}

	routes, err := m.collectRoutes(ctx, namespace)
	if err != nil {
		return err
	}

	key := types.NamespacedName{Name: ingressName, Namespace: namespace}
	var ing networkingv1.Ingress
	if err := m.client.Get(ctx, key, &ing); err != nil {
		if apierrors.IsNotFound(err) {
			newIng := m.buildIngress(namespace, routes)
			if err := m.client.Create(ctx, newIng); err != nil {
				return err
			}
			log.Info("Ingress created", "ingress", ingressName)
			return nil
		}
		return err
	}

	if m.needsUpdate(&ing, routes) {
		updated := ing.DeepCopy()
		m.applySpec(updated, routes)
		if err := m.client.Update(ctx, updated); err != nil {
			return err
		}
		log.Info("Ingress updated", "ingress", ingressName)
	}

	return nil
}

func (m *Manager) collectRoutes(ctx context.Context, namespace string) ([]networkingv1.HTTPIngressPath, error) {
	var list fgtechv1.FgtechList
	if err := m.client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	paths := make([]networkingv1.HTTPIngressPath, 0, len(list.Items))
	for i := range list.Items {
		item := list.Items[i]
		pathValue := buildRoutePath(item.Spec.ExtraPath, item.Name)
		backend := networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: pod.ServiceNameFor(&item),
				Port: networkingv1.ServiceBackendPort{Number: 80},
			},
		}
		paths = append(paths, networkingv1.HTTPIngressPath{
			Path:     pathValue,
			PathType: pathTypePtr(networkingv1.PathTypePrefix),
			Backend:  backend,
		})
	}

	sort.Slice(paths, func(i, j int) bool {
		return paths[i].Path < paths[j].Path
	})

	return paths, nil
}

func (m *Manager) buildIngress(namespace string, routes []networkingv1.HTTPIngressPath) *networkingv1.Ingress {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "fgtech",
			},
		},
	}
	m.applySpec(ing, routes)
	return ing
}

func (m *Manager) applySpec(ing *networkingv1.Ingress, routes []networkingv1.HTTPIngressPath) {
	if m.ingressClassName != "" {
		ing.Spec.IngressClassName = &m.ingressClassName
	}
	ing.Spec.DefaultBackend = &networkingv1.IngressBackend{
		Service: &networkingv1.IngressServiceBackend{
			Name: defaultBackendName,
			Port: networkingv1.ServiceBackendPort{Number: 80},
		},
	}

	if m.tlsSecret != "" {
		ing.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{m.host},
				SecretName: m.tlsSecret,
			},
		}
	}

	if len(routes) == 0 {
		ing.Spec.Rules = []networkingv1.IngressRule{
			{
				Host: m.host,
			},
		}
		return
	}

	ing.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: m.host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: routes,
				},
			},
		},
	}
}

func (m *Manager) needsUpdate(ing *networkingv1.Ingress, routes []networkingv1.HTTPIngressPath) bool {
	desired := &networkingv1.Ingress{}
	m.applySpec(desired, routes)
	return !ingressEqual(ing, desired)
}

func ingressEqual(existing, desired *networkingv1.Ingress) bool {
	if (existing.Spec.IngressClassName == nil) != (desired.Spec.IngressClassName == nil) {
		return false
	}
	if existing.Spec.IngressClassName != nil && desired.Spec.IngressClassName != nil && *existing.Spec.IngressClassName != *desired.Spec.IngressClassName {
		return false
	}
	if existing.Spec.DefaultBackend == nil || desired.Spec.DefaultBackend == nil {
		return false
	}
	if existing.Spec.DefaultBackend.Service == nil || desired.Spec.DefaultBackend.Service == nil {
		return false
	}
	if existing.Spec.DefaultBackend.Service.Name != desired.Spec.DefaultBackend.Service.Name {
		return false
	}
	if existing.Spec.DefaultBackend.Service.Port.Number != desired.Spec.DefaultBackend.Service.Port.Number {
		return false
	}

	if len(existing.Spec.TLS) != len(desired.Spec.TLS) {
		return false
	}
	for i := range desired.Spec.TLS {
		if existing.Spec.TLS[i].SecretName != desired.Spec.TLS[i].SecretName {
			return false
		}
		if len(existing.Spec.TLS[i].Hosts) != len(desired.Spec.TLS[i].Hosts) {
			return false
		}
		for j := range desired.Spec.TLS[i].Hosts {
			if existing.Spec.TLS[i].Hosts[j] != desired.Spec.TLS[i].Hosts[j] {
				return false
			}
		}
	}

	if len(existing.Spec.Rules) != len(desired.Spec.Rules) {
		return false
	}
	for i := range desired.Spec.Rules {
		if existing.Spec.Rules[i].Host != desired.Spec.Rules[i].Host {
			return false
		}
		existingHTTP := existing.Spec.Rules[i].HTTP
		desiredHTTP := desired.Spec.Rules[i].HTTP
		if (existingHTTP == nil) != (desiredHTTP == nil) {
			return false
		}
		if desiredHTTP == nil {
			continue
		}
		if len(existingHTTP.Paths) != len(desiredHTTP.Paths) {
			return false
		}
		for j := range desiredHTTP.Paths {
			if existingHTTP.Paths[j].Path != desiredHTTP.Paths[j].Path {
				return false
			}
			if existingHTTP.Paths[j].Backend.Service == nil || desiredHTTP.Paths[j].Backend.Service == nil {
				return false
			}
			if existingHTTP.Paths[j].Backend.Service.Name != desiredHTTP.Paths[j].Backend.Service.Name {
				return false
			}
			if existingHTTP.Paths[j].Backend.Service.Port.Number != desiredHTTP.Paths[j].Backend.Service.Port.Number {
				return false
			}
		}
	}

	return true
}

func buildRoutePath(extraPath, name string) string {
	base := strings.TrimSpace(extraPath)
	base = strings.Trim(base, "/")
	if base == "" {
		return "/" + name
	}
	return "/" + base + "/" + name
}

func pathTypePtr(p networkingv1.PathType) *networkingv1.PathType {
	return &p
}

func (m *Manager) ensureDefaultBackend(ctx context.Context, namespace string) error {
	if err := m.ensureBackendPod(ctx, namespace); err != nil {
		return err
	}
	if err := m.ensureBackendService(ctx, namespace); err != nil {
		return err
	}
	return nil
}

func (m *Manager) ensureBackendPod(ctx context.Context, namespace string) error {
	key := types.NamespacedName{Name: defaultBackendName, Namespace: namespace}
	var existing corev1.Pod
	if err := m.client.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultBackendName,
					Namespace: namespace,
					Labels: map[string]string{
						"app": defaultBackendName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  defaultBackendContainer,
							Image: defaultBackendImage,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 80},
							},
							Command: []string{"/bin/sh", "-c", "while true; do echo default backend; sleep 30; done"},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			}
			return m.client.Create(ctx, pod)
		}
		return err
	}
	return nil
}

func (m *Manager) ensureBackendService(ctx context.Context, namespace string) error {
	key := types.NamespacedName{Name: defaultBackendName, Namespace: namespace}
	var existing corev1.Service
	if err := m.client.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			svc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultBackendName,
					Namespace: namespace,
					Labels:    map[string]string{"app": defaultBackendName},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{"app": defaultBackendName},
					Ports: []corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
					},
				},
			}
			return m.client.Create(ctx, svc)
		}
		return err
	}
	return nil
}
