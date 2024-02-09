package kubernetes

import (
	"bytes"
	"context"
	"net/http"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/remotecommand"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy"
	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/cgi"
	"git.cs.nctu.edu.tw/aic/infra/fluorescence/internal/dappy/middlewares"
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
	log := dappy.LoggerFromContext(ctx)
	must := func(err error) {
		if err != nil {
			log.Panic(err)
		}
	}

	listOptions := []client.ListOption{
		client.InNamespace(namespace),
		// k8s.io/kubernetes/pkg/registry/core/event.ToSelectableFields
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
	log := dappy.LoggerFromContext(ctx)
	must := func(err error) {
		if err != nil {
			log.Panic(err)
		}
	}
	log.Printf("requested %s", r.RequestURI)

	input := dappy.BodyFromContext(ctx)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.Namespace,
			Name:      namify(h.Spec.Path) + "-" + dappy.IdFromContext(ctx),
			Labels: map[string]string{
				managedByKey: manager,
			},
			OwnerReferences: []metav1.OwnerReference{h.OwnerReference},
		},
		Spec: *h.Spec.PodSpec.DeepCopy(),
	}
	for k, v := range cgi.VarsFromRequest(r) {
		if dappy.EnvTooLarge(k, v) {
			w.WriteHeader(http.StatusRequestHeaderFieldsTooLarge)
			return
		}
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  k,
			Value: escapeKubernetesExpansion(v),
		})
	}

	if !dappy.EnvTooLarge(dappy.BodyEnvKey, string(input)) {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  dappy.BodyEnvKey,
			Value: escapeKubernetesExpansion(string(input)),
		})
	} else {
		if !pod.Spec.Containers[0].Stdin {
			log.Printf("request body too large for env but script does not accept stdin, rejecting request")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		log.Printf("request body too large for env, relying on stdin only for request body")
	}

	err := h.Client.Create(context.Background(), pod)
	must(err)

	log.Printf("dispatched pod %s", pod.ObjectMeta.Name)
	go logEventsForPod(ctx, h.Client, h.Namespace, pod.ObjectMeta.UID)
	defer func() {
		// TODO channel a sophisticated GC on a fixed goroutine instead
		must(h.Client.Get(context.Background(), client.ObjectKeyFromObject(pod), pod))
		if pod.Status.Phase == corev1.PodSucceeded {
			must(h.Client.Delete(context.Background(), pod))
		}
	}()

	var list corev1.PodList
	watchOptions := []client.ListOption{
		client.InNamespace(h.Namespace),
		// k8s.io/kubernetes/pkg/registry/core/pod.ToSelectableFields
		client.MatchingFields{metav1.ObjectNameField: string(pod.ObjectMeta.Name)},
	}
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

	if pod.Spec.Containers[0].Stdin && pod.Status.Phase == corev1.PodRunning {
		url := h.OldClient.CoreV1().RESTClient().Post().
			Namespace(h.Namespace).Resource("pods").
			Name(pod.ObjectMeta.Name).SubResource("attach").
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

	// XXX dynamic client supports only CRUD subresources
	pods := h.OldClient.CoreV1().Pods(h.Namespace)
	reader, err := pods.GetLogs(pod.ObjectMeta.Name,
		&corev1.PodLogOptions{Follow: true}).Stream(ctx)
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
		stack = middlewares.ValidateJson(stack, schema)
	}
	middlewares.Intrument(middlewares.LogWithIdentifier(
		middlewares.DrainBody(stack)), h.Spec.Path).
		ServeHTTP(w, r)
}