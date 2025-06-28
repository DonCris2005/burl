package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/tomnomnom/burl/checker"
)

type checkRequest struct {
	URLs    []string `json:"urls"`
	Threads int      `json:"threads"`
}

type checkResponse struct {
	Results []checker.Result `json:"results"`
}

func handleCheck(w http.ResponseWriter, r *http.Request) {
	var req checkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	results := checker.CheckURLs(req.URLs, req.Threads)
	json.NewEncoder(w).Encode(checkResponse{Results: results})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	http.HandleFunc("/check", handleCheck)
	http.HandleFunc("/health", handleHealth)

	log.Printf("server listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
