package kubernetes

import (
	"context"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluorescencev1alpha1 "git.cs.nctu.edu.tw/aic/infra/fluorescence/api/v1alpha1"
)

func cleanupOldGeneration(log logr.Logger, c client.Client, current *fluorescencev1alpha1.APISet) {
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
			spec  *fluorescencev1alpha1.HistoryLimitSpec
		}{
			{corev1.PodSucceeded, &current.Spec.Succeeded},
			{corev1.PodFailed, &current.Spec.Failed},
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
		err = c.Delete(context.Background(), &pod)
		if err != nil {
			log.Error(err, "cannot delete pod", "pod", pod.Name)
		}
	}
}

func CollectGarbage(log logr.Logger, c client.Client, apiset *fluorescencev1alpha1.APISet) {
	cleanupOldGeneration(log, c, apiset)
}
