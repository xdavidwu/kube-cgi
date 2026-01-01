package v1alpha1

import (
	"net/http"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kcgischema "github.com/xdavidwu/kube-cgi/internal/schema"
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

var fakeHandleFunc = func(_ http.ResponseWriter, _ *http.Request) {}

func tryRegisterPattern(s *http.ServeMux, p string) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()
	s.HandleFunc(p, fakeHandleFunc)
	return
}

func (r APISet) validate() (admission.Warnings, error) {
	tMux := &http.ServeMux{}
	errs := []*field.Error{}
	path := field.NewPath("spec", "apis")
	for i, api := range r.Spec.APIs {
		p := path.Index(i)
		if err := tryRegisterPattern(tMux, api.Path); err != nil {
			errs = append(errs, field.Invalid(
				p.Child("path"),
				api.Path,
				err.Error(),
			))
		}

		if api.Request != nil && api.Request.Schema != nil {
			_, err := kcgischema.CompileString(api.Request.Schema.RawJSON)
			if err != nil {
				errs = append(errs, field.Invalid(
					p.Child("request", "schema"),
					api.Request.Schema.RawJSON,
					err.Error(),
				))
			}
		}
	}

	if len(errs) != 0 {
		return nil, errors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: "APISet"},
			r.Name, errs)
	}
	return nil, nil
}
