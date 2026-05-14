package main

import (
	"embed"
	"flag"
	"log"
	"net/http"

	"easynode/internal/api"
	"easynode/internal/store"
)

//go:embed dist dist/assets
var static embed.FS

func main() {
	addr := flag.String("addr", ":8088", "listen address")
	dataDir := flag.String("data", "data", "data directory")
	flag.Parse()

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	srv := api.New(st, *dataDir, static)
	log.Printf("EasyNode listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
