package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func NewClient(user, address string, keys []string) (*ssh.Client, error) {
	ag, agentType, err := getAgent(keys)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the SSH agent: %w", err)
	}

	client, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(ag.Signers),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
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
	if err := agent.RequestAgentForwarding(sess); err != nil {
		logrus.Debugf("agent forwarding unavailable: %v", err)
	}

	var stdout, stderr strings.Builder
	sess.Stdout = &stdout
	sess.Stderr = &stderr
	err = sess.Run(command)
	return stdout.String(), stderr.String(), err
}

func defaultPrivateSSHKeys() (map[string]interface{}, error) {
	d := filepath.Join(os.Getenv("HOME"), ".ssh")
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

func LoadPrivateSSHKeys(paths []string) (map[string]interface{}, error) {
	var errs []error
	keys := make(map[string]interface{})
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to read %q: %w", path, err))
			continue
		}
		key, err := ssh.ParseRawPrivateKey(data)
		if err != nil {
			logrus.Debugf("failed to parse SSH private key from %q", path)
			errs = append(errs, fmt.Errorf("failed to parse SSH private key from %q: %w", path, err))
			continue
		}
		keys[path] = key
	}
	if err := utilerrors.NewAggregate(errs); err != nil {
		return keys, err
	}
	return keys, nil
}
