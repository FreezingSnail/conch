package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/config"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/kiro"
	"github.com/FreezingSnail/conch/internal/review"
)

// handlePRs routes PR actions. Returns (resp, true) for known actions,
// (zero, false) otherwise.
func handlePRs(req client.Request, database *db.DB) (client.Response, bool) {
	switch req.Action {
	case "poll_prs":
		pollPRs(database)
		return client.Response{OK: true}, true

	case "set_pr_comment_approved":
		if req.CommentID == 0 {
			return client.Response{Error: "comment_id required"}, true
		}
		if err := database.SetPRCommentApproved(req.CommentID, req.Approved); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true}, true

	case "push_pr_comment":
		if req.CommentID == 0 {
			return client.Response{Error: "comment_id required"}, true
		}
		comment, err := database.GetPRCommentByID(req.CommentID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		pr, err := database.GetPRByID(comment.PRID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		ownerRepo, err := repoOwnerSlug(pr.Repo)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		cmd := exec.Command("gh", "api",
			fmt.Sprintf("repos/%s/pulls/%d/comments", ownerRepo, pr.PRNumber),
			"-f", "body="+comment.Body,
			"-f", "path="+comment.FilePath,
			"-F", fmt.Sprintf("line=%d", comment.Line),
			"-f", "commit_id="+pr.HeadSHA,
			"-f", "side=RIGHT",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return client.Response{Error: string(out)}, true
		}
		if err := database.SetPRCommentPushed(req.CommentID); err != nil {
			return client.Response{Error: err.Error()}, true
		}
		allPushed, err := database.AllPRCommentsPushed(pr.ID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		if allPushed {
			if err := database.UpdatePRStatus(pr.ID, "completed"); err != nil {
				return client.Response{Error: err.Error()}, true
			}
		}
		return client.Response{OK: true}, true

	case "list_prs":
		prs, err := database.ListPRsByStatus(req.Status)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, PRs: prs}, true

	case "review_start":
		if req.PRID == 0 {
			return client.Response{Error: "pr_id required"}, true
		}
		pr, err := database.GetPRByID(req.PRID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		ownerRepo, err := repoOwnerSlug(pr.Repo)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		diffOut, err := exec.Command("gh", "pr", "diff", strconv.Itoa(pr.PRNumber), "--repo", ownerRepo).Output()
		if err != nil {
			return client.Response{Error: "gh pr diff: " + err.Error()}, true
		}
		prompt := kiro.BuildPRReviewPrompt(pr.PRNumber, ownerRepo, string(diffOut))
		tmpDir, _ := os.MkdirTemp("", "conch-review-*")
		reviewCfg, _ := config.Load()
		seedKiroConfig(tmpDir, reviewCfg.EffectiveSlugMode())
		sessionID, err := database.CreateSession(0, "kiro-pr-reviewer", "running")
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		database.UpdatePRStatus(req.PRID, "in_review") //nolint:errcheck
		go runPRReviewer(sessionID, req.PRID, prompt, tmpDir, database)
		return client.Response{OK: true, ID: sessionID}, true

	case "list_pr_comments":
		if req.PRID == 0 {
			return client.Response{Error: "pr_id required"}, true
		}
		comments, err := database.ListPRComments(req.PRID)
		if err != nil {
			return client.Response{Error: err.Error()}, true
		}
		return client.Response{OK: true, PRComments: comments}, true

	default:
		return client.Response{}, false
	}
}

// pollPRs discovers open PRs across all configured repos and upserts them into
// the database. Per-repo errors are skipped so a single bad repo does not abort
// the whole poll.
func pollPRs(database *db.DB) {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	repos, err := config.FindRepos(cfg.WorkDirs)
	if err != nil {
		return
	}
	for _, repo := range repos {
		slug, err := repoOwnerSlug(repo)
		if err != nil {
			continue
		}
		cmd := exec.Command("gh", "pr", "list", "--repo", slug, "--state", "open",
			"--json", "number,title,author,url,headRefOid")
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		var prs []struct {
			Number     int                    `json:"number"`
			Title      string                 `json:"title"`
			Author     struct{ Login string } `json:"author"`
			URL        string                 `json:"url"`
			HeadRefOid string                 `json:"headRefOid"`
		}
		if err := json.Unmarshal(out, &prs); err != nil {
			continue
		}
		for _, pr := range prs {
			database.UpsertPR(repo, pr.Number, pr.Title, pr.Author.Login, pr.URL, pr.HeadRefOid) //nolint:errcheck
		}
	}
}

// startPRPoller runs pollPRs every 5 minutes in a background goroutine.
func startPRPoller(database *db.DB) {
	go func() {
		for {
			pollPRs(database)
			time.Sleep(5 * time.Minute)
		}
	}()
}

// runPRReviewer spawns a headless kiro pr-reviewer agent in tmpDir, streams its
// output into session logs, parses the resulting .conch-review.md, and persists
// the comments. On success the PR status is set to "ready"; on failure it reverts
// to "open".
func runPRReviewer(sessionID, prID int64, prompt, tmpDir string, database *db.DB) {
	cmd := kiro.Kiro{}.BackgroundWithAgent("pr-reviewer", prompt, tmpDir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		database.UpdateSessionStatus(sessionID, "error") //nolint:errcheck
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		database.UpdateSessionStatus(sessionID, "error") //nolint:errcheck
		return
	}
	combined := io.MultiReader(stdout, stderr)
	if err := cmd.Start(); err != nil {
		database.UpdateSessionStatus(sessionID, "error")           //nolint:errcheck
		database.AppendSessionLog(sessionID, "error", err.Error()) //nolint:errcheck
		return
	}
	scanner := bufio.NewScanner(combined)
	for scanner.Scan() {
		database.AppendSessionLog(sessionID, "stdout", scanner.Text()) //nolint:errcheck
	}
	if err := cmd.Wait(); err != nil {
		database.UpdatePRStatus(prID, "open")                           //nolint:errcheck
		database.UpdateSessionStatus(sessionID, "error")                //nolint:errcheck
		database.AppendSessionLog(sessionID, "exit_error", err.Error()) //nolint:errcheck
		return
	}
	// Parse the review file written by the agent and persist each comment.
	comments, _ := review.ParseReviewFile(filepath.Join(tmpDir, ".conch-review.md"))
	for _, c := range comments {
		database.CreatePRComment(prID, c.Type, c.FilePath, c.Line, c.Body) //nolint:errcheck
	}
	database.UpdatePRStatus(prID, "ready")               //nolint:errcheck
	database.UpdateSessionStatus(sessionID, "completed") //nolint:errcheck
}

// repoOwnerSlug runs "git remote get-url origin" in repoPath and extracts the
// "owner/repo" slug from both HTTPS and SSH remote URL formats.
func repoOwnerSlug(repoPath string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git remote get-url: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	raw = strings.TrimSuffix(raw, ".git")
	// SSH format: git@github.com:owner/repo
	if idx := strings.Index(raw, ":"); idx != -1 && !strings.Contains(raw[:idx], "/") {
		return raw[idx+1:], nil
	}
	// HTTPS format: https://github.com/owner/repo
	parts := strings.Split(raw, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("cannot parse owner/repo from %q", raw)
	}
	return strings.Join(parts[len(parts)-2:], "/"), nil
}
