package main

import (
	"context"
	"io"
	"log"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := ctx.Value(ctxLogger).(*log.Logger)
	log.Printf("requested %s", r.RequestURI)

	name := ctx.Value(ctxId).(string)

	input := string(r.Context().Value(ctxBody).([]byte))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.namespace,
			Name:      name,
		},
		Spec: *h.spec.PodSpec.DeepCopy(),
	}
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
		Name:  "INPUT",
		Value: input,
	})

	err := h.client.Create(context.Background(), pod)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("dispatched pod %s", name)
	stop := logEventsForPod(log, h.client, h.namespace, pod.ObjectMeta.UID)
	defer close(stop)

	lastEvent, err := watchtools.Until(
		context.Background(),
		pod.ObjectMeta.ResourceVersion,
		&cache.ListWatch{
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				var list corev1.PodList
				return h.client.Watch(context.Background(), &list, &client.ListOptions{
					Namespace:     h.namespace,
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
	log.Printf("pod terminated with phase %s", pod.Status.Phase)
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		defer func() {
			go func() {
				err = h.client.Delete(context.Background(), pod)
				if err != nil {
					log.Panic(err)
				}
			}()
		}()
	case corev1.PodFailed:
		w.WriteHeader(int(pod.Status.ContainerStatuses[0].State.Terminated.ExitCode) + 399)
	}

	// XXX dynamic client supports only structured subresources
	pods := h.oldClient.CoreV1().Pods(h.namespace)
	reader, err := pods.GetLogs(name, &corev1.PodLogOptions{}).Stream(context.Background())
	if err != nil {
		log.Panic(err)
	}
	defer reader.Close()
	_, err = io.Copy(w, reader)
	if err != nil {
		log.Panic(err)
	}
}
