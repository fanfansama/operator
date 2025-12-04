package pod

import (
	"context"
	"testing"

	fgtechv1 "github.com/fgtech/ia/cursor/api/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func newPodScheme(t *testing.T) *runtime.Scheme {
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

func TestEnsureCreatesExpectedPod(t *testing.T) {
	tests := []struct {
		name              string
		specTTL           *int64
		defaultTTLSeconds int64
		wantTTLSeconds    int64
		specSA            string
		defaultSA         string
		wantSA            string
		defaultPort       int32
	}{
		{name: "default ttl applied", specTTL: nil, defaultTTLSeconds: 3600, wantTTLSeconds: 3600, specSA: "", defaultSA: "default", wantSA: "default", defaultPort: 8080},
		{name: "override ttl", specTTL: int64Ptr(7200), defaultTTLSeconds: 3600, wantTTLSeconds: 7200, specSA: "custom-sa", defaultSA: "default", wantSA: "custom-sa", defaultPort: 8080},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fg := &fgtechv1.Fgtech{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo",
					Namespace: "default",
				},
				Spec: fgtechv1.FgtechSpec{
					Version:        "1.0.0",
					Image:          "nginx:latest",
					TTLSeconds:     tt.specTTL,
					ServiceAccount: tt.specSA,
				},
			}

			scheme := newPodScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()
			mgr := NewManager(cl, scheme, tt.defaultTTLSeconds, tt.defaultSA, tt.defaultPort)

			if _, err := mgr.Ensure(context.Background(), fg, logr.Discard()); err != nil {
				t.Fatalf("Ensure returned error: %v", err)
			}

			var pod corev1.Pod
			if err := cl.Get(context.Background(), client.ObjectKey{Namespace: fg.Namespace, Name: PodNameFor(fg)}, &pod); err != nil {
				t.Fatalf("expected pod to be created: %v", err)
			}

			if pod.Spec.ActiveDeadlineSeconds == nil {
				t.Fatalf("expected ActiveDeadlineSeconds to be set")
			}
			if *pod.Spec.ActiveDeadlineSeconds != tt.wantTTLSeconds {
				t.Fatalf("ActiveDeadlineSeconds = %d, want %d", *pod.Spec.ActiveDeadlineSeconds, tt.wantTTLSeconds)
			}

			if len(pod.Spec.Containers) != 1 {
				t.Fatalf("expected 1 container, got %d", len(pod.Spec.Containers))
			}
			c := pod.Spec.Containers[0]
			if c.Image != fg.Spec.Image {
				t.Fatalf("container image = %s, want %s", c.Image, fg.Spec.Image)
			}
			if !envContains(c.Env, "FGTECH_VERSION", fg.Spec.Version) {
				t.Fatalf("expected env FGTECH_VERSION=%s", fg.Spec.Version)
			}
			if len(c.Ports) != 1 || c.Ports[0].ContainerPort != tt.defaultPort {
				t.Fatalf("expected one http port %d, got %v", tt.defaultPort, c.Ports)
			}

			if pod.Spec.ServiceAccountName != tt.wantSA {
				t.Fatalf("service account = %s, want %s", pod.Spec.ServiceAccountName, tt.wantSA)
			}
		})
	}
}

func envContains(env []corev1.EnvVar, name, value string) bool {
	for _, e := range env {
		if e.Name == name && e.Value == value {
			return true
		}
	}
	return false
}

