package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
)

// APISetReconciler reconciles a APISet object
type APISetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=fluorescence.aic.cs.nycu.edu.tw,resources=apisets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=fluorescence.aic.cs.nycu.edu.tw,resources=apisets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=fluorescence.aic.cs.nycu.edu.tw,resources=apisets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the APISet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *APISetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *APISetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fluorescencev1alpha1.APISet{}).
		Complete(r)
}
