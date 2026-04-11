package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/FreezingSnail/conch/internal/client"
)

const taskUsage = `usage: conch task <subcommand> [flags]

subcommands:
  create          --ticket <id> --title <title> --body <context>
  get             --task <id>
  update-status   --task <id> --status <status>
  list            --ticket <id>
  add-dep         --blocker <id> --blocked <id>
  remove-dep      --blocker <id> --blocked <id>
  list-blocked-by --task <id>
  list-blocks     --task <id>
`

func runTask(args []string) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(taskUsage)
		return
	}

	sub := args[0]
	rest := args[1:]

	var req client.Request

	switch sub {
	case "create":
		fs := flag.NewFlagSet("create", flag.ExitOnError)
		ticket := fs.Int64("ticket", 0, "ticket ID")
		title := fs.String("title", "", "task title")
		body := fs.String("body", "", "task body")
		fs.Parse(rest) //nolint:errcheck
		if *ticket == 0 || *title == "" {
			fmt.Fprintln(os.Stderr, "create: --ticket and --title required")
			os.Exit(1)
		}
		req = client.Request{Action: "create_task", TicketID: *ticket, Title: *title, Body: *body}

	case "get":
		fs := flag.NewFlagSet("get", flag.ExitOnError)
		task := fs.Int64("task", 0, "task ID")
		fs.Parse(rest) //nolint:errcheck
		if *task == 0 {
			fmt.Fprintln(os.Stderr, "get: --task required")
			os.Exit(1)
		}
		req = client.Request{Action: "get_task", TaskID: *task}

	case "update-status":
		fs := flag.NewFlagSet("update-status", flag.ExitOnError)
		task := fs.Int64("task", 0, "task ID")
		status := fs.String("status", "", "new status")
		fs.Parse(rest) //nolint:errcheck
		if *task == 0 || *status == "" {
			fmt.Fprintln(os.Stderr, "update-status: --task and --status required")
			os.Exit(1)
		}
		req = client.Request{Action: "update_task_status", TaskID: *task, Status: *status}

	case "list":
		fs := flag.NewFlagSet("list", flag.ExitOnError)
		ticket := fs.Int64("ticket", 0, "ticket ID")
		fs.Parse(rest) //nolint:errcheck
		if *ticket == 0 {
			fmt.Fprintln(os.Stderr, "list: --ticket required")
			os.Exit(1)
		}
		req = client.Request{Action: "list_tasks", TicketID: *ticket}

	case "add-dep":
		fs := flag.NewFlagSet("add-dep", flag.ExitOnError)
		blocker := fs.Int64("blocker", 0, "blocker task ID")
		blocked := fs.Int64("blocked", 0, "blocked task ID")
		fs.Parse(rest) //nolint:errcheck
		if *blocker == 0 || *blocked == 0 {
			fmt.Fprintln(os.Stderr, "add-dep: --blocker and --blocked required")
			os.Exit(1)
		}
		req = client.Request{Action: "add_dep", BlockerID: *blocker, BlockedID: *blocked}

	case "remove-dep":
		fs := flag.NewFlagSet("remove-dep", flag.ExitOnError)
		blocker := fs.Int64("blocker", 0, "blocker task ID")
		blocked := fs.Int64("blocked", 0, "blocked task ID")
		fs.Parse(rest) //nolint:errcheck
		if *blocker == 0 || *blocked == 0 {
			fmt.Fprintln(os.Stderr, "remove-dep: --blocker and --blocked required")
			os.Exit(1)
		}
		req = client.Request{Action: "remove_dep", BlockerID: *blocker, BlockedID: *blocked}

	case "list-blocked-by":
		fs := flag.NewFlagSet("list-blocked-by", flag.ExitOnError)
		task := fs.Int64("task", 0, "task ID")
		fs.Parse(rest) //nolint:errcheck
		if *task == 0 {
			fmt.Fprintln(os.Stderr, "list-blocked-by: --task required")
			os.Exit(1)
		}
		req = client.Request{Action: "list_blocked_by", TaskID: *task}

	case "list-blocks":
		fs := flag.NewFlagSet("list-blocks", flag.ExitOnError)
		task := fs.Int64("task", 0, "task ID")
		fs.Parse(rest) //nolint:errcheck
		if *task == 0 {
			fmt.Fprintln(os.Stderr, "list-blocks: --task required")
			os.Exit(1)
		}
		req = client.Request{Action: "list_blocks", TaskID: *task}

	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n%s", sub, taskUsage)
		os.Exit(1)
	}

	resp, err := client.Send(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "task:", err)
		os.Exit(1)
	}
	b, _ := json.Marshal(resp)
	fmt.Println(string(b))
}
