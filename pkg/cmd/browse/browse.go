package browse

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cli/cli/internal/ghrepo"
	"github.com/cli/cli/pkg/cmdutil"
	"github.com/cli/cli/pkg/iostreams"
	"github.com/spf13/cobra"
)

type browser interface {
	Browse(string) error
}

type BrowseOptions struct {
	HttpClient func() (*http.Client, error)
	IO         *iostreams.IOStreams
	BaseRepo   func() (ghrepo.Interface, error)
	Browser    browser

	SelectorArg string
	FileArg     string // Used for storing the file path
	NumberArg   int    // Used for storing pull request number

	ProjectsFlag bool
	WikiFlag     bool
	SettingsFlag bool
}

type exitCode int

const (
	exitSuccess      exitCode = 0
	exitNotInRepo    exitCode = 1
	exitTooManyFlags exitCode = 2
	exitError        exitCode = 3
)

func NewCmdBrowse(f *cmdutil.Factory) *cobra.Command {

	opts := &BrowseOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Browser:    f.Browser,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Long:  "Work with GitHub in the browser", // displays when you are on the help page of this command
		Short: "Open GitHub in the browser",      // displays in the gh root help
		Use:   "browse",                          // necessary!!! This is the cmd that gets passed on the prompt
		Args:  cobra.RangeArgs(0, 1),             // make sure only one arg at most is passed

		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				opts.SelectorArg = args[0]
			}
			openInBrowser(cmd, opts) // run gets rid of the usage / runs function
		},
	}

	cmd.Flags().BoolVarP(&opts.ProjectsFlag, "projects", "p", false, "Open projects tab in browser")
	cmd.Flags().BoolVarP(&opts.WikiFlag, "wiki", "w", false, "Opens the wiki in browser")
	cmd.Flags().BoolVarP(&opts.SettingsFlag, "settings", "s", false, "Opens the settings in browse")

	return cmd
}

func openInBrowser(cmd *cobra.Command, opts *BrowseOptions) {

	baseRepo, err := opts.BaseRepo()

	if !inRepo(err) { // must be in a repo to execute
		printExit(exitNotInRepo, cmd, opts, "")
		return
	}

	if getFlagAmount(cmd) > 1 { // command can't have more than one flag
		printExit(exitTooManyFlags, cmd, opts, "")
		return
	}

	repoUrl := ghrepo.GenerateRepoURL(baseRepo, "")
	parseArgs(opts)

	if opts.SelectorArg == "" {
		if opts.ProjectsFlag {
			repoUrl += "/projects"
			printExit(exitSuccess, cmd, opts, repoUrl)
		} else if opts.SettingsFlag {
			repoUrl += "/settings"
			printExit(exitSuccess, cmd, opts, repoUrl)
		} else if opts.WikiFlag {
			repoUrl += "/wiki"
			printExit(exitSuccess, cmd, opts, repoUrl)
		} else if getFlagAmount(cmd) == 0 {
			printExit(exitSuccess, cmd, opts, repoUrl)
		}

		opts.Browser.Browse(repoUrl)
		return
	}

	printExit(exitError, cmd, opts, "")
}

func parseArgs(opts *BrowseOptions) {
	if opts.SelectorArg != "" {
		convertedArg, err := strconv.Atoi(opts.SelectorArg)
		if err != nil { //It's not a number, but a file name
			opts.FileArg = opts.SelectorArg
		} else { // It's a number, open issue or pull request
			opts.NumberArg = convertedArg
		}
	}
}

func printExit(errorCode exitCode, cmd *cobra.Command, opts *BrowseOptions, url string) {
	w := opts.IO.ErrOut
	cs := opts.IO.ColorScheme()

	switch errorCode {
	case exitSuccess:
		fmt.Fprintf(opts.IO.ErrOut, "%s Now opening %s in browser . . .\n",
			opts.IO.ColorScheme().Green("✓"),
			opts.IO.ColorScheme().Bold(url))
		break
	case exitNotInRepo:
		fmt.Fprintf(w, "%s Change directory to a repository to open in browser\nUse 'gh browse --help' for more information about browse\n",
			cs.Red("x"))
		break
	case exitTooManyFlags:
		fmt.Fprintf(w, "%s accepts 1 flag, %d flags were recieved\nUse 'gh browse --help' for more information about browse\n",
			cs.Red("x"), getFlagAmount(cmd))
		break
	case exitError:
		fmt.Fprintf(w, "%s Incorrect use of arguments and flags\nUse 'gh browse --help' for more information about browse\n",
			cs.Red("x"))
		break
	}

}

func getFlagAmount(cmd *cobra.Command) int {
	return cmd.Flags().NFlag()
}

func inRepo(err error) bool {
	return err == nil
}
