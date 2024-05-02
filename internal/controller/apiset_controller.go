package controller

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
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

	kubecgiv1alpha1 "github.com/xdavidwu/kube-cgi/api/v1alpha1"
	"github.com/xdavidwu/kube-cgi/internal"
)

// APISetReconciler reconciles a APISet object
type APISetReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	KcgidImage string
	PullSecret *corev1.Secret
}

//+kubebuilder:rbac:groups=kube-cgi.aic.cs.nycu.edu.tw,resources=apisets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kube-cgi.aic.cs.nycu.edu.tw,resources=apisets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kube-cgi.aic.cs.nycu.edu.tw,resources=apisets/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=create;patch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=services,verbs=create;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;patch
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=create;patch

// rbac in internal/cgid is also set on manager to be able to bind

const (
	fieldManager     = "kube-cgi"
	managedByKey     = "app.kubernetes.io/managed-by"
	managedByManager = fieldManager
	metricsPortName  = "metrics"
	httpPortName     = "http"
)

var (
	apiSetKey = kubecgiv1alpha1.GroupVersion.Group + "/apiset"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.0/pkg/reconcile
func (r *APISetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var apiSet kubecgiv1alpha1.APISet
	err := r.Get(ctx, req.NamespacedName, &apiSet)
	if err != nil {
		log.Error(err, "cannot get requested APISet")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	serviceAccount := corev1.ServiceAccount{}

	roleBinding := rbacv1.RoleBinding{
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "kube-cgi-kcgid",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      req.Name,
				Namespace: req.Namespace,
			},
		},
	}

	args := []string{}
	if apiSet.Spec.Kcgid != nil {
		args = apiSet.Spec.Args
	}

	apiSetLabelValue := req.Namespace + "." + req.Name
	deployment := appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{apiSetKey: apiSetLabelValue},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{apiSetKey: apiSetLabelValue},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image: r.KcgidImage,
							Name:  "kcgid",
							Args:  args,
							Env: []corev1.EnvVar{
								{
									Name:  internal.KcgidEnvAPISetNamespace,
									Value: req.Namespace,
								},
								{
									Name:  internal.KcgidEnvAPISetName,
									Value: req.Name,
								},
								{
									Name:  internal.KcgidEnvAPISetResourceVersion,
									Value: apiSet.ObjectMeta.ResourceVersion,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(internal.KcgidPort),
									},
								},
							},
						},
					},
					ServiceAccountName: req.Name,
				},
			},
		},
	}
	if apiSet.Spec.Kcgid != nil && *apiSet.Spec.Kcgid.Replicas != 0 {
		deployment.Spec.Replicas = apiSet.Spec.Kcgid.Replicas
	}

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{apiSetKey: apiSetLabelValue},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       httpPortName,
					Port:       80,
					TargetPort: intstr.FromInt(internal.KcgidPort),
				},
				{
					Name: metricsPortName,
					Port: internal.KcgidMetricsPort,
				},
			},
			Selector: map[string]string{apiSetKey: apiSetLabelValue},
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

	// XXX
	if r.PullSecret != nil {
		secret := r.PullSecret.DeepCopy()
		// reset that there is nothing (especially not managedFields) other than spec-like
		secret.ObjectMeta = metav1.ObjectMeta{}
		deployment.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: req.Name,
			},
		}

		resources = append(resources, resource{secret, &apiSet.Status.ImagePullSecret})
	}

	if apiSet.Spec.Kcgid != nil && apiSet.Spec.Kcgid.ServiceMonitor {
		serviceMonitor := monitoringv1.ServiceMonitor{
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{apiSetKey: apiSetLabelValue},
				},
				Endpoints: []monitoringv1.Endpoint{
					{Port: metricsPortName},
				},
			},
		}

		resources = append(resources, resource{&serviceMonitor, &apiSet.Status.ServiceMonitor})
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

	for _, obj := range resources {
		gvk, err := r.GroupVersionKindFor(obj.obj)
		if err != nil {
			log.Error(err, "cannot get gvk", "object", obj.obj)
			return ctrl.Result{}, err
		}
		obj.obj.GetObjectKind().SetGroupVersionKind(gvk)

		obj.obj.SetNamespace(req.Namespace)
		obj.obj.SetName(req.Name)

		err = ctrl.SetControllerReference(&apiSet, obj.obj, r.Scheme)
		if err != nil {
			log.Error(err, "cannot set owner", "object", obj.obj)
			return ctrl.Result{}, err
		}

		labels := obj.obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[managedByKey] = managedByManager
		obj.obj.SetLabels(labels)

		err = r.Patch(ctx, obj.obj, client.Apply, client.ForceOwnership, client.FieldOwner(fieldManager))
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

	soTrue := true
	apiSet.Status.Deployed = &soTrue
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *APISetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// TODO also watch servicemonitor if supported
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubecgiv1alpha1.APISet{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
