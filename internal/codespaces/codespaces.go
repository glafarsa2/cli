package codespaces

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/github/ghcs/api"
	"github.com/github/go-liveshare"
)

var (
	ErrNoCodespaces = errors.New("You have no Codespaces.")
)

func ChooseCodespace(ctx context.Context, apiClient *api.API, user *api.User) (*api.Codespace, error) {
	codespaces, err := apiClient.ListCodespaces(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("error getting Codespaces: %v", err)
	}

	if len(codespaces) == 0 {
		return nil, ErrNoCodespaces
	}

	sort.Slice(codespaces, func(i, j int) bool {
		return codespaces[i].CreatedAt > codespaces[j].CreatedAt
	})

	codespacesByName := make(map[string]*api.Codespace)
	codespacesNames := make([]string, 0, len(codespaces))
	for _, codespace := range codespaces {
		codespacesByName[codespace.Name] = codespace
		codespacesNames = append(codespacesNames, codespace.Name)
	}

	sshSurvey := []*survey.Question{
		{
			Name: "codespace",
			Prompt: &survey.Select{
				Message: "Choose Codespace:",
				Options: codespacesNames,
				Default: codespacesNames[0],
			},
			Validate: survey.Required,
		},
	}

	answers := struct {
		Codespace string
	}{}
	if err := survey.Ask(sshSurvey, &answers); err != nil {
		return nil, fmt.Errorf("error getting answers: %v", err)
	}

	codespace := codespacesByName[answers.Codespace]
	return codespace, nil
}

type logger interface {
	Print(v ...interface{}) (int, error)
	Println(v ...interface{}) (int, error)
}

func connectionReady(codespace *api.Codespace) bool {
	return codespace.Environment.Connection.SessionID != "" &&
		codespace.Environment.Connection.SessionToken != "" &&
		codespace.Environment.Connection.RelayEndpoint != "" &&
		codespace.Environment.Connection.RelaySAS != "" &&
		codespace.Environment.State == api.CodespaceEnvironmentStateAvailable
}

func ConnectToLiveshare(ctx context.Context, log logger, apiClient *api.API, userLogin, token string, codespace *api.Codespace) (*liveshare.Session, error) {
	var startedCodespace bool
	if codespace.Environment.State != api.CodespaceEnvironmentStateAvailable {
		startedCodespace = true
		log.Print("Starting your Codespace...")
		if err := apiClient.StartCodespace(ctx, token, codespace); err != nil {
			return nil, fmt.Errorf("error starting Codespace: %v", err)
		}
	}

	for retries := 0; !connectionReady(codespace); retries++ {
		if retries > 1 {
			if retries%2 == 0 {
				log.Print(".")
			}

			time.Sleep(1 * time.Second)
		}

		if retries == 30 {
			return nil, errors.New("timed out while waiting for the Codespace to start")
		}

		var err error
		codespace, err = apiClient.GetCodespace(ctx, token, userLogin, codespace.Name)
		if err != nil {
			return nil, fmt.Errorf("error getting Codespace: %v", err)
		}
	}

	if startedCodespace {
		fmt.Print("\n")
	}

	log.Println("Connecting to your Codespace...")

	lsclient, err := liveshare.NewClient(
		liveshare.WithConnection(liveshare.Connection{
			SessionID:     codespace.Environment.Connection.SessionID,
			SessionToken:  codespace.Environment.Connection.SessionToken,
			RelaySAS:      codespace.Environment.Connection.RelaySAS,
			RelayEndpoint: codespace.Environment.Connection.RelayEndpoint,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating Live Share client: %v", err)
	}

	return lsclient.JoinWorkspace(ctx)
}

func GetOrChooseCodespace(ctx context.Context, apiClient *api.API, user *api.User, codespaceName string) (codespace *api.Codespace, token string, err error) {
	if codespaceName == "" {
		codespace, err = ChooseCodespace(ctx, apiClient, user)
		if err != nil {
			if err == ErrNoCodespaces {
				return nil, "", err
			}
			return nil, "", fmt.Errorf("choosing Codespace: %v", err)
		}
		codespaceName = codespace.Name

		token, err = apiClient.GetCodespaceToken(ctx, user.Login, codespaceName)
		if err != nil {
			return nil, "", fmt.Errorf("getting Codespace token: %v", err)
		}
	} else {
		token, err = apiClient.GetCodespaceToken(ctx, user.Login, codespaceName)
		if err != nil {
			return nil, "", fmt.Errorf("getting Codespace token for given codespace: %v", err)
		}

		codespace, err = apiClient.GetCodespace(ctx, token, user.Login, codespaceName)
		if err != nil {
			return nil, "", fmt.Errorf("getting full Codespace details: %v", err)
		}
	}

	return codespace, token, nil
}
