package dappy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
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

func logEventsForPod(log *log.Logger, c client.WithWatch, namespace string, uid types.UID) chan struct{} {
	stop := make(chan struct{})
	go func() {
		listOptions := client.ListOptions{
			Namespace:     namespace,
			FieldSelector: fields.OneTermEqualSelector("involvedObject.uid", string(uid)),
		}
		var list corev1.EventList
		err := c.List(context.Background(), &list, &listOptions)
		if err != nil {
			log.Panic(err)
		}
		for _, event := range list.Items {
			log.Println(event.Message)
		}

		watcher, err := watchtools.NewRetryWatcher(
			list.ListMeta.ResourceVersion,
			&cache.ListWatch{
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					listOptions.Raw = &options
					return c.Watch(context.Background(), &list, &listOptions)
				},
			},
		)
		if err != nil {
			log.Panic(err)
		}

		results := watcher.ResultChan()
		for {
			select {
			case <-stop:
				watcher.Stop()
				return
			case watchEvent := <-results:
				if watchEvent.Type == watch.Added {
					event := watchEvent.Object.(*corev1.Event)
					log.Println(event.Message)
				}
			}
		}
	}()
	return stop
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

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := ctx.Value(ctxLogger).(*log.Logger)
	log.Printf("requested %s", r.RequestURI)

	name := fmt.Sprintf("%s-%s", namify(h.Spec.Path), ctx.Value(ctxId).(string))

	input := string(ctx.Value(ctxBody).([]byte))
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
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "INPUT",
		Value: input,
	})
	for k, v := range cgi.VarsFromRequest(r) {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	err := controllerutil.SetControllerReference(h.APISet, pod, h.Client.Scheme())
	if err != nil {
		log.Panic(err)
	}

	err = h.Client.Create(context.Background(), pod)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("dispatched pod %s", name)
	stop := logEventsForPod(log, h.Client, h.Namespace, pod.ObjectMeta.UID)
	defer close(stop)

	if pod.Spec.Containers[0].Stdin {
		lastEvent, err := watchtools.Until(
			ctx,
			pod.ObjectMeta.ResourceVersion,
			&cache.ListWatch{
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					var list corev1.PodList
					return h.Client.Watch(context.Background(), &list, &client.ListOptions{
						Namespace:     h.Namespace,
						FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
					})
				},
			},
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
		if err != nil {
			log.Panic(err)
		}
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
			if err != nil {
				log.Panic(err)
			}
			log.Printf("streaming input to pod")
			err = attach.StreamWithContext(ctx, remotecommand.StreamOptions{
				Stdin:  strings.NewReader(input),
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
			&cache.ListWatch{
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					var list corev1.PodList
					return h.Client.Watch(context.Background(), &list, &client.ListOptions{
						Namespace:     h.Namespace,
						FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
					})
				},
			},
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
		if err != nil {
			log.Panic(err)
		}
		pod = lastEvent.Object.(*corev1.Pod)
	}

	log.Printf("pod terminated with phase %s", pod.Status.Phase)
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		defer func() {
			go func() {
				err = h.Client.Delete(context.Background(), pod)
				if err != nil {
					log.Panic(err)
				}
			}()
		}()
	case corev1.PodFailed:
		w.WriteHeader(ExitCodeToHttpStatus(int(pod.Status.ContainerStatuses[0].State.Terminated.ExitCode)))
	}

	// XXX dynamic client supports only structured subresources
	pods := h.OldClient.CoreV1().Pods(h.Namespace)
	reader, err := pods.GetLogs(name, &corev1.PodLogOptions{}).Stream(ctx)
	if err != nil {
		log.Panic(err)
	}
	defer reader.Close()
	_, err = io.Copy(w, reader)
	if err != nil {
		log.Panic(err)
	}
}
