package list

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/internal/config"
	"github.com/cli/cli/internal/ghrepo"
	"github.com/cli/cli/internal/run"
	"github.com/cli/cli/pkg/cmdutil"
	"github.com/cli/cli/pkg/httpmock"
	"github.com/cli/cli/pkg/iostreams"
	"github.com/cli/cli/test"
	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
)

func runCommand(rt http.RoundTripper, isTTY bool, cli string) (*test.CmdOut, error) {
	io, _, stdout, stderr := iostreams.Test()
	io.SetStdoutTTY(isTTY)
	io.SetStdinTTY(isTTY)
	io.SetStderrTTY(isTTY)

	factory := &cmdutil.Factory{
		IOStreams: io,
		HttpClient: func() (*http.Client, error) {
			return &http.Client{Transport: rt}, nil
		},
		Config: func() (config.Config, error) {
			return config.NewBlankConfig(), nil
		},
		BaseRepo: func() (ghrepo.Interface, error) {
			return ghrepo.New("OWNER", "REPO"), nil
		},
	}

	cmd := NewCmdList(factory, nil)

	argv, err := shlex.Split(cli)
	if err != nil {
		return nil, err
	}
	cmd.SetArgs(argv)

	cmd.SetIn(&bytes.Buffer{})
	cmd.SetOut(ioutil.Discard)
	cmd.SetErr(ioutil.Discard)

	_, err = cmd.ExecuteC()
	return &test.CmdOut{
		OutBuf: stdout,
		ErrBuf: stderr,
	}, err
}

func TestIssueList_nontty(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.Register(
		httpmock.GraphQL(`query IssueList\b`),
		httpmock.FileResponse("./fixtures/issueList.json"))

	output, err := runCommand(http, false, "")
	if err != nil {
		t.Errorf("error running command `issue list`: %v", err)
	}

	assert.Equal(t, "", output.Stderr())
	//nolint:staticcheck // prefer exact matchers over ExpectLines
	test.ExpectLines(t, output.String(),
		`1[\t]+number won[\t]+label[\t]+\d+`,
		`2[\t]+number too[\t]+label[\t]+\d+`,
		`4[\t]+number fore[\t]+label[\t]+\d+`)
}

func TestIssueList_tty(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.Register(
		httpmock.GraphQL(`query IssueList\b`),
		httpmock.FileResponse("./fixtures/issueList.json"))

	output, err := runCommand(http, true, "")
	if err != nil {
		t.Errorf("error running command `issue list`: %v", err)
	}

	out := output.String()
	timeRE := regexp.MustCompile(`\d+ years`)
	out = timeRE.ReplaceAllString(out, "X years")

	assert.Equal(t, heredoc.Doc(`

		Showing 3 of 3 open issues in OWNER/REPO

		#1  number won   (label)  about X years ago
		#2  number too   (label)  about X years ago
		#4  number fore  (label)  about X years ago
	`), out)
	assert.Equal(t, ``, output.Stderr())
}

func TestIssueList_tty_withFlags(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.Register(
		httpmock.GraphQL(`query IssueList\b`),
		httpmock.GraphQLQuery(`
		{ "data": {	"repository": {
			"hasIssuesEnabled": true,
			"issues": { "nodes": [] }
		} } }`, func(_ string, params map[string]interface{}) {
			assert.Equal(t, "probablyCher", params["assignee"].(string))
			assert.Equal(t, "foo", params["author"].(string))
			assert.Equal(t, "me", params["mention"].(string))
			assert.Equal(t, "12345", params["milestone"].(string))
			assert.Equal(t, []interface{}{"web", "bug"}, params["labels"].([]interface{}))
			assert.Equal(t, []interface{}{"OPEN"}, params["states"].([]interface{}))
		}))

	http.Register(
		httpmock.GraphQL(`query RepositoryMilestoneList\b`),
		httpmock.StringResponse(`
		{ "data": { "repository": { "milestones": {
			"nodes": [{ "title":"1.x", "id": "MDk6TWlsZXN0b25lMTIzNDU=" }],
			"pageInfo": { "hasNextPage": false }
		} } } }
		`))

	output, err := runCommand(http, true, "-a probablyCher -l web,bug -s open -A foo --mention me --milestone 1.x")
	if err != nil {
		t.Errorf("error running command `issue list`: %v", err)
	}

	assert.Equal(t, "", output.Stderr())
	assert.Equal(t, `
No issues match your search in OWNER/REPO

`, output.String())
}

func TestIssueList_atMe(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.Register(
		httpmock.GraphQL(`query UserCurrent\b`),
		httpmock.StringResponse(`{"data": {"viewer": {"login": "monalisa"} } }`))
	http.Register(
		httpmock.GraphQL(`query IssueList\b`),
		httpmock.GraphQLQuery(`
		{ "data": {	"repository": {
			"hasIssuesEnabled": true,
			"issues": { "nodes": [] }
		} } }`, func(_ string, params map[string]interface{}) {
			assert.Equal(t, "monalisa", params["assignee"].(string))
			assert.Equal(t, "monalisa", params["author"].(string))
			assert.Equal(t, "monalisa", params["mention"].(string))
		}))

	_, err := runCommand(http, true, "-a @me -A @me --mention @me")
	if err != nil {
		t.Errorf("error running command `issue list`: %v", err)
	}
}

func TestIssueList_withInvalidLimitFlag(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	_, err := runCommand(http, true, "--limit=0")

	if err == nil || err.Error() != "invalid limit: 0" {
		t.Errorf("error running command `issue list`: %v", err)
	}
}

