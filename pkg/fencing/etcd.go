package fencing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/wait"
)

const etcdTimeout = 10 * time.Minute

func checkEtcdHealth(client *gossh.Client, nodes []NodeInfo) error {
	endpoints := formatEtcdURL(nodes[0].IP) + "," + formatEtcdURL(nodes[1].IP)

	healthCmd := fmt.Sprintf("podman exec etcd sh -lc 'ETCDCTL_API=3 etcdctl -w json endpoint health --endpoints=%s'", endpoints)
	out, err := sshRun(client, healthCmd)
	if err != nil {
		return fmt.Errorf("etcd endpoint health check failed: %w", err)
	}
	if err := parseEtcdHealth(out); err != nil {
		return err
	}

	memberCmd := "podman exec etcd sh -lc 'ETCDCTL_API=3 etcdctl -w json member list'"
	out, err = sshRun(client, memberCmd)
	if err != nil {
		return fmt.Errorf("etcd member list failed: %w", err)
	}
	if err := parseEtcdMembers(out, nodes[0].IP, nodes[1].IP); err != nil {
		return err
	}

	logrus.Info("etcd has 2 healthy voter members")
	return nil
}

func pollEtcdHealth(ctx context.Context, client *gossh.Client, nodes []NodeInfo) error {
	endpoints := formatEtcdURL(nodes[0].IP) + "," + formatEtcdURL(nodes[1].IP)
	pollCtx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	var lastErr error
	err := wait.PollUntilContextCancel(pollCtx, pollInterval, true, func(ctx context.Context) (bool, error) {
		healthCmd := fmt.Sprintf("podman exec etcd sh -lc 'ETCDCTL_API=3 etcdctl -w json endpoint health --endpoints=%s'", endpoints)
		out, err := sshRun(client, healthCmd)
		if err != nil {
			lastErr = err
			logrus.Debugf("Error checking etcd health: %v", err)
			return false, nil
		}
		if parseErr := parseEtcdHealth(out); parseErr != nil {
			lastErr = parseErr
			return false, nil
		}

		memberCmd := "podman exec etcd sh -lc 'ETCDCTL_API=3 etcdctl -w json member list'"
		out, err = sshRun(client, memberCmd)
		if err != nil {
			lastErr = err
			logrus.Debugf("Error checking etcd members: %v", err)
			return false, nil
		}
		if parseErr := parseEtcdMembers(out, nodes[0].IP, nodes[1].IP); parseErr != nil {
			lastErr = parseErr
			return false, nil
		}
		return true, nil
	})
	if err != nil && lastErr != nil {
		return fmt.Errorf("%w; last error: %w", err, lastErr)
	}
	return err
}

type etcdHealthEntry struct {
	Health bool   `json:"health"`
	Error  string `json:"error,omitempty"`
}

func parseEtcdHealth(output string) error {
	var entries []etcdHealthEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		return fmt.Errorf("failed to parse etcd health output: %w", err)
	}
	if len(entries) < 2 {
		return fmt.Errorf("expected health for 2 endpoints, got %d", len(entries))
	}
	for _, e := range entries {
		if !e.Health {
			return fmt.Errorf("unhealthy etcd endpoint: %s", e.Error)
		}
	}
	return nil
}

type etcdMemberList struct {
	Members []etcdMember `json:"members"`
}

type etcdMember struct {
	IsLearner  bool     `json:"isLearner"`
	ClientURLs []string `json:"clientURLs"`
}

func parseEtcdMembers(output, ipA, ipB string) error {
	var list etcdMemberList
	if err := json.Unmarshal([]byte(output), &list); err != nil {
		return fmt.Errorf("failed to parse etcd member list: %w", err)
	}

	foundA, foundB := false, false
	voters := 0
	for _, m := range list.Members {
		if m.IsLearner {
			continue
		}
		voters++
		for _, u := range m.ClientURLs {
			parsed, err := url.Parse(u)
			if err != nil {
				continue
			}
			host := parsed.Hostname()
			if host == ipA {
				foundA = true
			}
			if host == ipB {
				foundB = true
			}
		}
	}
	if voters != 2 || !foundA || !foundB {
		return fmt.Errorf("etcd does not have exactly 2 voting members (voters=%d, foundA=%v, foundB=%v)", voters, foundA, foundB)
	}
	return nil
}

func formatEtcdURL(ip string) string {
	if strings.Contains(ip, ":") {
		return fmt.Sprintf("https://[%s]:2379", ip)
	}
	return fmt.Sprintf("https://%s:2379", ip)
}
