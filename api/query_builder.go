package api

import (
	"strings"
)

func squeeze(r rune) rune {
	switch r {
	case '\n', '\t':
		return -1
	default:
		return r
	}
}

func shortenQuery(q string) string {
	return strings.Map(squeeze, q)
}

var issueComments = shortenQuery(`
	comments(last: 100) {
		nodes {
			author{login},
			authorAssociation,
			body,
			createdAt,
			includesCreatedEdit,
			isMinimized,
			minimizedReason,
			reactionGroups{content,users{totalCount}}
		},
		totalCount
	}
`)

var prReviewRequests = shortenQuery(`
	reviewRequests(last: 100) {
		nodes {
			requestedReviewer {
				__typename,
				...on User{login},
				...on Team{name}
			}
		},
		totalCount
	}
`)

var prStatusCheckRollup = shortenQuery(`
	commits(last: 1) {
		totalCount,
		nodes {
			commit {
				oid,
				statusCheckRollup {
					contexts(last: 100) {
						nodes {
							__typename
							...on StatusContext {
								context,
								state,
								targetUrl
							},
							...on CheckRun {
								name,
								status,
								conclusion,
								startedAt,
								completedAt,
								detailsUrl
							}
						}
					}
				}
			}
		}
	}
`)

var IssueFields = []string{
	"assignees",
	"author",
	"body",
	"closed",
	"comments",
	"createdAt",
	"closedAt",
	"id",
	"labels",
	"milestone",
	"number",
	"projectCards",
	"reactionGroups",
	"state",
	"title",
	"updatedAt",
	"url",
}

var PullRequestFields = append(IssueFields,
	"additions",
	"baseRefName",
	"deletions",
	"headRefName",
	"headRepository",
	"headRepositoryOwner",
	"isCrossRepository",
	"isDraft",
	"maintainerCanModify",
	"mergeable",
	"mergedAt",
	"mergeStateStatus",
	"reviewDecision",
	"reviewRequests",
	"statusCheckRollup",
)

func PullRequestGraphQL(fields []string) string {
	var q []string
	for _, field := range fields {
		switch field {
		case "author":
			q = append(q, `author{login}`)
		case "headRepositoryOwner":
			q = append(q, `headRepositoryOwner{login}`)
		case "headRepository":
			q = append(q, `headRepository{name}`)
		case "assignees":
			q = append(q, `assignees(first:100){nodes{login},totalCount}`)
		case "labels":
			q = append(q, `labels(first:100){nodes{name},totalCount}`)
		case "projectCards":
			q = append(q, `projectCards(first:100){nodes{project{name}column{name}},totalCount}`)
		case "milestone":
			q = append(q, `milestone{title}`)
		case "reactionGroups":
			q = append(q, `reactionGroups{content,users{totalCount}}`)
		case "comments":
			q = append(q, issueComments)
		case "reviewRequests":
			q = append(q, prReviewRequests)
		case "statusCheckRollup":
			q = append(q, prStatusCheckRollup)
		default:
			q = append(q, field)
		}
	}
	return strings.Join(q, ",")
}