func TestPodManifestMatchesExpectedYAML(t *testing.T) {
	fg := &fgtechv1.Fgtech{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: fgtechv1.FgtechSpec{
			Version:        "1.0.0",
			Image:          "nginx:latest",
			ExtraPath:      "/api/v1",
			ServiceAccount: "pro",
			TTLSeconds:     int64Ptr(3600),
		},
	}

	defaultPort := int32(8182)
	scheme := newPodScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	mgr := NewManager(cl, scheme, 3600, "default-sa", defaultPort)

	if _, err := mgr.Ensure(context.Background(), fg, logr.Discard()); err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}

	var pod corev1.Pod
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: fg.Namespace, Name: PodNameFor(fg)}, &pod); err != nil {
		t.Fatalf("expected pod to be created: %v", err)
	}
	pod.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
	pod.ResourceVersion = ""
	pod.ManagedFields = nil

	actualYAML, err := yaml.Marshal(&pod)
	if err != nil {
		t.Fatalf("marshal pod: %v", err)
	}

	expectedYAML := `apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  labels:
    app: fgtech
    fgtech-name: demo
    fgtech-version: 1.0.0
  name: demo-pod
  namespace: default
  ownerReferences:
  - apiVersion: fgtech.fgtech.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: Fgtech
    name: demo
    uid: ""
spec:
  activeDeadlineSeconds: 3600
  containers:
  - command:
    - sh
    - -c
    - while true; do echo fgtech running; sleep 30; done
    env:
    - name: FGTECH_VERSION
      value: 1.0.0
    image: nginx:latest
    imagePullPolicy: IfNotPresent
    name: fgtech
    ports:
    - containerPort: 8182
      name: http
    resources: {}
    volumeMounts:
    - mountPath: /home/clovers/.kube
      name: kube-config
  restartPolicy: Always
  serviceAccountName: pro
  volumes:
  - name: kube-config
    secret:
      secretName: k8sconfig
status: {}
`

	if string(actualYAML) != expectedYAML {
		t.Fatalf("generated pod yaml differs.\nGot:\n%s\nWant:\n%s", string(actualYAML), expectedYAML)
	}
}

func TestPodIncludesKubeConfigVolumeAndMount(t *testing.T) {
	fg := &fgtechv1.Fgtech{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: fgtechv1.FgtechSpec{
			Version: "1.0.0",
			Image:   "nginx:latest",
		},
	}

	scheme := newPodScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	mgr := NewManager(cl, scheme, 3600, "default", 8080)

	if _, err := mgr.Ensure(context.Background(), fg, logr.Discard()); err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}

	var pod corev1.Pod
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: fg.Namespace, Name: PodNameFor(fg)}, &pod); err != nil {
		t.Fatalf("expected pod to be created: %v", err)
	}

	if len(pod.Spec.Volumes) != 1 || pod.Spec.Volumes[0].Name != "kube-config" {
		t.Fatalf("expected kube-config volume, got %v", pod.Spec.Volumes)
	}
	if pod.Spec.Volumes[0].Secret == nil || pod.Spec.Volumes[0].Secret.SecretName != "k8sconfig" {
		t.Fatalf("expected kube-config secret volume, got %v", pod.Spec.Volumes[0].Secret)
	}
	c := pod.Spec.Containers[0]
	if len(c.VolumeMounts) != 1 || c.VolumeMounts[0].Name != "kube-config" || c.VolumeMounts[0].MountPath != "/home/clovers/.kube" {
		t.Fatalf("expected kube-config mount, got %v", c.VolumeMounts)
	}
}

func TestServiceManifestMatchesExpectedYAML(t *testing.T) {
	fg := &fgtechv1.Fgtech{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "test",
		},
		Spec: fgtechv1.FgtechSpec{
			Version:   "1.0.0",
			Image:     "nginx:latest",
			ExtraPath: "/api/v1",
		},
	}

	defaultPort := int32(8182)
	scheme := newPodScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	mgr := NewManager(cl, scheme, 3600, "test-sa", defaultPort)

	// First call creates the Pod, second ensures the Service.
	if _, err := mgr.Ensure(context.Background(), fg, logr.Discard()); err != nil {
		t.Fatalf("Ensure returned error: %v", err)
	}
	if _, err := mgr.Ensure(context.Background(), fg, logr.Discard()); err != nil {
		t.Fatalf("Ensure returned error on second pass: %v", err)
	}

	var svc corev1.Service
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: fg.Namespace, Name: ServiceNameFor(fg)}, &svc); err != nil {
		t.Fatalf("expected service to be created: %v", err)
	}
	svc.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Service"}
	svc.ResourceVersion = ""
	svc.ManagedFields = nil

	actualYAML, err := yaml.Marshal(&svc)
	if err != nil {
		t.Fatalf("marshal service: %v", err)
	}

	expectedYAML := `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: fgtech
    fgtech-name: demo
  name: demo-svc
  namespace: test
  ownerReferences:
  - apiVersion: fgtech.fgtech.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: Fgtech
    name: demo
    uid: ""
spec:
  ports:
  - name: http
    port: 80
    targetPort: 8182
  selector:
    fgtech-name: demo
  type: ClusterIP
status:
  loadBalancer: {}
`

	if string(actualYAML) != expectedYAML {
		t.Fatalf("generated service yaml differs.\nGot:\n%s\nWant:\n%s", string(actualYAML), expectedYAML)
	}
}
