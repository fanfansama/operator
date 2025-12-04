package controllers

import (
	"context"

	fgtechv1 "github.com/fgtech/ia/cursor/api/v1"
	"github.com/fgtech/ia/cursor/pkg/ingress"
	"github.com/fgtech/ia/cursor/pkg/pod"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// FgtechReconciler reconciles a Fgtech object
// +kubebuilder:rbac:groups=fgtech.fgtech.io,resources=fgteches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fgtech.fgtech.io,resources=fgteches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fgtech.fgtech.io,resources=fgteches/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
type FgtechReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Log               logr.Logger
	podMgr            *pod.Manager
	ingressMgr        *ingress.Manager
	IngressHost       string
	IngressTLSSecret  string
	IngressClassName  string
	DefaultTTLSeconds int64
	DefaultSA         string
	DefaultPodPort    int32
}

func (r *FgtechReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("fgtech", req.NamespacedName)

	var fgtech fgtechv1.Fgtech
	if err := r.Get(ctx, req.NamespacedName, &fgtech); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.ingressManager().SyncNamespace(ctx, req.Namespace, log); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	podResult, err := r.podManager().Ensure(ctx, &fgtech, log)
	if err != nil {
		return ctrl.Result{}, err
	}
	if podResult.Requeue || podResult.RequeueAfter > 0 {
		return podResult, nil
	}

	if err := r.ingressManager().SyncNamespace(ctx, fgtech.Namespace, log); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *FgtechReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			r.Log.Info("ajout", "name", e.Object.GetName(), "namespace", e.Object.GetNamespace())
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			r.Log.Info("modification", "name", e.ObjectNew.GetName(), "namespace", e.ObjectNew.GetNamespace())
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			r.Log.Info("supprission", "name", e.Object.GetName(), "namespace", e.Object.GetNamespace())
			return true
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&fgtechv1.Fgtech{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		WithEventFilter(pred).
		Complete(r)
}

func (r *FgtechReconciler) podManager() *pod.Manager {
	if r.podMgr == nil {
		r.podMgr = pod.NewManager(r.Client, r.Scheme, r.DefaultTTLSeconds, r.DefaultSA, r.DefaultPodPort)
	}
	return r.podMgr
}

func (r *FgtechReconciler) ingressManager() *ingress.Manager {
	if r.ingressMgr == nil {
		r.ingressMgr = ingress.NewManager(r.Client, r.IngressHost, r.IngressTLSSecret, r.IngressClassName)
	}
	return r.ingressMgr
}
