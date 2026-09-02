// Copyright 2026 Redpanda Data, Inc.
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

// Command prstatus reports a PR's CI failures and unresolved review threads.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ghPR struct {
	Number            int    `json:"number"`
	Title             string `json:"title"`
	URL               string `json:"url"`
	HeadRefName       string `json:"headRefName"`
	HeadRefOid        string `json:"headRefOid"`
	ReviewDecision    string `json:"reviewDecision"`
	StatusCheckRollup []struct {
		TypeName   string `json:"__typename"`
		Name       string `json:"name"`
		Context    string `json:"context"`
		State      string `json:"state"`
		Conclusion string `json:"conclusion"`
		TargetURL  string `json:"targetUrl"`
		DetailsURL string `json:"detailsUrl"`
	} `json:"statusCheckRollup"`
}

type bkBuild struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	WebURL string `json:"web_url"`
	Jobs   []struct {
		Type   string `json:"type"`
		Name   string `json:"name"`
		State  string `json:"state"`
		WebURL string `json:"web_url"`
		LogURL string `json:"log_url"`
	} `json:"jobs"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "prstatus:", err)
		os.Exit(1)
	}
}

func run() error {
	maxLines := flag.Int("max-lines", 25, "failure lines kept per job")
	org := flag.String("org", "redpanda", "Buildkite organization slug")
	pipeline := flag.String("pipeline", "terraform-provider-redpanda", "Buildkite pipeline slug")
	logDir := flag.String("log-dir", "", "directory for full job logs (default .logs/pr-<N>)")
	buildNum := flag.Int("build", 0, "Buildkite build number to triage instead of the PR head's latest build")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	rep, err := build(ctx, flag.Arg(0), *org, *pipeline, *logDir, *buildNum, *maxLines)
	if err != nil {
		return err
	}
	fmt.Print(Render(rep))
	return nil
}

func build(ctx context.Context, prArg, org, pipeline, logDir string, buildNum, maxLines int) (Report, error) {
	pr, err := fetchPR(ctx, prArg)
	if err != nil {
		return Report{}, err
	}
	rep := Report{PR: PRInfo{Number: pr.Number, Title: pr.Title, URL: pr.URL, Branch: pr.HeadRefName, Head: pr.HeadRefOid, ReviewDecision: pr.ReviewDecision}}

	token := os.Getenv("BUILDKITE_API_TOKEN")
	if token != "" {
		if logDir == "" {
			logDir = filepath.Join(".logs", "pr-"+strconv.Itoa(pr.Number))
		}
		var logs map[string]string
		rep.Failures, logs, err = buildkiteFailures(ctx, token, org, pipeline, pr.HeadRefOid, buildNum, maxLines)
		if err != nil {
			rep.Note = "Buildkite: " + err.Error()
		} else if len(logs) > 0 {
			if _, err := WriteLogs(logDir, logs); err != nil {
				return rep, err
			}
			rep.LogDir = logDir
		}
	}
	if token == "" || rep.Note != "" {
		for _, c := range pr.StatusCheckRollup {
			state := strings.ToUpper(c.State + c.Conclusion)
			if state != "FAILURE" && state != "ERROR" && state != "TIMED_OUT" {
				continue
			}
			name := c.Name
			if name == "" {
				name = strings.TrimPrefix(c.Context, "buildkite/"+pipeline+"/")
			}
			link := c.TargetURL
			if link == "" {
				link = c.DetailsURL
			}
			rep.Failures = append(rep.Failures, JobFailure{Job: name, State: strings.ToLower(state), URL: link})
		}
		if token == "" {
			rep.Note = "Set BUILDKITE_API_TOKEN (scopes: read_builds, read_build_logs) to include failure lines from job logs."
		}
	}

	rep.Threads, err = fetchThreads(ctx, pr.Number)
	if err != nil {
		return rep, err
	}
	return rep, nil
}

func gh(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args[:min(2, len(args))], " "), err)
	}
	return out, nil
}

func fetchPR(ctx context.Context, prArg string) (ghPR, error) {
	args := []string{"pr", "view", "--json", "number,title,url,headRefName,headRefOid,reviewDecision,statusCheckRollup"}
	if prArg != "" {
		if _, err := strconv.Atoi(prArg); err != nil {
			return ghPR{}, fmt.Errorf("PR argument must be a number, got %q", prArg)
		}
		args = append(args, prArg)
	}
	out, err := gh(ctx, args...)
	if err != nil {
		return ghPR{}, err
	}
	var pr ghPR
	if err := json.Unmarshal(out, &pr); err != nil {
		return ghPR{}, fmt.Errorf("decode gh pr view: %w", err)
	}
	return pr, nil
}

const threadsQuery = `query($owner:String!,$repo:String!,$pr:Int!){
  repository(owner:$owner,name:$repo){ pullRequest(number:$pr){
    reviewThreads(first:100){ nodes{ isResolved isOutdated path line
      comments(first:50){ nodes{ author{login} body url } } } } } } }`

func fetchThreads(ctx context.Context, number int) ([]ReviewThread, error) {
	repoOut, err := gh(ctx, "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return nil, err
	}
	owner, repo, ok := strings.Cut(strings.TrimSpace(string(repoOut)), "/")
	if !ok {
		return nil, fmt.Errorf("unexpected repo name %q", repoOut)
	}
	out, err := gh(ctx, "api", "graphql", "-f", "query="+threadsQuery, "-F", "owner="+owner, "-F", "repo="+repo, "-F", "pr="+strconv.Itoa(number))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						Nodes []struct {
							IsResolved bool   `json:"isResolved"`
							IsOutdated bool   `json:"isOutdated"`
							Path       string `json:"path"`
							Line       int    `json:"line"`
							Comments   struct {
								Nodes []struct {
									Author struct {
										Login string `json:"login"`
									} `json:"author"`
									Body string `json:"body"`
									URL  string `json:"url"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decode review threads: %w", err)
	}
	var threads []ReviewThread
	for _, n := range resp.Data.Repository.PullRequest.ReviewThreads.Nodes {
		if n.IsResolved || len(n.Comments.Nodes) == 0 {
			continue
		}
		first := n.Comments.Nodes[0]
		threads = append(threads, ReviewThread{Path: n.Path, Line: n.Line, Author: first.Author.Login, Body: first.Body, URL: first.URL})
	}
	return threads, nil
}

