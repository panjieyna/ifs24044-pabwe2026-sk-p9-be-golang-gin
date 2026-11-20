// Delcom Blog API — titik masuk utama (dibangun oleh pipeline dengan `go build .`).
package main

import (
	"log"

	"app/internal/config"
	"app/internal/server"
)

func main() {
	if err := server.Run(config.LoadConfig()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
