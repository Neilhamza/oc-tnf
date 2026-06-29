package ssh

import (
	"fmt"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/agent"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func getAgent(keys []string) (agent.Agent, string, error) {
	if authSock := os.Getenv("SSH_AUTH_SOCK"); authSock != "" {
		logrus.Debugf("Using SSH_AUTH_SOCK %s to connect to an existing agent", authSock)
		if conn, err := net.Dial("unix", authSock); err == nil { //nolint:gosec // SSH_AUTH_SOCK is a trusted env var
			return agent.NewClient(conn), "agent", nil
		}
	}

	return newAgent(keys)
}

func newAgent(keyPaths []string) (agent.Agent, string, error) {
	keys, err := loadKeys(keyPaths)
	if err != nil {
		return nil, "", err
	}

	ag := agent.NewKeyring()
	var errs []error
	for name, key := range keys {
		if err := ag.Add(agent.AddedKey{PrivateKey: key}); err != nil {
			errs = append(errs, fmt.Errorf("failed to add %s to agent: %w", name, err))
		}
		logrus.Debugf("Added %s to internal agent", name)
	}
	if agg := utilerrors.NewAggregate(errs); agg != nil {
		return nil, "", agg
	}
	return ag, "keys", nil
}

func loadKeys(paths []string) (map[string]interface{}, error) {
	keys := map[string]interface{}{}
	if len(paths) > 0 {
		pkeys, err := LoadPrivateSSHKeys(paths)
		if err != nil {
			return nil, err
		}
		for k, v := range pkeys {
			keys[k] = v
		}
	}
	dkeys, err := defaultPrivateSSHKeys()
	if err != nil && len(paths) == 0 {
		return nil, err
	}
	for k, v := range dkeys {
		keys[k] = v
	}
	return keys, nil
}
