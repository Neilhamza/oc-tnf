package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const sshDialTimeout = 30 * time.Second

func NewClient(user, address string, keys []string, insecureHostKey bool) (*ssh.Client, error) {
	ag, agentType, err := getAgent(keys)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the SSH agent: %w", err)
	}

	hostKeyCb, err := hostKeyCallback(insecureHostKey)
	if err != nil {
		return nil, fmt.Errorf("failed to set up host key verification: %w", err)
	}

	client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(ag.Signers),
		},
		HostKeyCallback: hostKeyCb,
		Timeout:         sshDialTimeout,
	})
	if err != nil {
		if strings.Contains(err.Error(), "ssh: handshake failed: ssh: unable to authenticate") {
			if agentType == "agent" {
				return nil, fmt.Errorf("failed to use pre-existing agent, make sure the appropriate keys exist in the agent for authentication: %w", err)
			}
			return nil, fmt.Errorf("failed to use the provided keys for authentication: %w", err)
		}
		return nil, err
	}
	return client, nil
}

func RunOutput(client *ssh.Client, command string) (string, string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer sess.Close()

	var stdout, stderr strings.Builder
	sess.Stdout = &stdout
	sess.Stderr = &stderr
	err = sess.Run(command)
	return stdout.String(), stderr.String(), err
}

func hostKeyCallback(insecure bool) (ssh.HostKeyCallback, error) {
	if insecure {
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // user explicitly opted out via --insecure-skip-host-key-check
	}
	home, err := os.UserHomeDir()
	if err != nil {
		logrus.Warnf("Could not determine home directory: %v — falling back to insecure host key verification", err)
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // no home directory available
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHostsPath); err != nil {
		logrus.Warnf("No known_hosts file found at %s — falling back to insecure host key verification", knownHostsPath)
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // no known_hosts available
	}
	return knownhosts.New(knownHostsPath)
}

func defaultPrivateSSHKeys() (map[string]any, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}
	d := filepath.Join(home, ".ssh")
	paths, err := os.ReadDir(d)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", d, err)
	}

	var files []string
	for _, path := range paths {
		if path.IsDir() {
			continue
		}
		files = append(files, filepath.Join(d, path.Name()))
	}
	keys, err := LoadPrivateSSHKeys(files)
	if len(keys) > 0 {
		return keys, nil
	}
	return nil, err
}

func LoadPrivateSSHKeys(paths []string) (map[string]any, error) {
	var errs []error
	keys := make(map[string]any)
	for _, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // paths come from user-provided --ssh-key flag or ~/.ssh/
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to read %q: %w", path, err))
			continue
		}
		key, err := ssh.ParseRawPrivateKey(data)
		if err != nil {
			if isPassphraseError(err) {
				errs = append(errs, fmt.Errorf("key %q is passphrase-protected; add it to ssh-agent or provide an unencrypted key", path))
			} else {
				logrus.Debugf("skipping %q: not a valid private key", path)
			}
			continue
		}
		keys[path] = key
	}
	if err := utilerrors.NewAggregate(errs); err != nil {
		return keys, err
	}
	return keys, nil
}
