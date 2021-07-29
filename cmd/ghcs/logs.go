package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/github/ghcs/api"
	"github.com/github/ghcs/internal/codespaces"
	"github.com/spf13/cobra"
)

func NewLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs",
		Short: "Access Codespace logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			var codespaceName string
			if len(args) > 0 {
				codespaceName = args[0]
			}
			return Logs(codespaceName)
		},
	}
}

func init() {
	rootCmd.AddCommand(NewLogsCmd())
}

func Logs(codespaceName string) error {
	apiClient := api.New(os.Getenv("GITHUB_TOKEN"))
	ctx := context.Background()

	user, err := apiClient.GetUser(ctx)
	if err != nil {
		return fmt.Errorf("getting user: %v", err)
	}

	codespace, token, err := codespaces.GetOrChooseCodespace(ctx, apiClient, user, codespaceName)
	if err != nil {
		return fmt.Errorf("get or choose codespace: %v", err)
	}

	lsclient, err := codespaces.ConnectToLiveshare(ctx, apiClient, token, codespace)
	if err != nil {
		return fmt.Errorf("connecting to liveshare: %v", err)
	}

	tunnelPort, connClosed, err := codespaces.MakeSSHTunnel(ctx, lsclient, 0)
	if err != nil {
		return fmt.Errorf("make ssh tunnel: %v", err)
	}

	dst := fmt.Sprintf("%s@localhost", getSSHUser(codespace))
	stdout, err := codespaces.RunCommand(
		ctx, tunnelPort, dst, "cat /workspaces/.codespaces/.persistedshare/creation.log",
	)
	if err != nil {
		return fmt.Errorf("run command: %v", err)
	}

	done := make(chan error)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			done <- fmt.Errorf("error scanning: %v", err)
			return
		}

		if err := stdout.Close(); err != nil {
			done <- fmt.Errorf("close stdout: %v", err)
			return
		}
		done <- nil
	}()

	select {
	case err := <-connClosed:
		if err != nil {
			return fmt.Errorf("connection closed: %v", err)
		}
	case err := <-done:
		if err != nil {
			return err
		}
	}

	return nil
}
