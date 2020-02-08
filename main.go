package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v29/github"
	"github.com/pkg/errors"
)

// Check is our status check
type Check struct {
	g *github.Client

	Org  string
	Repo string
	PR   int
}

func main() {
	client := github.NewClient(nil)
	ctx := context.Background()

	c := &Check{
		g: client,

		// Org: "multitheftauto", Repo: "mtasa-blue", PR: 1031,
		// Org: "facebook", Repo: "react", PR: 7311,
	}

	// Get data from environment
	c.Org, c.Repo, c.PR = getEnvInfo()

	ok := false
	defer func() {
		if ok {
			os.Exit(0)
			return
		}
		os.Exit(1)
	}()

	left, err := c.isReady(ctx)
	if err != nil {
		fmt.Printf("Internal error: %s\n", err.Error())
		ok = false
	} else {
		ok = left == 0
	}

	if err != nil {
		fmt.Println("::error::Internal error")
		return
	}

	if !ok {
		if left > 0 {
			fmt.Printf("::error:: %d approvals required\n", left)
		} else {
			fmt.Println("::error:: Approvals required")
		}
		return
	}

	fmt.Println("::set-output::Everyone is happy!")
}

func (c *Check) getReviews(ctx context.Context) ([]*github.PullRequestReview, error) {
	var allReviews []*github.PullRequestReview
	opts := &github.ListOptions{}

	for {
		reviews, resp, err := c.g.PullRequests.ListReviews(ctx, c.Org, c.Repo, c.PR, opts)
		if err != nil {
			return nil, err
		}

		allReviews = append(allReviews, reviews...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allReviews, nil
}

func (c *Check) isReady(ctx context.Context) (int, error) {
	reviews, err := c.getReviews(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "could not get reviews")
	}

	acceptances := make(map[int64]bool)
	for _, review := range reviews {
		// We only care about collaborators
		assoc := review.GetAuthorAssociation()
		if assoc != "COLLABORATOR" && assoc != "OWNER" && assoc != "MEMBER" {
			continue
		}

		// Ignore if it was just a comment
		if review.GetState() == "COMMENTED" {
			continue
		}

		uid := review.GetUser().GetID()

		// Forget their response if it was dismissed
		if review.GetState() == "DISMISSED" {
			delete(acceptances, uid)
			continue
		}

		// Reviews are in chronological order, so just take the last approval
		acceptances[uid] = review.GetState() == "APPROVED"
		fmt.Printf("%d\t\t%s\t%#v\n", uid, review.GetState(), acceptances[uid])
	}

	needed := 1
	if len(acceptances) == 0 {
		return needed, nil
	}

	for i, ok := range acceptances {
		if !ok {
			fmt.Println("no", i)
			return -1, nil
		}
	}

	return needed, nil
}

func getEnvInfo() (org string, repo string, pr int) {
	repoStrs := strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")
	org = repoStrs[0]
	repo = repoStrs[1]

	var err error
	pr, err = strconv.Atoi(
		strings.TrimSuffix(
			strings.TrimPrefix(
				os.Getenv("GITHUB_REF"),
				"refs/pull/"),
			"/merge",
		),
	)

	if err != nil {
		panic(err.Error())
	}
	return
}
