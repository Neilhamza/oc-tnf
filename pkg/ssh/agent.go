package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// getAgent returns an SSH agent for authentication.
// If explicit key paths are provided, builds an in-memory keyring from only those keys.
// Otherwise, tries the running SSH agent (SSH_AUTH_SOCK), then falls back to ~/.ssh/ defaults.
func getAgent(keys []string) (agent.Agent, string, error) {
	if len(keys) > 0 {
		return newAgent(keys)
	}

	if authSock := os.Getenv("SSH_AUTH_SOCK"); authSock != "" {
		logrus.Debugf("Using SSH_AUTH_SOCK %s to connect to an existing agent", authSock)
		if conn, err := net.Dial("unix", authSock); err == nil { //nolint:gosec // SSH_AUTH_SOCK is a trusted local socket
			return agent.NewClient(conn), "agent", nil
		}
	}

	return newAgentFromDefaults()
}

func newAgent(keyPaths []string) (agent.Agent, string, error) {
	keys, err := LoadPrivateSSHKeys(keyPaths, true)
	if err != nil {
		return nil, "", err
	}

	ag := agent.NewKeyring()
	var errs []error
	for name, key := range keys {
		if err := ag.Add(agent.AddedKey{PrivateKey: key}); err != nil {
			errs = append(errs, fmt.Errorf("failed to add %s to agent: %w", name, err))
			continue
		}
		logrus.Debugf("Added %s to internal agent", name)
	}
	if agg := utilerrors.NewAggregate(errs); agg != nil {
		return nil, "", agg
	}
	return ag, "keys", nil
}

func newAgentFromDefaults() (agent.Agent, string, error) {
	keys, err := defaultPrivateSSHKeys()
	if err != nil {
		return nil, "", fmt.Errorf("no SSH keys available: provide --ssh-key, start an ssh-agent, or add keys to ~/.ssh/: %w", err)
	}
	if len(keys) == 0 {
		return nil, "", fmt.Errorf("no SSH keys available: provide --ssh-key, start an ssh-agent, or add keys to ~/.ssh/")
	}

	ag := agent.NewKeyring()
	var errs []error
	added := 0
	for name, key := range keys {
		if addErr := ag.Add(agent.AddedKey{PrivateKey: key}); addErr != nil {
			logrus.Debugf("failed to add default key %s: %v", name, addErr)
			errs = append(errs, fmt.Errorf("failed to add default key %s: %w", name, addErr))
			continue
		}
		added++
		logrus.Debugf("Added default key %s to internal agent", name)
	}
	if added == 0 {
		return nil, "", fmt.Errorf("none of the default SSH keys in ~/.ssh/ could be loaded: %w", utilerrors.NewAggregate(errs))
	}
	return ag, "keys", nil
}

// isPassphraseError checks if an error is due to a passphrase-protected key.
func isPassphraseError(err error) bool {
	var passErr *ssh.PassphraseMissingError
	return errors.As(err, &passErr)
}
