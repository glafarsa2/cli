package edit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/cli/api"
	"github.com/cli/cli/internal/config"
	"github.com/cli/cli/internal/ghinstance"
	"github.com/cli/cli/pkg/cmd/gist/shared"
	"github.com/cli/cli/pkg/cmdutil"
	"github.com/cli/cli/pkg/iostreams"
	"github.com/cli/cli/pkg/prompt"
	"github.com/cli/cli/pkg/surveyext"
	"github.com/spf13/cobra"
)

type EditOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	Config     func() (config.Config, error)

	Edit         func(string, string, string, *iostreams.IOStreams) (string, error)
	Add          func(string, string, *iostreams.IOStreams) (string, error)
	Selector     string
	EditFilename string
	AddFilename  string
}

func NewCmdEdit(f *cmdutil.Factory, runF func(*EditOptions) error) *cobra.Command {
	opts := EditOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Config:     f.Config,
		Edit: func(editorCmd, filename, defaultContent string, io *iostreams.IOStreams) (string, error) {
			return surveyext.Edit(
				editorCmd,
				"*."+filename,
				defaultContent,
				io.In, io.Out, io.ErrOut, nil)
		},
		Add: func(editorCmd, filename string, io *iostreams.IOStreams) (string, error) {
			return surveyext.Edit(
				editorCmd,
				"*."+filename,
				"",
				io.In, io.Out, io.ErrOut, nil)
		},
	}

	cmd := &cobra.Command{
		Use:   "edit {<id> | <url>}",
		Short: "Edit or add files in a gist",
		Args:  cmdutil.MinimumArgs(1, "cannot edit: gist argument required"),
		RunE: func(c *cobra.Command, args []string) error {
			opts.Selector = args[0]

			if runF != nil {
				return runF(&opts)
			}

			return editRun(&opts)
		},
	}
	cmd.Flags().StringVarP(&opts.AddFilename, "add", "a", "", "Add a file")
	cmd.Flags().StringVarP(&opts.EditFilename, "filename", "f", "", "Select a file to edit")

	return cmd
}

func editRun(opts *EditOptions) error {
	gistID := opts.Selector

	if strings.Contains(gistID, "/") {
		id, err := shared.GistIDFromURL(gistID)
		if err != nil {
			return err
		}
		gistID = id
	}

	client, err := opts.HttpClient()
	if err != nil {
		return err
	}

	apiClient := api.NewClientFromHTTP(client)

	gist, err := shared.GetGist(client, ghinstance.OverridableDefault(), gistID)
	if err != nil {
		return err
	}

	username, err := api.CurrentLoginName(apiClient, ghinstance.OverridableDefault())
	if err != nil {
		return err
	}

	if username != gist.Owner.Login {
		return fmt.Errorf("You do not own this gist.")
	}

	filesToUpdate := map[string]string{}

	addFilename := opts.AddFilename
	cs := opts.IO.ColorScheme()

	if addFilename != "" {
		//Add files to an existing gist.
		files, err := getFilesToAdd(addFilename, opts)
		if err != nil {
			return err
		}

		fmt.Printf("%v", files)

		gist.Files = files
		err = updateGist(apiClient, ghinstance.OverridableDefault(), gist)
		if err != nil {
			return err
		}

		completionMessage := filepath.Base(addFilename) + " added to gist"

		fmt.Fprintf(opts.IO.Out, "%s %s\n", cs.SuccessIconWithColor(cs.Green), completionMessage)

		return nil
	}

	for {
		filename := opts.EditFilename
		candidates := []string{}
		for filename := range gist.Files {
			candidates = append(candidates, filename)
		}

		sort.Strings(candidates)

		if filename == "" {
			if len(candidates) == 1 {
				filename = candidates[0]
			} else {
				if !opts.IO.CanPrompt() {
					return errors.New("unsure what file to edit; either specify --filename or run interactively")
				}
				err = prompt.SurveyAskOne(&survey.Select{
					Message: "Edit which file?",
					Options: candidates,
				}, &filename)

				if err != nil {
					return fmt.Errorf("could not prompt: %w", err)
				}
			}
		}

		if _, ok := gist.Files[filename]; !ok {
			return fmt.Errorf("gist has no file %q", filename)
		}

		editorCommand, err := cmdutil.DetermineEditor(opts.Config)
		if err != nil {
			return err
		}
		text, err := opts.Edit(editorCommand, filename, gist.Files[filename].Content, opts.IO)

		if err != nil {
			return err
		}

		if text != gist.Files[filename].Content {
			gistFile := gist.Files[filename]
			gistFile.Content = text // so it appears if they re-edit
			filesToUpdate[filename] = text
		}

		if !opts.IO.CanPrompt() {
			break
		}

		if len(candidates) == 1 {
			break
		}

		choice := ""

		err = prompt.SurveyAskOne(&survey.Select{
			Message: "What next?",
			Options: []string{
				"Edit another file",
				"Submit",
				"Cancel",
			},
		}, &choice)

		if err != nil {
			return fmt.Errorf("could not prompt: %w", err)
		}

		stop := false

		switch choice {
		case "Edit another file":
			continue
		case "Submit":
			stop = true
		case "Cancel":
			return cmdutil.SilentError
		}

		if stop {
			break
		}
	}

	if len(filesToUpdate) == 0 {
		return nil
	}

	err = updateGist(apiClient, ghinstance.OverridableDefault(), gist)
	if err != nil {
		return err
	}

	return nil
}

func updateGist(apiClient *api.Client, hostname string, gist *shared.Gist) error {
	body := shared.Gist{
		Description: gist.Description,
		Files:       gist.Files,
	}

	path := "gists/" + gist.ID

	requestByte, err := json.Marshal(body)
	if err != nil {
		return err
	}

	requestBody := bytes.NewReader(requestByte)

	result := shared.Gist{}

	err = apiClient.REST(hostname, "POST", path, requestBody, &result)

	if err != nil {
		return err
	}

	return nil
}

func getFilesToAdd(file string, opts *EditOptions) (map[string]*shared.GistFile, error) {
	cs := opts.IO.ColorScheme()

	fileExists, err := fileExists(file)
	if err != nil {
		return nil, fmt.Errorf("%s %s", cs.Red("!"), err)
	}

	filesToAdd := map[string]*shared.GistFile{}

	filename := filepath.Base(file)

	if fileExists {
		content, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("%s failed to read file %s: %w", cs.FailureIcon(), file, err)
		}

		if string(content) == "" {
			return nil, fmt.Errorf("%s Contents can't be empty", cs.FailureIcon())
		}

		filesToAdd[filename] = &shared.GistFile{
			Filename: filename,
			Content:  string(content),
		}
		return filesToAdd, nil
	} else {
		editorCommand, err := cmdutil.DetermineEditor(opts.Config)
		if err != nil {
			return nil, err
		}

		text, err := opts.Add(editorCommand, filename, opts.IO)
		if err != nil {
			return nil, err
		}

		if text == "" {
			return nil, fmt.Errorf("%s Contents can't be empty", cs.Red("!"))
		}

		filesToAdd[filename] = &shared.GistFile{
			Filename: filename,
			Content:  text,
		}

		return filesToAdd, nil
	}
}

func fileExists(filename string) (bool, error) {

	fi, err := os.Stat(filename)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
	}

	switch mode := fi.Mode(); {
	case mode.IsDir():
		return false, fmt.Errorf("found directory %s", filename)
	case mode.IsRegular():
		return true, nil
	}

	return false, nil
}
