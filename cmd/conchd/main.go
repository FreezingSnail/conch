package main

import (
	"log"

	"github.com/FreezingSnail/conch/internal/daemon"
	"github.com/FreezingSnail/conch/internal/db"
	"github.com/FreezingSnail/conch/internal/kiro"
)

func main() {
	database, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	if err := daemon.Run(database, kiro.Kiro{}); err != nil {
		log.Fatal(err)
	}
}
