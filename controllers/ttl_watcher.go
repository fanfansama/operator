package controllers

import (
	"context"
	"time"

	fgtechv1 "github.com/fgtech/ia/cursor/api/v1"
	"github.com/fgtech/ia/cursor/pkg/ingress"
	"github.com/fgtech/ia/cursor/pkg/pod"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const ttlWatcherInterval = time.Minute

// ttlWatcher periodically cleans up expired Fgtech resources.
type ttlWatcher struct {
	client            client.Client
	log               logr.Logger
	defaultTTLSeconds int64
	ingressHost       string
	ingressTLSSecret  string
	ingressClassName  string
}

// NewTTLWatcher registers a periodic cleanup task that removes expired resources.
func NewTTLWatcher(c client.Client, log logr.Logger, defaultTTLSeconds int64, ingressHost, ingressTLSSecret, ingressClassName string) manager.Runnable {
	return &ttlWatcher{
		client:            c,
		log:               log,
		defaultTTLSeconds: defaultTTLSeconds,
		ingressHost:       ingressHost,
		ingressTLSSecret:  ingressTLSSecret,
		ingressClassName:  ingressClassName,
	}
}

// Start implements manager.Runnable.
func (w *ttlWatcher) Start(ctx context.Context) error {
	ticker := time.NewTicker(ttlWatcherInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := w.sweep(ctx, time.Now()); err != nil {
				w.log.Error(err, "ttl sweep failed")
			}
		}
	}
}

func (w *ttlWatcher) sweep(ctx context.Context, now time.Time) error {
	var list fgtechv1.FgtechList
	if err := w.client.List(ctx, &list); err != nil {
		return err
	}

	namespacesToSync := make(map[string]struct{})
	for i := range list.Items {
		item := list.Items[i]
		ttl := pod.ResolveTTLSeconds(&item, w.defaultTTLSeconds)
		if ttl <= 0 {
			continue
		}
		expiry := item.CreationTimestamp.Time.Add(time.Duration(ttl) * time.Second)
		if now.Before(expiry) {
			continue
		}
		if err := w.cleanup(ctx, &item); err != nil {
			w.log.Error(err, "failed to cleanup expired fgtech", "name", item.Name, "namespace", item.Namespace)
			continue
		}
		namespacesToSync[item.Namespace] = struct{}{}
	}

	if len(namespacesToSync) > 0 {
		ingMgr := ingress.NewManager(w.client, w.ingressHost, w.ingressTLSSecret, w.ingressClassName)
		for ns := range namespacesToSync {
			if err := ingMgr.SyncNamespace(ctx, ns, w.log); err != nil {
				w.log.Error(err, "failed to sync ingress after ttl cleanup", "namespace", ns)
			}
		}
	}

	return nil
}

func (w *ttlWatcher) cleanup(ctx context.Context, fg *fgtechv1.Fgtech) error {
	deleteIgnoreNotFound := func(obj client.Object) error {
		err := w.client.Delete(ctx, obj)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := deleteIgnoreNotFound(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.PodNameFor(fg),
			Namespace: fg.Namespace,
		},
	}); err != nil {
		return err
	}

	if err := deleteIgnoreNotFound(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.ServiceNameFor(fg),
			Namespace: fg.Namespace,
		},
	}); err != nil {
		return err
	}

	if err := deleteIgnoreNotFound(fg); err != nil {
		return err
	}

	return nil
}