func TestIssueList_nullAssigneeLabels(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.Register(
		httpmock.GraphQL(`query IssueList\b`),
		httpmock.StringResponse(`
			{ "data": {	"repository": {
				"hasIssuesEnabled": true,
				"issues": { "nodes": [] }
			} } }`),
	)

	_, err := runCommand(http, true, "")
	if err != nil {
		t.Errorf("error running command `issue list`: %v", err)
	}

	bodyBytes, _ := ioutil.ReadAll(http.Requests[0].Body)
	reqBody := struct {
		Variables map[string]interface{}
	}{}
	_ = json.Unmarshal(bodyBytes, &reqBody)

	_, assigneeDeclared := reqBody.Variables["assignee"]
	_, labelsDeclared := reqBody.Variables["labels"]
	assert.Equal(t, false, assigneeDeclared)
	assert.Equal(t, false, labelsDeclared)
}

func TestIssueList_disabledIssues(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.Register(
		httpmock.GraphQL(`query IssueList\b`),
		httpmock.StringResponse(`
			{ "data": {	"repository": {
				"hasIssuesEnabled": false
			} } }`),
	)

	_, err := runCommand(http, true, "")
	if err == nil || err.Error() != "the 'OWNER/REPO' repository has disabled issues" {
		t.Errorf("error running command `issue list`: %v", err)
	}
}

func TestIssueList_web(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	cs, cmdTeardown := run.Stub()
	defer cmdTeardown(t)

	cs.Register(`https://github\.com`, 0, "", func(args []string) {
		url := strings.ReplaceAll(args[len(args)-1], "^", "")
		assert.Equal(t, "https://github.com/OWNER/REPO/issues?q=is%3Aissue+assignee%3Apeter+label%3Abug+label%3Adocs+author%3Ajohn+mentions%3Afrank+milestone%3Av1.1", url)
	})

	output, err := runCommand(http, true, "--web -a peter -A john -l bug -l docs -L 10 -s all --mention frank --milestone v1.1")
	if err != nil {
		t.Errorf("error running command `issue list` with `--web` flag: %v", err)
	}

	assert.Equal(t, "", output.String())
	assert.Equal(t, "Opening github.com/OWNER/REPO/issues in your browser.\n", output.Stderr())
}

func TestIssueList_milestoneNotFound(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.Register(
		httpmock.GraphQL(`query RepositoryMilestoneList\b`),
		httpmock.StringResponse(`
		{ "data": { "repository": { "milestones": {
			"nodes": [{ "title":"1.x", "id": "MDk6TWlsZXN0b25lMTIzNDU=" }],
			"pageInfo": { "hasNextPage": false }
		} } } }
		`))

	_, err := runCommand(http, true, "--milestone NotFound")
	if err == nil || err.Error() != `no milestone found with title "NotFound"` {
		t.Errorf("error running command `issue list`: %v", err)
	}
}

func TestIssueList_milestoneByNumber(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)
	http.Register(
		httpmock.GraphQL(`query RepositoryMilestoneByNumber\b`),
		httpmock.StringResponse(`
		{ "data": { "repository": { "milestone": {
			"id": "MDk6TWlsZXN0b25lMTIzNDU="
		} } } }
		`))
	http.Register(
		httpmock.GraphQL(`query IssueList\b`),
		httpmock.GraphQLQuery(`
		{ "data": {	"repository": {
			"hasIssuesEnabled": true,
			"issues": { "nodes": [] }
		} } }`, func(_ string, params map[string]interface{}) {
			assert.Equal(t, "12345", params["milestone"].(string)) // Database ID for the Milestone (see #1462)
		}))

	_, err := runCommand(http, true, "--milestone 13")
	if err != nil {
		t.Fatalf("error running issue list: %v", err)
	}
}

func TestIssueList_Search_tty(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	http.Register(
		httpmock.GraphQL(`query IssueSearch\b`),
		httpmock.FileResponse("./fixtures/issueSearch.json"))

	output, err := runCommand(http, true, "--search \"auth bug\"")
	if err != nil {
		t.Errorf("error running command `issue list`: %v", err)
	}

	out := output.String()
	timeRE := regexp.MustCompile(`\d+ years`)
	out = timeRE.ReplaceAllString(out, "X years")

	assert.Equal(t, heredoc.Doc(`

		Showing 3 of 3 open issues in OWNER/REPO

		#1  number won   (label)  about X years ago
		#2  number too   (label)  about X years ago
		#4  number fore  (label)  about X years ago
	`), out)
}

func TestIssueList_Search_web(t *testing.T) {
	http := &httpmock.Registry{}
	defer http.Verify(t)

	cs, cmdTeardown := run.Stub()
	defer cmdTeardown(t)

	cs.Register(`https://github\.com`, 0, "", func(args []string) {
		url := strings.ReplaceAll(args[len(args)-1], "^", "")
		assert.Equal(t, "https://github.com/OWNER/REPO/issues?q=is%3Aissue+assignee%3Apeter+label%3Abug+label%3Adocs+author%3Ajohn+mentions%3Afrank+milestone%3Av1.1+transfer", url)
	})

	output, err := runCommand(http, true, "--web -a peter -A john -l bug -l docs -L 10 -s all --mention frank --milestone v1.1 --search transfer")
	if err != nil {
		t.Errorf("error running command `issue list` with `--web` flag: %v", err)
	}

	assert.Equal(t, "", output.String())
	assert.Equal(t, "Opening github.com/OWNER/REPO/issues in your browser.\n", output.Stderr())

}
