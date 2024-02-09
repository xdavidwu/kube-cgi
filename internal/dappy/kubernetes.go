package dappy

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/remotecommand"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/cgi"
)

//+kubebuilder:rbac:groups=fluorescence.aic.cs.nycu.edu.tw,resources=apisets,verbs=get
//+kubebuilder:rbac:groups="",resources=pods,verbs=*
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get
//+kubebuilder:rbac:groups="",resources=pods/attach,verbs=create
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch

const (
	managedByKey = "app.kubernetes.io/managed-by"
	manager      = "dappy"
)

func watcherWithOpts(
	c client.WithWatch,
	list client.ObjectList,
	opts ...client.ListOption,
) cache.Watcher {
	return &cache.ListWatch{
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			merged := append(opts, &client.ListOptions{Raw: &options})
			return c.Watch(context.Background(), list, merged...)
		},
	}
}

func logEventsForPod(ctx context.Context, c client.WithWatch, namespace string, uid types.UID) {
	log := loggerFromContext(ctx)
	must := func(err error) {
		if err != nil {
			log.Panic(err)
		}
	}

	listOptions := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingFields{"involvedObject.uid": string(uid)},
	}

	var list corev1.EventList
	must(c.List(context.Background(), &list, listOptions...))
	for _, event := range list.Items {
		log.Println(event.Message)
	}

	watcher, err := watchtools.NewRetryWatcher(
		list.ListMeta.ResourceVersion,
		watcherWithOpts(c, &list, listOptions...),
	)
	must(err)

	results := watcher.ResultChan()
	for {
		select {
		case <-ctx.Done():
			watcher.Stop()
			return
		case watchEvent := <-results:
			if watchEvent.Type == watch.Added {
				event := watchEvent.Object.(*corev1.Event)
				log.Println(event.Message)
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
	return strings.TrimLeft(strings.Map(sanitize, strings.ToLower(i)), "-")
}

// k8s.io/kubernetes/third_party/forked/golang/expansion
func escapeKubernetesExpansion(i string) string {
	return strings.ReplaceAll(i, "$", "$$")
}

type kHandler KubernetesHandler

func (h kHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := loggerFromContext(ctx)
	must := func(err error) {
		if err != nil {
			log.Panic(err)
		}
	}
	log.Printf("requested %s", r.RequestURI)

	name := namify(h.Spec.Path) + "-" + idFromContext(ctx)

	input := bodyFromContext(ctx)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.Namespace,
			Name:      name,
			Labels: map[string]string{
				managedByKey: manager,
			},
		},
		Spec: *h.Spec.PodSpec.DeepCopy(),
	}
	for k, v := range cgi.VarsFromRequest(r) {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  k,
			Value: escapeKubernetesExpansion(v),
		})
	}
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  bodyEnvKey,
		Value: escapeKubernetesExpansion(string(input)),
	})
	must(controllerutil.SetControllerReference(h.APISet, pod, h.Client.Scheme()))

	err := h.Client.Create(context.Background(), pod)
	if err != nil && errors.IsRequestEntityTooLargeError(err) {
		if !pod.Spec.Containers[0].Stdin {
			log.Printf("pod spec too large but script does not accept stdin, rejecting request: %v", err)
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}

		log.Printf("pod spec too large, falling back to stdin only for request body: %v", err)
		pod.Spec.Containers[0].Env = pod.Spec.Containers[0].Env[:len(pod.Spec.Containers[0].Env)-1]
		err = h.Client.Create(context.Background(), pod)
	}
	must(err)

	log.Printf("dispatched pod %s", name)
	go logEventsForPod(ctx, h.Client, h.Namespace, pod.ObjectMeta.UID)

	var list corev1.PodList
	watchOptions := []client.ListOption{
		client.InNamespace(h.Namespace),
		client.MatchingFields{"metadata.uid": string(pod.ObjectMeta.UID)},
	}
	if pod.Spec.Containers[0].Stdin {
		lastEvent, err := watchtools.Until(
			ctx,
			pod.ObjectMeta.ResourceVersion,
			watcherWithOpts(h.Client, &list, watchOptions...),
			func(event watch.Event) (bool, error) {
				if event.Type == watch.Deleted {
					log.Panic("pod deleted while still waiting")
				}
				if event.Type != watch.Added && event.Type != watch.Modified {
					return false, nil
				}
				pod := event.Object.(*corev1.Pod)
				if pod.Status.Phase == corev1.PodRunning {
					return true, nil
				}
				if pod.Status.Phase == corev1.PodSucceeded ||
					pod.Status.Phase == corev1.PodFailed {
					return true, nil
				}
				return false, nil
			},
		)
		must(err)
		pod = lastEvent.Object.(*corev1.Pod)

		if pod.Status.Phase == corev1.PodRunning {
			url := h.OldClient.CoreV1().RESTClient().Post().
				Namespace(h.Namespace).Resource("pods").
				Name(name).SubResource("attach").
				VersionedParams(&corev1.PodAttachOptions{
					Stdin:  true,
					Stdout: false,
					Stderr: false,
					TTY:    false,
				}, scheme.ParameterCodec).URL()
			attach, err := remotecommand.NewSPDYExecutor(h.ClientConfig, "POST", url)
			// does not really fire request yet, nothing should happen
			must(err)
			log.Printf("streaming input to pod")
			err = attach.StreamWithContext(ctx, remotecommand.StreamOptions{
				Stdin:  bytes.NewReader(input),
				Stdout: nil,
				Stderr: nil,
				Tty:    false,
			})
			if err != nil {
				log.Printf("streaming input: %v", err)
			}
		}
	}

	if pod.Status.Phase != corev1.PodSucceeded &&
		pod.Status.Phase != corev1.PodFailed {
		lastEvent, err := watchtools.Until(
			ctx,
			pod.ObjectMeta.ResourceVersion,
			watcherWithOpts(h.Client, &list, watchOptions...),
			func(event watch.Event) (bool, error) {
				if event.Type == watch.Deleted {
					log.Panic("pod deleted while still waiting")
				}
				if event.Type != watch.Added && event.Type != watch.Modified {
					return false, nil
				}
				pod := event.Object.(*corev1.Pod)
				if pod.Status.Phase == corev1.PodSucceeded ||
					pod.Status.Phase == corev1.PodFailed {
					return true, nil
				}
				return false, nil
			},
		)
		must(err)
		pod = lastEvent.Object.(*corev1.Pod)
	}

	log.Printf("pod terminated with phase %s", pod.Status.Phase)
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		defer func() {
			go func() {
				must(h.Client.Delete(context.Background(), pod))
			}()
		}()
	}

	// XXX dynamic client supports only structured subresources
	pods := h.OldClient.CoreV1().Pods(h.Namespace)
	reader, err := pods.GetLogs(name, &corev1.PodLogOptions{}).Stream(ctx)
	must(err)
	defer reader.Close()
	redir, err := cgi.WriteResponse(w, reader)
	if redir != "" {
		log.Panic("interal redirects not implemented")
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
	}
}

func (h KubernetesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var stack http.Handler = kHandler(h)
	if h.Spec.Request != nil && h.Spec.Request.Schema != nil {
		schema := jsonschema.MustCompileString("api.schema.json", h.Spec.Request.Schema.RawJSON)
		stack = validateJson(stack, schema)
	}
	stack = intrument(logWithIdentifier(drainBody(stack)), h.Spec.Path)
	stack.ServeHTTP(w, r)
}
