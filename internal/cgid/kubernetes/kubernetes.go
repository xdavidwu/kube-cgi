package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/remotecommand"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/xdavidwu/kube-cgi/internal/cgid"
	"github.com/xdavidwu/kube-cgi/internal/cgid/cgi"
	"github.com/xdavidwu/kube-cgi/internal/cgid/middlewares"
)

// TODO derive config at controller to avoid these
//+kubebuilder:rbac:groups=kube-cgi.aic.cs.nycu.edu.tw,resources=apisets,verbs=get
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get

//+kubebuilder:rbac:groups="",resources=pods,verbs=*
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get
//+kubebuilder:rbac:groups="",resources=pods/attach,verbs=create
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch

const (
	managedByKey = "app.kubernetes.io/managed-by"
	manager      = "cgid"
)

func watcherWithOpts(
	ctx context.Context,
	c client.WithWatch,
	list client.ObjectList,
	opts ...client.ListOption,
) cache.Watcher {
	return &cache.ListWatch{
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			merged := append(opts, &client.ListOptions{Raw: &options})
			return c.Watch(ctx, list, merged...)
		},
	}
}

func logEventsForPod(ctx context.Context, c client.WithWatch, namespace string, uid types.UID) {
	log := logr.FromContextOrDiscard(ctx)
	must := func(err error, op string) {
		if err != nil {
			log.Error(err, "cannot "+op)
			panic(err)
		}
	}

	log = log.WithValues("source", "events")

	listOptions := []client.ListOption{
		client.InNamespace(namespace),
		// k8s.io/kubernetes/pkg/registry/core/event.ToSelectableFields
		client.MatchingFields{"involvedObject.uid": string(uid)},
	}

	var list corev1.EventList
	must(c.List(ctx, &list, listOptions...), "list events")
	for _, event := range list.Items {
		log.Info(event.Message)
	}

	watcher, err := watchtools.NewRetryWatcher(
		list.ListMeta.ResourceVersion,
		watcherWithOpts(ctx, c, &list, listOptions...),
	)
	must(err, "watch events")

	results := watcher.ResultChan()
	for {
		select {
		case <-ctx.Done():
			watcher.Stop()
			return
		case watchEvent := <-results:
			if watchEvent.Type == watch.Added {
				event := watchEvent.Object.(*corev1.Event)
				log.Info(event.Message)
			}
		}
	}
}

func sanitize(i rune) rune {
	if (i >= 'a' && i <= 'z') || (i >= '0' && i <= '9') {
		return i
	}
	return '-'
}

func namify(i string) string {
	return strings.Trim(strings.Map(sanitize, strings.ToLower(i)), "-")
}

// k8s.io/kubernetes/third_party/forked/golang/expansion
func escapeKubernetesExpansion(i string) string {
	return strings.ReplaceAll(i, "$", "$$")
}

func containerStarted(pod *corev1.Pod) bool {
	if len(pod.Status.ContainerStatuses) > 0 {
		c := pod.Status.ContainerStatuses[0]
		return c.Started != nil && *c.Started
	}
	return false
}

type kHandler KubernetesHandler