func buildkiteFailures(ctx context.Context, token, org, pipeline, sha string, buildNum, maxLines int) ([]JobFailure, map[string]string, error) {
	base := "https://api.buildkite.com/v2/organizations/" + url.PathEscape(org) + "/pipelines/" + url.PathEscape(pipeline)
	var builds []bkBuild
	if buildNum > 0 {
		var b bkBuild
		if err := bkGet(ctx, token, base+"/builds/"+strconv.Itoa(buildNum), &b); err != nil {
			return nil, nil, err
		}
		builds = []bkBuild{b}
	} else if err := bkGet(ctx, token, base+"/builds?commit="+url.QueryEscape(sha)+"&per_page=1", &builds); err != nil {
		return nil, nil, err
	}
	if len(builds) == 0 {
		return nil, nil, fmt.Errorf("no build for %s", shortSHA(sha))
	}
	var failures []JobFailure
	logs := map[string]string{}
	for _, j := range builds[0].Jobs {
		if j.Type != "script" || !IsFailedJob(j.State) {
			continue
		}
		f := JobFailure{Job: j.Name, State: j.State, URL: j.WebURL}
		var log struct {
			Content string `json:"content"`
		}
		if err := bkGet(ctx, token, j.LogURL, &log); err != nil {
			f.Lines = []string{"log unavailable: " + err.Error()}
		} else {
			f.Lines = ExtractFailures(log.Content, maxLines)
			logs[j.Name] = log.Content
		}
		failures = append(failures, f)
	}
	return failures, logs, nil
}

func bkGet(ctx context.Context, token, endpoint string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", endpoint, resp.StatusCode)
	}
	if len(body) == 0 {
		return fmt.Errorf("%s: empty response", endpoint)
	}
	return json.Unmarshal(body, v)
}
