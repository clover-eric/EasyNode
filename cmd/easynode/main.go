package main

import (
	"embed"
	"flag"
	"log"
	"net/http"
	"time"

	"easynode/internal/api"
	"easynode/internal/store"
)

//go:embed dist dist/assets
var static embed.FS

func main() {
	addr := flag.String("addr", ":8088", "listen address")
	tlsAddr := flag.String("tls-addr", ":8443", "HTTPS listen address")
	dataDir := flag.String("data", "data", "data directory")
	flag.Parse()

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	srv := api.New(st, *dataDir, static)
	go serveHTTPS(*tlsAddr, srv)
	log.Printf("EasyNode listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}

func serveHTTPS(addr string, srv *api.Server) {
	for {
		st := srv.StateSnapshot()
		if st.SetupDone && st.CertReady && st.CertPath != "" && st.KeyPath != "" {
			log.Printf("EasyNode HTTPS listening on %s", addr)
			if err := http.ListenAndServeTLS(addr, st.CertPath, st.KeyPath, srv.Handler()); err != nil {
				log.Printf("EasyNode HTTPS stopped: %v", err)
			}
		}
		time.Sleep(5 * time.Second)
	}
}