func (h kHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logr.FromContextOrDiscard(ctx)
	must := func(err error, op string) {
		if err != nil {
			log.Error(err, "cannot "+op)
			panic(err)
		}
	}

	path := namify(h.Spec.Path)
	input := cgid.BodyFromContext(ctx)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.Namespace,
			Name:      path + "-" + cgid.IdFromContext(ctx),
			Labels: map[string]string{
				managedByKey:  manager,
				generationKey: strconv.FormatInt(h.Generation, 10),
				pathKey:       path,
			},
			OwnerReferences: []metav1.OwnerReference{h.OwnerReference},
		},
	}
	h.Spec.PodSpec.DeepCopyInto(&pod.Spec)
	container := &pod.Spec.Containers[0]

	for k, v := range cgi.VarsFromRequest(r) {
		if cgid.EnvTooLarge(k, v) {
			cgid.WriteError(w, http.StatusRequestHeaderFieldsTooLarge, "")
			return
		}
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  k,
			Value: escapeKubernetesExpansion(v),
		})
	}

	if input != nil {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  cgid.BodyEnvKey,
			Value: escapeKubernetesExpansion(string(input)),
		})
	} else {
		if !container.Stdin {
			log.Info("request body not drained for env but script does not accept stdin, rejecting request")
			cgid.WriteError(w, http.StatusRequestEntityTooLarge, "")
			return
		}
		log.Info("request body not drained for env, relying on stdin only for request body")
	}

	err := h.Client.Create(context.Background(), pod)
	must(err, "create pod")
	defer func() {
		go func() {
			patch := corev1ac.Pod(pod.Name, pod.Namespace).
				WithLabels(map[string]string{gcKey: "true"})
			u := unstructured.Unstructured{}
			u.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(patch)
			if err != nil {
				log.Error(err, "cannot prepare patch")
				return
			}
			err = h.Client.Patch(context.Background(), &u, client.Apply, client.ForceOwnership, client.FieldOwner(manager))
			if err != nil {
				log.Error(err, "cannot apply patch")
				return
			}
		}()
	}()

	log.Info("dispatched pod", "name", pod.ObjectMeta.Name)
	go logEventsForPod(ctx, h.Client, h.Namespace, pod.ObjectMeta.UID)

	var list corev1.PodList
	watchOptions := []client.ListOption{
		client.InNamespace(h.Namespace),
		// k8s.io/kubernetes/pkg/registry/core/pod.ToSelectableFields
		client.MatchingFields{metav1.ObjectNameField: string(pod.ObjectMeta.Name)},
	}
	lastEvent, err := watchtools.Until(
		ctx,
		pod.ObjectMeta.ResourceVersion,
		watcherWithOpts(ctx, h.Client, &list, watchOptions...),
		func(event watch.Event) (bool, error) {
			if event.Type == watch.Deleted {
				return false, fmt.Errorf("pod deleted while still waiting")
			}
			if event.Type != watch.Added && event.Type != watch.Modified {
				return false, nil
			}
			pod := event.Object.(*corev1.Pod)
			if pod.Status.Phase == corev1.PodSucceeded ||
				pod.Status.Phase == corev1.PodFailed {
				return true, nil
			}
			return containerStarted(pod), nil
		},
	)
	must(err, "watch pod")
	pod = lastEvent.Object.(*corev1.Pod)

	if container.Stdin && containerStarted(pod) {
		url := h.OldClient.CoreV1().RESTClient().Post().
			Namespace(h.Namespace).Resource("pods").
			Name(pod.ObjectMeta.Name).SubResource("attach").
			VersionedParams(&corev1.PodAttachOptions{
				Container: container.Name,
				Stdin:     true,
				Stdout:    false,
				Stderr:    false,
				TTY:       false,
			}, scheme.ParameterCodec).URL()
		attach, err := remotecommand.NewSPDYExecutor(h.ClientConfig, "POST", url)
		// does not really fire request yet, nothing should happen
		must(err, "attach pod")

		var reader io.Reader
		if input != nil {
			reader = bytes.NewReader(input)
		} else {
			reader = r.Body
		}

		go func() {
			log.Info("streaming input to pod")
			err := attach.StreamWithContext(ctx, remotecommand.StreamOptions{
				Stdin:  reader,
				Stdout: nil,
				Stderr: nil,
				Tty:    false,
			})
			if err != nil {
				log.Error(err, "streaming input")
			} else {
				log.Info("request body fully streamed")
			}
		}()
	}
	log.Info("ready for streaming response")

	// XXX dynamic client supports only CRUD subresources
	pods := h.OldClient.CoreV1().Pods(h.Namespace)
	reader, err := pods.GetLogs(pod.ObjectMeta.Name, &corev1.PodLogOptions{
		Container: container.Name,
		Follow:    true,
	}).Stream(ctx)
	must(err, "get pod logs")
	defer reader.Close()
	redir, err := cgi.WriteResponse(w, reader)
	if redir != "" {
		log.Info("internal redirects not implemented")
		panic("not implemented")
	}
	if err != nil {
		cgid.WriteError(w, http.StatusInternalServerError, "")
		log.Error(err, "cannot proxy cgi response")
	} else {
		log.Info("response streamed")
	}
}

// TODO do init stuff elsewhere
func (h KubernetesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var stack http.Handler = kHandler(h)

	if h.Spec.Request != nil {
		rSpec := h.Spec.Request

		if rSpec.Schema != nil {
			schema := jsonschema.MustCompileString("api.schema.json", rSpec.Schema.RawJSON)
			stack = middlewares.ValidateJson(stack, schema)
		}

		if rSpec.Authentication != nil &&
			rSpec.Authentication.PreShared != nil &&
			rSpec.Authentication.PreShared.SecretKeyRef != nil {
			log := logr.FromContextOrDiscard(r.Context())
			ref := rSpec.Authentication.PreShared.SecretKeyRef

			var secret corev1.Secret
			err := h.Client.Get(
				r.Context(),
				client.ObjectKey{Namespace: h.Namespace, Name: ref.Name},
				&secret,
			)
			if err != nil {
				log.Error(err, "cannot get secret", "namespace", h.Namespace, "name", ref.Name)
				panic(err)
			}

			v, ok := secret.Data[ref.Key]
			if !ok {
				err = fmt.Errorf("referred key not found in secret")
				log.Error(err, "cannot get pre-shared token", "namespace", h.Namespace, "name", ref.Name, "key", ref.Key)
				panic(err)
			}
			stack = middlewares.AuthnWithPreShared(stack, string(v))
		}
	}

	middlewares.Instrument(middlewares.LogWithIdentifier(
		middlewares.DrainBody(stack)), h.Spec.Path).
		ServeHTTP(w, r)
}
