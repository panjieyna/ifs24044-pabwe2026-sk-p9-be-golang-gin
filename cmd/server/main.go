// Titik masuk alternatif sesuai struktur materi: `go run cmd/server/main.go`.
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
