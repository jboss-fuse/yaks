package kubernetes

import (
	"context"
	"errors"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

func WaitForPodReady(ctx context.Context, client ctrl.Client, ns string, labels map[string]string) (string, error) {
	for {
		pod, err := getReadyPod(ctx, client, ns, labels)
		if err != nil {
			log.Print("error looking up target pod: ", err)
		}
		if err == nil && pod != "" {
			return pod, nil
		}

		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return "", errors.New("unable to find target pod")
		}
	}
}

func getReadyPod(ctx context.Context, client ctrl.Client, ns string, labels map[string]string) (string, error) {
	podList := corev1.PodList{}
	if err := client.List(ctx, &podList, ctrl.InNamespace(ns), ctrl.MatchingLabels(labels)); err != nil {
		return "", err
	}
	for _, pod := range podList.Items {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.ContainersReady && condition.Status == corev1.ConditionTrue {
				return pod.Name, nil
			}
		}
	}
	return "", nil
}
