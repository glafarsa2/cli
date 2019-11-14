package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/github/gh-cli/api"
	"github.com/github/gh-cli/utils"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

func init() {
	RootCmd.AddCommand(issueCmd)
	issueCmd.AddCommand(
		&cobra.Command{
			Use:   "status",
			Short: "Show status of relevant issues",
			RunE:  issueList,
		},
		&cobra.Command{
			Use:   "view <issue-number>",
			Args:  cobra.MinimumNArgs(1),
			Short: "View an issue in the browser",
			RunE:  issueView,
		},
	)
	issueCmd.AddCommand(issueCreateCmd)
	issueCreateCmd.Flags().StringArrayP("message", "m", nil, "set title and body")
	issueCreateCmd.Flags().BoolP("web", "w", false, "open the web browser to create an issue")
}

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Work with GitHub issues",
	Long:  `Helps you work with issues.`,
}
var issueCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new issue",
	RunE:  issueCreate,
}

func issueList(cmd *cobra.Command, args []string) error {
	ctx := contextForCommand(cmd)
	apiClient, err := apiClientForContext(ctx)
	if err != nil {
		return err
	}

	baseRepo, err := ctx.BaseRepo()
	if err != nil {
		return err
	}

	currentUser, err := ctx.AuthLogin()
	if err != nil {
		return err
	}

	issuePayload, err := api.Issues(apiClient, baseRepo, currentUser)
	if err != nil {
		return err
	}

	printHeader("Issues assigned to you")
	if issuePayload.Assigned != nil {
		printIssues(issuePayload.Assigned...)
	} else {
		message := fmt.Sprintf("  There are no issues assgined to you")
		printMessage(message)
	}
	fmt.Println()

	printHeader("Issues mentioning you")
	if len(issuePayload.Mentioned) > 0 {
		printIssues(issuePayload.Mentioned...)
	} else {
		printMessage("  There are no issues mentioning you")
	}
	fmt.Println()

	printHeader("Recent issues")
	if len(issuePayload.Recent) > 0 {
		printIssues(issuePayload.Recent...)
	} else {
		printMessage("  There are no recent issues")
	}
	fmt.Println()

	return nil
}

func issueView(cmd *cobra.Command, args []string) error {
	ctx := contextForCommand(cmd)

	baseRepo, err := ctx.BaseRepo()
	if err != nil {
		return err
	}

	var openURL string
	if number, err := strconv.Atoi(args[0]); err == nil {
		// TODO: move URL generation into GitHubRepository
		openURL = fmt.Sprintf("https://github.com/%s/%s/issues/%d", baseRepo.RepoOwner(), baseRepo.RepoName(), number)
	} else {
		return fmt.Errorf("invalid issue number: '%s'", args[0])
	}

	fmt.Printf("Opening %s in your browser.\n", openURL)
	return utils.OpenInBrowser(openURL)
}

func issueCreate(cmd *cobra.Command, args []string) error {
	ctx := contextForCommand(cmd)

	baseRepo, err := ctx.BaseRepo()
	if err != nil {
		return err
	}

	if isWeb, err := cmd.Flags().GetBool("web"); err == nil && isWeb {
		// TODO: move URL generation into GitHubRepository
		openURL := fmt.Sprintf("https://github.com/%s/%s/issues/new", baseRepo.RepoOwner(), baseRepo.RepoName())
		// TODO: figure out how to stub this in tests
		if stat, err := os.Stat(".github/ISSUE_TEMPLATE"); err == nil && stat.IsDir() {
			openURL += "/choose"
		}
		return utils.OpenInBrowser(openURL)
	}

	var title string
	var body string

	message, err := cmd.Flags().GetStringArray("message")
	if err != nil {
		return err
	}

	apiClient, err := apiClientForContext(ctx)
	if err != nil {
		return err
	}

	if len(message) > 0 {
		title = message[0]
		body = strings.Join(message[1:], "\n\n")
	} else {
		// TODO: open the text editor for issue title & body
		input := os.Stdin
		if terminal.IsTerminal(int(input.Fd())) {
			cmd.Println("Enter the issue title and body; press Enter + Ctrl-D when done:")
		}
		inputBytes, err := ioutil.ReadAll(input)
		if err != nil {
			return err
		}

		parts := strings.SplitN(string(inputBytes), "\n\n", 2)
		if len(parts) > 0 {
			title = parts[0]
		}
		if len(parts) > 1 {
			body = parts[1]
		}
	}

	if title == "" {
		return fmt.Errorf("aborting due to empty title")
	}
	params := map[string]interface{}{
		"title": title,
		"body":  body,
	}

	newIssue, err := api.IssueCreate(apiClient, baseRepo, params)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), newIssue.URL)
	return nil
}

func printIssues(issues ...api.Issue) {
	for _, issue := range issues {
		fmt.Printf("  #%d %s\n", issue.Number, truncate(70, issue.Title))
	}
}
