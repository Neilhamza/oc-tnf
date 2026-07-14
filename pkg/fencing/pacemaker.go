package fencing

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	pcmkTimeout  = 10 * time.Minute
	fenceTimeout = 600 // seconds, passed to timeout(1)
)

var (
	stonithEnabledRe = regexp.MustCompile(`(?i)stonith-enabled\s*[:=]\s*true`)
	daemonActiveRe   = regexp.MustCompile(`(?i)\bactive\b.*(running|enabled)`)
)

func checkStonith(client *gossh.Client) error {
	out, err := sshRun(client, "pcs stonith config || pcs stonith status || pcs stonith show")
	if err != nil && strings.TrimSpace(out) == "" {
		return fmt.Errorf("failed to query STONITH configuration (check that pcs is installed and SSH user has sudo access): %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return fmt.Errorf("no STONITH devices configured")
	}

	prop, err := sshRun(client, "pcs property config stonith-enabled || pcs property list stonith-enabled || pcs property show --all stonith-enabled")
	if err != nil {
		return fmt.Errorf("could not read stonith-enabled property: %w", err)
	}
	if !parseStonithEnabled(prop) {
		return fmt.Errorf("stonith-enabled is not set to true")
	}
	logrus.Info("STONITH is configured and enabled")
	return nil
}

func checkPacemakerOnline(client *gossh.Client, nodes []NodeInfo) error {
	out, err := sshRun(client, "pcs status nodes || crm_mon -1")
	if err != nil {
		return fmt.Errorf("checking pacemaker status: %w", err)
	}

	online := parsePacemakerOnline(out)
	for _, n := range nodes {
		if !nodeInOnlineList(n, online) {
			return fmt.Errorf("node %s is not online in Pacemaker", n.Name)
		}
	}
	logrus.Info("Both nodes are online in Pacemaker")
	return nil
}

func checkDaemons(client *gossh.Client) error {
	out, err := sshRun(client, "pcs status --full || pcs status")
	if err != nil {
		return fmt.Errorf("checking daemon status: %w", err)
	}

	missing, found := parseDaemonStatus(out)
	if len(missing) > 0 {
		return fmt.Errorf("daemons not active/running: %s", strings.Join(missing, ", "))
	}
	if found {
		logrus.Info("All daemons (corosync, pacemaker, pcsd) are active")
	}
	return nil
}

func fenceNode(client *gossh.Client, pcmkName string) error {
	cmd := fmt.Sprintf("timeout %d pcs stonith fence %s", fenceTimeout, pcmkName)
	_, err := sshRun(client, cmd)
	if err != nil {
		var exitErr *gossh.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitStatus() == 124 {
			return fmt.Errorf("fencing %s timed out after %ds — check BMC connectivity and fence agent config: %w",
				pcmkName, fenceTimeout, err)
		}
	}
	return err
}

func pollPacemakerOnline(ctx context.Context, client *gossh.Client, nodes []NodeInfo) error {
	pollCtx, cancel := context.WithTimeout(ctx, pcmkTimeout)
	defer cancel()
	var lastErr error
	err := wait.PollUntilContextCancel(pollCtx, pollInterval, true, func(ctx context.Context) (bool, error) {
		out, err := sshRun(client, "pcs status nodes || crm_mon -1")
		if err != nil {
			lastErr = err
			logrus.Debugf("Error checking pacemaker status: %v", err)
			return false, nil
		}
		online := parsePacemakerOnline(out)
		for _, n := range nodes {
			if !nodeInOnlineList(n, online) {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil && lastErr != nil {
		return fmt.Errorf("%w; last error: %w", err, lastErr)
	}
	return err
}

func resolvePacemakerNames(client *gossh.Client, nodes []NodeInfo) error {
	out, err := sshRun(client, "pcs status nodes || crm_mon -1")
	if err != nil {
		return fmt.Errorf("resolving pacemaker node names: %w", err)
	}

	online := parsePacemakerOnline(out)
	if len(online) < 2 {
		return fmt.Errorf("expected at least 2 pacemaker nodes online, found %d", len(online))
	}

	for i := range nodes {
		short := strings.SplitN(nodes[i].Name, ".", 2)[0]
		for _, pname := range online {
			pshort := strings.SplitN(pname, ".", 2)[0]
			if pname == nodes[i].Name || pshort == short {
				nodes[i].PacemakerName = pname
				break
			}
		}
		if nodes[i].PacemakerName == "" {
			return fmt.Errorf("could not resolve pacemaker name for node %s", nodes[i].Name)
		}
	}

	logrus.Infof("Pacemaker names: %s → %s, %s → %s",
		nodes[0].Name, nodes[0].PacemakerName, nodes[1].Name, nodes[1].PacemakerName)
	return nil
}

func parseStonithEnabled(output string) bool {
	return stonithEnabledRe.MatchString(output)
}

func parsePacemakerOnline(output string) []string {
	var names []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		// crm_mon -1 on Pacemaker 2.x emits "* Online: [ node1 node2 ]"
		trimmed = strings.TrimPrefix(trimmed, "* ")
		if !strings.HasPrefix(trimmed, "Online:") {
			continue
		}
		rest := strings.TrimPrefix(trimmed, "Online:")
		rest = strings.NewReplacer("[", "", "]", "").Replace(rest)
		for _, name := range strings.Fields(rest) {
			if name != "" {
				names = append(names, name)
			}
		}
		break
	}
	return names
}

func nodeInOnlineList(node NodeInfo, online []string) bool {
	short := strings.SplitN(node.Name, ".", 2)[0]
	pcmkShort := strings.SplitN(node.PacemakerName, ".", 2)[0]
	for _, name := range online {
		nameShort := strings.SplitN(name, ".", 2)[0]
		if name == node.Name || name == node.PacemakerName ||
			nameShort == short || nameShort == pcmkShort {
			return true
		}
	}
	return false
}

func parseDaemonStatus(output string) (missing []string, found bool) {
	inSection := false
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Daemon Status:") {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		lower := strings.ToLower(strings.TrimSpace(line))
		for _, svc := range []string{"corosync", "pacemaker", "pcsd"} {
			if strings.HasPrefix(lower, svc+":") && !daemonActiveRe.MatchString(line) {
				missing = append(missing, svc)
			}
		}
	}
	if !inSection {
		logrus.Warn("Could not find Daemon Status section in pcs output — skipping daemon check")
	}
	return missing, inSection
}
