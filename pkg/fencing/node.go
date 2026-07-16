package fencing

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultNotReadyTimeout = 20 * time.Minute
	defaultReadyTimeout    = 10 * time.Minute
)

func waitNotReady(ctx context.Context, kube kubernetes.Interface, nodeName string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = defaultNotReadyTimeout
	}
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	lastLog := start
	var lastErr error
	err := wait.PollUntilContextCancel(pollCtx, pollInterval, true, func(ctx context.Context) (bool, error) {
		ready, err := isNodeReady(ctx, kube, nodeName)
		if err != nil {
			lastErr = err
			logrus.Debugf("Error checking %s readiness: %v", nodeName, err)
			return false, nil
		}
		if !ready {
			return true, nil
		}
		if time.Since(lastLog) > time.Minute {
			logrus.Debugf("Still waiting for %s to become NotReady (%s elapsed)", nodeName, time.Since(start).Round(time.Second))
			lastLog = time.Now()
		}
		return false, nil
	})
	if err != nil && lastErr != nil {
		return fmt.Errorf("%w; last error: %w", err, lastErr)
	}
	return err
}

func waitReady(ctx context.Context, kube kubernetes.Interface, nodeName string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = defaultReadyTimeout
	}
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	lastLog := start
	var lastErr error
	err := wait.PollUntilContextCancel(pollCtx, pollInterval, true, func(ctx context.Context) (bool, error) {
		ready, err := isNodeReady(ctx, kube, nodeName)
		if err != nil {
			lastErr = err
			logrus.Debugf("Error checking %s readiness: %v", nodeName, err)
			return false, nil
		}
		if ready {
			return true, nil
		}
		if time.Since(lastLog) > time.Minute {
			logrus.Debugf("Still waiting for %s to become Ready (%s elapsed)", nodeName, time.Since(start).Round(time.Second))
			lastLog = time.Now()
		}
		return false, nil
	})
	if err != nil && lastErr != nil {
		return fmt.Errorf("%w; last error: %w", err, lastErr)
	}
	return err
}

func isNodeReady(ctx context.Context, kube kubernetes.Interface, nodeName string) (bool, error) {
	node, err := kube.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue, nil
		}
	}
	return false, nil
}

func nodeInternalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}
