package codespaces

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/github/ghcs/api"
)

// PostCreateStateStatus is a string value representing the different statuses a state can have.
type PostCreateStateStatus string

func (p PostCreateStateStatus) String() string {
	return strings.Title(string(p))
}

const (
	PostCreateStateRunning PostCreateStateStatus = "running"
	PostCreateStateSuccess PostCreateStateStatus = "succeeded"
	PostCreateStateFailed  PostCreateStateStatus = "failed"
)

// PostCreateState is a combination of a state and status value that is captured
// during codespace creation.
type PostCreateState struct {
	Name   string                `json:"name"`
	Status PostCreateStateStatus `json:"status"`
}

// PollPostCreateStates watches for state changes in a codespace,
// and calls the supplied poller for each batch of state changes.
// It runs until the context is cancelled or SSH tunnel is closed.
func PollPostCreateStates(ctx context.Context, log logger, apiClient *api.API, user *api.User, codespace *api.Codespace, poller func([]PostCreateState)) error {
	token, err := apiClient.GetCodespaceToken(ctx, user.Login, codespace.Name)
	if err != nil {
		return fmt.Errorf("getting codespace token: %v", err)
	}

	lsclient, err := ConnectToLiveshare(ctx, log, apiClient, user.Login, token, codespace)
	if err != nil {
		return fmt.Errorf("connect to liveshare: %v", err)
	}

	tunnelPort, connClosed, err := StartPortForwarding(ctx, lsclient, "sshd", 0)
	if err != nil {
		return fmt.Errorf("make ssh tunnel: %v", err)
	}

	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-connClosed:
			return fmt.Errorf("connection closed: %v", err)
		case <-t.C:
			states, err := getPostCreateOutput(ctx, tunnelPort, codespace)
			if err != nil {
				return fmt.Errorf("get post create output: %v", err)
			}

			poller(states)
		}
	}
}

func getPostCreateOutput(ctx context.Context, tunnelPort int, codespace *api.Codespace) ([]PostCreateState, error) {
	cmd := NewRemoteCommand(
		ctx, tunnelPort, sshDestination(codespace),
		"cat /workspaces/.codespaces/shared/postCreateOutput.json",
	)
	stdout := new(bytes.Buffer)
	cmd.Stdout = stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run command: %v", err)
	}
	var output struct {
		Steps []PostCreateState `json:"steps"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("unmarshal output: %v", err)
	}

	return output.Steps, nil
}

// TODO(josebalius): this won't be needed soon
func sshDestination(codespace *api.Codespace) string {
	user := "codespace"
	if codespace.RepositoryNWO == "github/github" {
		user = "root"
	}
	return user + "@localhost"
}
