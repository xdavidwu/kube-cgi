package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal"
)

// APISetReconciler reconciles a APISet object
type APISetReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	DAPIImage  string
	PullSecret *corev1.Secret
}

//+kubebuilder:rbac:groups=fluorescence.aic.cs.nycu.edu.tw,resources=apisets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=fluorescence.aic.cs.nycu.edu.tw,resources=apisets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=fluorescence.aic.cs.nycu.edu.tw,resources=apisets/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=create;patch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=create;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;patch

// needed by dappy, set on manager to assign related rolebindings
//+kubebuilder:rbac:groups="",resources=pods,verbs=*
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch

const (
	fieldManager = "fluorescence"
	managedByKey = "app.kubernetes.io/managed-by"
	manager      = "fluorescence"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *APISetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var apiSet fluorescencev1alpha1.APISet
	err := r.Get(ctx, req.NamespacedName, &apiSet)
	if err != nil {
		log.Error(err, "cannot get requested APISet")
		return ctrl.Result{}, err
	}

	// XXX ssa needs gvk to be set, but this looks verbose
	serviceAccount := corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}

	roleBinding := rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "fluorescence-dappy",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      req.Name,
				Namespace: req.Namespace,
			},
		},
	}

	labels := map[string]string{"fluorescence.aic.cs.nycu.edu.tw/apiset": req.Namespace + "." + req.Name}
	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: r.DAPIImage,
							Name:  "dapi",
							Env: []corev1.EnvVar{
								{
									Name:  internal.DAPIEnvAPISetNamespace,
									Value: req.Namespace,
								},
								{
									Name:  internal.DAPIEnvAPISetName,
									Value: req.Name,
								},
								{
									Name:  internal.DAPIEnvAPISetResourceVersion,
									Value: apiSet.ObjectMeta.ResourceVersion,
								},
							},
						},
					},
					ServiceAccountName: req.Name,
				},
			},
		},
	}
	if apiSet.Spec.DAPI != nil {
		deployment.Spec.Replicas = apiSet.Spec.DAPI.Replicas
	}

	service := corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(internal.DAPIPort),
				},
			},
			Selector: labels,
		},
	}

	pathTypeExact := networkingv1.PathTypeExact
	paths := make([]networkingv1.HTTPIngressPath, len(apiSet.Spec.APIs))
	for i := range apiSet.Spec.APIs {
		paths[i] = networkingv1.HTTPIngressPath{
			Path:     apiSet.Spec.APIs[i].Path,
			PathType: &pathTypeExact,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: req.Name,
					Port: networkingv1.ServiceBackendPort{
						Number: 80,
					},
				},
			},
		}
	}
	ingress := networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: networkingv1.SchemeGroupVersion.String(),
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: apiSet.Spec.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: paths,
						},
					},
				},
			},
		},
	}

	type resource struct {
		obj       client.Object
		statusRef **corev1.ObjectReference
	}

	resources := []resource{
		{&serviceAccount, &apiSet.Status.ServiceAccount},
		{&roleBinding, &apiSet.Status.RoleBinding},
		{&deployment, &apiSet.Status.Deployment},
		{&service, &apiSet.Status.Service},
		{&ingress, &apiSet.Status.Ingress},
	}

	if r.PullSecret != nil {
		secret := r.PullSecret.DeepCopy()
		secret.TypeMeta = metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		}
		secret.ObjectMeta = metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
		}
		deployment.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: req.Name,
			},
		}

		resources = append(resources, resource{secret, &apiSet.Status.ImagePullSecret})
	}

	apiSet.Status.ObservedGeneration = apiSet.ObjectMeta.Generation
	defer func() {
		err2 := r.Status().Update(ctx, &apiSet)
		if err2 != nil {
			log.Error(err, "cannot update status", "APISet", apiSet)
		}
		if err == nil {
			err = err2
		}
	}()

	// XXX?
	soTrue := true
	for _, obj := range resources {
		err = ctrl.SetControllerReference(&apiSet, obj.obj, r.Scheme)
		if err != nil {
			log.Error(err, "cannot set owner", "object", obj.obj)
			return ctrl.Result{}, err
		}

		labels := obj.obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[managedByKey] = manager
		obj.obj.SetLabels(labels)

		err = r.Patch(ctx, obj.obj, client.Apply, &client.PatchOptions{Force: &soTrue, FieldManager: fieldManager})
		if err != nil {
			log.Error(err, "cannot apply object", "object", obj.obj)
			return ctrl.Result{}, err
		}

		*obj.statusRef, err = reference.GetReference(r.Scheme, obj.obj)
		if err != nil {
			log.Error(err, "failed to get reference", "object", obj.obj)
			return ctrl.Result{}, err
		}
	}

	apiSet.Status.Deployed = &soTrue
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *APISetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fluorescencev1alpha1.APISet{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
