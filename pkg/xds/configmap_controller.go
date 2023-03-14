package xds

import (
	"context"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	LabelXDSKind = "xds.pf9.io/kind"
)

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

type ConfigMapReconciler struct {
	Client        client.Client
	Scheme        *runtime.Scheme
	SnapshotCache cache.SnapshotCache
	ConfigClient  ConfigClient
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	hasLabel, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      LabelXDSKind,
				Operator: metav1.LabelSelectorOpExists,
			},
		},
	})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(hasLabel).
		Complete(r)
}

func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling ConfigMap", "req", req)
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, req.NamespacedName, cm)
	if err != nil {
		return ctrl.Result{}, err
	}

	cfg, err := r.ConfigClient.Get(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}

	snapshot, err := cache.NewSnapshot(cm.ResourceVersion, ToMap(cfg))
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := snapshot.Consistent(); err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Setting snapshot", "cfg", cfg)
	err = r.SnapshotCache.SetSnapshot(ctx, cfg.Node.Id, snapshot)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
