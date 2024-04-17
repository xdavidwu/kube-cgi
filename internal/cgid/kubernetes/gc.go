package kubernetes

import (
	"container/ring"
	"context"
	"sort"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubecgiv1alpha1 "git.cs.nctu.edu.tw/aic/infra/kube-cgi/api/v1alpha1"
)

func cleanupOldGeneration(log logr.Logger, c client.Client, current *kubecgiv1alpha1.APISet) {
	// deletion may race with other policy or instance, thus ignoring not found

	var list corev1.PodList
	err := c.List(context.Background(), &list,
		client.InNamespace(current.Namespace),
		client.MatchingLabels{managedByKey: manager})
	if err != nil {
		log.Error(err, "cannot list pods")
		panic("cannot list pods")
	}

	keep := map[corev1.PodPhase]bool{}

	if current.Spec.HistoryLimit != nil {
		for _, item := range []struct {
			phase corev1.PodPhase
			spec  *kubecgiv1alpha1.HistoryLimitSpec
		}{
			{corev1.PodSucceeded, &current.Spec.HistoryLimit.Succeeded},
			{corev1.PodFailed, &current.Spec.HistoryLimit.Failed},
		} {
			if item.spec != nil && item.spec.KeepPreviousVersions != nil {
				keep[item.phase] = *item.spec.KeepPreviousVersions
			}
		}
	}

	for _, pod := range list.Items {
		generation, _ := strconv.ParseInt(pod.Labels[generationKey], 10, 0)
		if generation >= current.Generation {
			continue
		}

		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			log.Info("found non-terminated pod of previous geneation",
				"pod", pod.Name, "generation", generation, "phase", pod.Status.Phase)
			continue
		}
		if keep[pod.Status.Phase] {
			continue
		}
		log.Info("delete terminated pod of previous generation",
			"pod", pod.Name, "generation", generation)
		err = client.IgnoreNotFound(c.Delete(context.Background(), &pod))
		if err != nil {
			log.Error(err, "cannot delete pod", "pod", pod.Name)
		}
	}
}

func deleteUnlessLastN(log logr.Logger, c client.WithWatch, n int32, listOpts ...client.ListOption) {
	// deletion may race with other policy or instance, thus ignoring not found
	// last n should always be available as long as the order instances see is the same

	var list corev1.PodList
	err := c.List(context.Background(), &list, listOpts...)
	if err != nil {
		log.Error(err, "cannot list pods")
		panic("cannot list pods")
	}

	sort.Slice(list.Items, func(i, j int) bool {
		iTermAt := list.Items[i].Status.ContainerStatuses[0].State.Terminated.FinishedAt.Time
		jTermAt := list.Items[j].Status.ContainerStatuses[0].State.Terminated.FinishedAt.Time
		return iTermAt.Before(jTermAt)
	})

	l := int32(len(list.Items))
	for i := int32(0); i < l-n; i += 1 {
		err = client.IgnoreNotFound(c.Delete(context.Background(), &list.Items[i]))
		if err != nil {
			log.Error(err, "cannot delete pod", "pod", list.Items[i].Name)
		}
	}

	q := ring.New(int(n))
	for i := max(l-n, 0); i < l; i += 1 {
		q.Value = &list.Items[i]
		q = q.Next()
	}

	watcher, err := watchtools.NewRetryWatcher(
		list.ResourceVersion,
		watcherWithOpts(context.Background(), c, &list, listOpts...),
	)
	if err != nil {
		log.Error(err, "cannot watch pods")
		panic("cannot watch pods")
	}
	results := watcher.ResultChan()
	for {
		ev := <-results
		if ev.Type == watch.Added {
			if q == nil {
				pod := ev.Object.(*corev1.Pod)
				log.Info("remove pod due to maxCount", "pod", pod.Name)
				err = client.IgnoreNotFound(c.Delete(context.Background(), pod))
				if err != nil {
					log.Error(err, "cannot delete pod", "pod", pod.Name)
				}
			} else {
				if q.Value != nil {
					pod := q.Value.(*corev1.Pod)
					log.Info("remove pod due to maxCount", "pod", pod.Name)
					err = client.IgnoreNotFound(c.Delete(context.Background(), pod))
					if err != nil {
						log.Error(err, "cannot delete pod", "pod", pod.Name)
					}
				}
				q.Value = ev.Object.(*corev1.Pod)
				q = q.Next()
			}
		}
	}
}

func CollectGarbage(log logr.Logger, c client.WithWatch, apiset *kubecgiv1alpha1.APISet) {
	cleanupOldGeneration(log.WithValues("policy", "previousVersions"), c, apiset)

	lastNPolicy := map[corev1.PodPhase]int32{
		corev1.PodSucceeded: 0,
		corev1.PodFailed:    5,
	}

	if apiset.Spec.HistoryLimit != nil {
		for _, item := range []struct {
			phase corev1.PodPhase
			spec  *kubecgiv1alpha1.HistoryLimitSpec
		}{
			{corev1.PodSucceeded, &apiset.Spec.HistoryLimit.Succeeded},
			{corev1.PodFailed, &apiset.Spec.HistoryLimit.Failed},
		} {
			if item.spec != nil && item.spec.MaxCount != nil {
				lastNPolicy[item.phase] = *item.spec.MaxCount
			}
		}
	}

	gen := strconv.FormatInt(apiset.Generation, 10)
	for phase, n := range lastNPolicy {
		go deleteUnlessLastN(
			log.WithValues("for", phase, "policy", "maxCount", "maxCount", n),
			c, n,
			client.InNamespace(apiset.Name),
			client.MatchingLabels{managedByKey: manager},
			client.MatchingLabels{generationKey: gen},
			client.MatchingFields{"status.phase": string(phase)})
	}
}
