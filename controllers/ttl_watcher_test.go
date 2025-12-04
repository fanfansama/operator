package controllers

import (
	"context"
	"testing"
	"time"

	fgtechv1 "github.com/fgtech/ia/cursor/api/v1"
	"github.com/fgtech/ia/cursor/pkg/pod"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add clientgoscheme: %v", err)
	}
	if err := fgtechv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add fgtech scheme: %v", err)
	}
	return scheme
}

func TestTTLWatcherDeletesExpiredResources(t *testing.T) {
	now := time.Now()
	fg := &fgtechv1.Fgtech{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "demo",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(now.Add(-2 * time.Hour)),
		},
	}
	podObj := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pod.PodNameFor(fg), Namespace: fg.Namespace}}
	svcObj := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: pod.ServiceNameFor(fg), Namespace: fg.Namespace}}

	cl := fake.NewClientBuilder().WithScheme(newScheme(t)).WithRuntimeObjects(fg, podObj, svcObj).Build()
	w := &ttlWatcher{
		client:            cl,
		log:               logr.Discard(),
		defaultTTLSeconds: 3600,
		ingressHost:       "example.com",
		ingressTLSSecret:  "fgtech-tls",
	}

	if err := w.sweep(context.Background(), now); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	assertNotFound := func(obj clientObject, desc string) {
		err := cl.Get(context.Background(), client.ObjectKey{Namespace: fg.Namespace, Name: obj.GetName()}, obj)
		if err == nil {
			t.Fatalf("%s still exists", desc)
		}
	}

	assertNotFound(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pod.PodNameFor(fg), Namespace: fg.Namespace}}, "pod")
	assertNotFound(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: pod.ServiceNameFor(fg), Namespace: fg.Namespace}}, "service")
	assertNotFound(&fgtechv1.Fgtech{ObjectMeta: metav1.ObjectMeta{Name: fg.Name, Namespace: fg.Namespace}}, "fgtech")
}

func TestTTLWatcherKeepsNonExpiredResources(t *testing.T) {
	now := time.Now()
	fg := &fgtechv1.Fgtech{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "demo",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Minute)),
		},
		Spec: fgtechv1.FgtechSpec{
			TTLSeconds: int64Ptr(7200),
		},
	}

	cl := fake.NewClientBuilder().WithScheme(newScheme(t)).WithRuntimeObjects(fg).Build()
	w := &ttlWatcher{
		client:            cl,
		log:               logr.Discard(),
		defaultTTLSeconds: 3600,
		ingressHost:       "example.com",
		ingressTLSSecret:  "fgtech-tls",
	}

	if err := w.sweep(context.Background(), now); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	var got fgtechv1.Fgtech
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: fg.Namespace, Name: fg.Name}, &got); err != nil {
		t.Fatalf("expected fgtech to remain, got err: %v", err)
	}
}

type clientObject interface {
	runtime.Object
	metav1.Object
}

// int64Ptr reused from pod/manager.go but kept private here for tests.
func int64Ptr(v int64) *int64 {
	return &v
}
