package main

import (
	"log"
)

func main() {
	cfg := LoadConfig()
	server, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	log.Printf("starting go panel backend on %s", cfg.Addr)
	if cfg.AdminPassword != "" {
		log.Printf("bootstrap admin username: %s", cfg.AdminUsername)
	}
	if err := server.Start(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
