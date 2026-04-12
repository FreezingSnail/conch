package db

import (
	"os"
	"testing"
)

func TestDB(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	d, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	ticketID, err := d.CreateTicket("PROJ-1", "test ticket", "desc", "myrepo")
	if err != nil {
		t.Fatal(err)
	}

	if err := d.SetTicketRepo(ticketID, "myrepo", "/tmp/wt/1"); err != nil {
		t.Fatal(err)
	}

	tickets, err := d.ListTickets()
	if err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 1 || tickets[0].ID != ticketID {
		t.Fatalf("expected 1 ticket, got %+v", tickets)
	}
	if tickets[0].Repo != "myrepo" || tickets[0].WorktreePath != "/tmp/wt/1" {
		t.Fatalf("expected repo fields set, got %+v", tickets[0])
	}

	taskID, err := d.CreateTaskWithBody(ticketID, "task one", "do the thing")
	if err != nil {
		t.Fatal(err)
	}
	if taskID == 0 {
		t.Fatal("expected non-zero task id")
	}

	task, err := d.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Body != "do the thing" || task.Status != "todo" {
		t.Fatalf("unexpected task: %+v", task)
	}

	if err := d.UpdateTaskStatus(taskID, "in-progress"); err != nil {
		t.Fatal(err)
	}
	task, _ = d.GetTask(taskID)
	if task.Status != "in-progress" {
		t.Fatalf("expected in-progress, got %s", task.Status)
	}

	task2ID, _ := d.CreateTaskWithBody(ticketID, "task two", "depends on one")
	if err := d.AddDependency(taskID, task2ID); err != nil {
		t.Fatal(err)
	}

	blockers, err := d.ListBlockedBy(task2ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockers) != 1 || blockers[0].ID != taskID {
		t.Fatalf("expected task one to block task two, got %+v", blockers)
	}

	blocking, err := d.ListBlocks(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocking) != 1 || blocking[0].ID != task2ID {
		t.Fatalf("expected task one to block task two, got %+v", blocking)
	}

	if err := d.RemoveDependency(taskID, task2ID); err != nil {
		t.Fatal(err)
	}
	blockers, _ = d.ListBlockedBy(task2ID)
	if len(blockers) != 0 {
		t.Fatal("expected no blockers after removal")
	}

	tasks, err := d.ListTasksByTicket(ticketID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	sessionID, err := d.CreateSession(0, "kiro", "running")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AppendSessionLog(sessionID, "stdout", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateSessionStatus(sessionID, "completed"); err != nil {
		t.Fatal(err)
	}
	sessions, err := d.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Status != "completed" {
		t.Fatalf("expected 1 completed session, got %+v", sessions)
	}

	_ = os.Getenv("HOME")
}

func TestGetTicketByID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	d, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id, err := d.CreateTicket("PROJ-2", "hello", "world", "repo1")
	if err != nil {
		t.Fatal(err)
	}

	ticket, err := d.GetTicketByID(id)
	if err != nil {
		t.Fatalf("expected ticket, got error: %v", err)
	}
	if ticket.ID != id || ticket.Title != "hello" {
		t.Fatalf("unexpected ticket: %+v", ticket)
	}

	_, err = d.GetTicketByID(99999)
	if err == nil {
		t.Fatal("expected error for missing ticket, got nil")
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	d, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestFeedbackNotesCRUD(t *testing.T) {
	d := openTestDB(t)

	ticketID, err := d.CreateTicket("T-1", "t", "", "")
	if err != nil {
		t.Fatal(err)
	}

	id, err := d.CreateFeedbackNote(ticketID, "abc123", "main.go", "@@ -1,3 +1,4 @@", "initial body")
	if err != nil {
		t.Fatal(err)
	}

	// list by ticket — expect 1
	notes, err := d.ListFeedbackNotesByTicket(ticketID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].ID != id {
		t.Fatalf("expected 1 note, got %+v", notes)
	}

	// list by hunk — expect 1
	byHunk, err := d.ListFeedbackNotesByHunk(ticketID, "abc123", "main.go", "@@ -1,3 +1,4 @@")
	if err != nil {
		t.Fatal(err)
	}
	if len(byHunk) != 1 || byHunk[0].ID != id {
		t.Fatalf("expected 1 note by hunk, got %+v", byHunk)
	}

	// update body
	if err := d.UpdateFeedbackNote(id, "updated body"); err != nil {
		t.Fatal(err)
	}
	notes, _ = d.ListFeedbackNotesByTicket(ticketID)
	if notes[0].Body != "updated body" {
		t.Fatalf("expected updated body, got %q", notes[0].Body)
	}

	// delete — list should return 0
	if err := d.DeleteFeedbackNote(id); err != nil {
		t.Fatal(err)
	}
	notes, _ = d.ListFeedbackNotesByTicket(ticketID)
	if len(notes) != 0 {
		t.Fatalf("expected 0 notes after delete, got %d", len(notes))
	}
}

func TestMarkNotesAddressed(t *testing.T) {
	d := openTestDB(t)

	t1, _ := d.CreateTicket("T-1", "t1", "", "")
	t2, _ := d.CreateTicket("T-2", "t2", "", "")

	d.CreateFeedbackNote(t1, "abc", "a.go", "@@ -1 @@", "note for t1")
	d.CreateFeedbackNote(t2, "def", "b.go", "@@ -2 @@", "note for t2")

	if err := d.MarkNotesAddressed(t1); err != nil {
		t.Fatal(err)
	}

	t1Notes, _ := d.ListFeedbackNotesByTicket(t1)
	if !t1Notes[0].Addressed {
		t.Fatal("expected t1 note to be addressed")
	}

	t2Notes, _ := d.ListFeedbackNotesByTicket(t2)
	if t2Notes[0].Addressed {
		t.Fatal("expected t2 note to remain unaddressed")
	}
}

func TestUpsertPR_idempotent(t *testing.T) {
	d := openTestDB(t)

	id1, err := d.UpsertPR("myrepo", 42, "original title", "alice", "http://example.com/1", "sha1")
	if err != nil {
		t.Fatal(err)
	}

	id2, err := d.UpsertPR("myrepo", 42, "updated title", "alice", "http://example.com/1", "sha2")
	if err != nil {
		t.Fatal(err)
	}

	if id1 != id2 {
		t.Fatalf("expected same id on upsert, got %d and %d", id1, id2)
	}

	prs, err := d.ListPRsByStatus("open")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Title != "updated title" {
		t.Fatalf("expected updated title, got %q", prs[0].Title)
	}
	if prs[0].HeadSHA != "sha2" {
		t.Fatalf("expected sha2, got %q", prs[0].HeadSHA)
	}
}

func TestListPRsByStatus(t *testing.T) {
	d := openTestDB(t)

	d.UpsertPR("repo", 1, "open PR", "alice", "u1", "s1")
	id2, _ := d.UpsertPR("repo", 2, "in_review PR", "bob", "u2", "s2")
	id3, _ := d.UpsertPR("repo", 3, "completed PR", "carol", "u3", "s3")

	d.UpdatePRStatus(id2, "in_review")
	d.UpdatePRStatus(id3, "completed")

	open, err := d.ListPRsByStatus("open")
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].Title != "open PR" {
		t.Fatalf("expected 1 open PR, got %+v", open)
	}

	inReview, _ := d.ListPRsByStatus("in_review")
	if len(inReview) != 1 || inReview[0].ID != id2 {
		t.Fatalf("expected 1 in_review PR, got %+v", inReview)
	}

	completed, _ := d.ListPRsByStatus("completed")
	if len(completed) != 1 || completed[0].ID != id3 {
		t.Fatalf("expected 1 completed PR, got %+v", completed)
	}
}

func TestPRCommentCRUD(t *testing.T) {
	d := openTestDB(t)

	prID, err := d.UpsertPR("repo", 10, "my PR", "dev", "http://x", "abc")
	if err != nil {
		t.Fatal(err)
	}

	// create comment
	cID, err := d.CreatePRComment(prID, "suggestion", "main.go", 5, "use X instead")
	if err != nil {
		t.Fatal(err)
	}

	// list — expect 1 unapproved, unpushed
	comments, err := d.ListPRComments(prID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].ID != cID {
		t.Fatalf("expected 1 comment, got %+v", comments)
	}
	if comments[0].Approved || comments[0].Pushed {
		t.Fatalf("expected unapproved and unpushed, got %+v", comments[0])
	}

	// approve
	if err := d.SetPRCommentApproved(cID, true); err != nil {
		t.Fatal(err)
	}
	comments, _ = d.ListPRComments(prID)
	if !comments[0].Approved {
		t.Fatal("expected comment to be approved")
	}

	// un-approve
	if err := d.SetPRCommentApproved(cID, false); err != nil {
		t.Fatal(err)
	}
	comments, _ = d.ListPRComments(prID)
	if comments[0].Approved {
		t.Fatal("expected comment to be unapproved")
	}

	// mark pushed
	if err := d.SetPRCommentPushed(cID); err != nil {
		t.Fatal(err)
	}
	comments, _ = d.ListPRComments(prID)
	if !comments[0].Pushed {
		t.Fatal("expected comment to be pushed")
	}
}
