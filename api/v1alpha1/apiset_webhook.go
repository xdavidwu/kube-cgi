package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var apisetlog = logf.Log.WithName("apiset-resource")

func (r *APISet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-fluorescence-aic-cs-nycu-edu-tw-v1alpha1-apiset,mutating=false,failurePolicy=fail,sideEffects=None,groups=fluorescence.aic.cs.nycu.edu.tw,resources=apisets,verbs=create;update,versions=v1alpha1,name=vapiset.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &APISet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *APISet) ValidateCreate() (admission.Warnings, error) {
	apisetlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *APISet) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	apisetlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *APISet) ValidateDelete() (admission.Warnings, error) {
	apisetlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
