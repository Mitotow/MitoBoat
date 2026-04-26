package main

import (
	"fmt"

	"mitoboat/internal/config"
	"mitoboat/internal/db"
)

func main() {
	fmt.Println("[BOT] Initializing MitoBoat ...")
	config.LoadEnv()
	db.ConnectDb()
}
