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
