package v1alpha1

import (
	"github.com/santhosh-tekuri/jsonschema/v5"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

//+kubebuilder:webhook:path=/validate-kube-cgi-aic-cs-nycu-edu-tw-v1alpha1-apiset,mutating=false,failurePolicy=fail,sideEffects=None,groups=kube-cgi.aic.cs.nycu.edu.tw,resources=apisets,verbs=create;update,versions=v1alpha1,name=vapiset.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &APISet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *APISet) ValidateCreate() (admission.Warnings, error) {
	apisetlog.Info("validate create", "name", r.Name)

	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *APISet) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	apisetlog.Info("validate update", "name", r.Name)

	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *APISet) ValidateDelete() (admission.Warnings, error) {
	apisetlog.Info("validate delete", "name", r.Name)

	return nil, nil
}

func (r APISet) validate() (admission.Warnings, error) {
	errs := []*field.Error{}
	for i, api := range r.Spec.APIs {
		path := field.NewPath("spec", "apis")
		if api.Request != nil && api.Request.Schema != nil {
			_, err := jsonschema.CompileString("api.schema.json", api.Request.Schema.RawJSON)
			if err != nil {
				errs = append(errs, field.Invalid(
					path.Index(i).Child("request", "schema"),
					api.Request.Schema.RawJSON,
					err.(*jsonschema.SchemaError).Unwrap().Error(),
				))
			}
		}
	}

	if len(errs) != 0 {
		return nil, errors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: "APTSet"},
			r.Name, errs)
	}
	return nil, nil
}
