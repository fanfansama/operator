package ingress

import (
	"context"
	"testing"

	fgtechv1 "github.com/fgtech/ia/cursor/api/v1"
	"github.com/go-logr/logr"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func TestBuildRoutePath(t *testing.T) {
	tests := []struct {
		name      string
		extraPath string
		appName   string
		want      string
	}{
		{name: "empty extra path", extraPath: "", appName: "demo", want: "/demo"},
		{name: "slash only", extraPath: "/", appName: "demo", want: "/demo"},
		{name: "simple path no slashes", extraPath: "apps", appName: "demo", want: "/apps/demo"},
		{name: "leading slash", extraPath: "/apps", appName: "demo", want: "/apps/demo"},
		{name: "trailing slash", extraPath: "apps/", appName: "demo", want: "/apps/demo"},
		{name: "leading and trailing", extraPath: "/apps/", appName: "demo", want: "/apps/demo"},
		{name: "nested path", extraPath: "/apps/v1/", appName: "demo", want: "/apps/v1/demo"},
		{name: "extra whitespace and slashes", extraPath: "  //apps//  ", appName: "demo", want: "/apps/demo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRoutePath(tt.extraPath, tt.appName)
			if got != tt.want {
				t.Fatalf("buildRoutePath(%q, %q) = %q, want %q", tt.extraPath, tt.appName, got, tt.want)
			}
		})
	}
}

func TestSyncNamespaceBuildsIngressYAML(t *testing.T) {
	scheme := newIngressScheme(t)
	fg1 := &fgtechv1.Fgtech{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha",
			Namespace: "demo",
		},
		Spec: fgtechv1.FgtechSpec{
			Image:     "nginx:1.25",
			Version:   "1.0.0",
			ExtraPath: "/apps",
		},
	}
	fg2 := &fgtechv1.Fgtech{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "beta",
			Namespace: "demo",
		},
		Spec: fgtechv1.FgtechSpec{
			Image:     "nginx:1.25",
			Version:   "2.0.0",
			ExtraPath: "/api/v1",
		},
	}
	defaultNamespace := "demo"
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(fg1, fg2).Build()
	mgr := NewManager(cl, "apps.example.com", "fgtech-tls", "nginx")

	if err := mgr.SyncNamespace(context.Background(), defaultNamespace, logr.Discard()); err != nil {
		t.Fatalf("SyncNamespace error: %v", err)
	}

	var ing networkingv1.Ingress
	if err := cl.Get(context.Background(), types.NamespacedName{Name: ingressName, Namespace: defaultNamespace}, &ing); err != nil {
		t.Fatalf("ingress not found: %v", err)
	}
	ing.TypeMeta = metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "Ingress"}
	ing.ResourceVersion = ""
	ing.ManagedFields = nil

	actualYAML, err := yaml.Marshal(&ing)
	if err != nil {
		t.Fatalf("marshal ingress: %v", err)
	}

	expected := `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  creationTimestamp: null
  labels:
    app: fgtech
  name: fgtech-global-ingress
  namespace: demo
spec:
  defaultBackend:
    service:
      name: fgtech-fake-backend
      port:
        number: 80
  ingressClassName: nginx
  rules:
  - host: apps.example.com
    http:
      paths:
      - backend:
          service:
            name: beta-svc
            port:
              number: 80
        path: /api/v1/beta
        pathType: Prefix
      - backend:
          service:
            name: alpha-svc
            port:
              number: 80
        path: /apps/alpha
        pathType: Prefix
  tls:
  - hosts:
    - apps.example.com
    secretName: fgtech-tls
status:
  loadBalancer: {}
`

	if string(actualYAML) != expected {
		t.Fatalf("ingress yaml mismatch\nGot:\n%s\nWant:\n%s", string(actualYAML), expected)
	}
}

func newIngressScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client scheme: %v", err)
	}
	if err := fgtechv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add fgtech scheme: %v", err)
	}
	return scheme
}
