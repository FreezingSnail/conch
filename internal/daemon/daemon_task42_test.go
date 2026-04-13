package daemon

import (
	"testing"

	"github.com/FreezingSnail/conch/internal/client"
	"github.com/FreezingSnail/conch/internal/kiro"
)

// TestDispatch_ping: action="ping" → OK=true.
func TestDispatch_ping(t *testing.T) {
	database := openTestDB(t)
	resp := dispatch(client.Request{Action: "ping"}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("expected OK=true, got error: %s", resp.Error)
	}
}

// TestDispatch_unknownAction: unknown action → Error="unknown action".
func TestDispatch_unknownAction(t *testing.T) {
	database := openTestDB(t)
	resp := dispatch(client.Request{Action: "nonexistent"}, database, kiro.Kiro{})
	if resp.OK {
		t.Fatal("expected OK=false for unknown action")
	}
	if resp.Error != "unknown action" {
		t.Fatalf("expected error %q, got %q", "unknown action", resp.Error)
	}
}

// TestDispatch_listTickets_empty: list_tickets on empty DB → OK=true, empty Tickets slice.
func TestDispatch_listTickets_empty(t *testing.T) {
	database := openTestDB(t)
	resp := dispatch(client.Request{Action: "list_tickets"}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("list_tickets: %s", resp.Error)
	}
	if len(resp.Tickets) != 0 {
		t.Fatalf("expected empty tickets, got %d", len(resp.Tickets))
	}
}

// TestDispatch_createAndListTasks: create_task then list_tasks → task appears.
func TestDispatch_createAndListTasks(t *testing.T) {
	database := openTestDB(t)
	ticketID := seedTicket(t, database)

	resp := dispatch(client.Request{Action: "create_task", TicketID: ticketID, Title: "my task"}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("create_task: %s", resp.Error)
	}
	taskID := resp.ID
	if taskID == 0 {
		t.Fatal("expected non-zero task ID")
	}

	resp = dispatch(client.Request{Action: "list_tasks", TicketID: ticketID}, database, kiro.Kiro{})
	if !resp.OK {
		t.Fatalf("list_tasks: %s", resp.Error)
	}
	if len(resp.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].Title != "my task" {
		t.Fatalf("unexpected task title: %q", resp.Tasks[0].Title)
	}
}
