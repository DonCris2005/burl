package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func readURLs(r io.Reader) ([]string, error) {
	var urls []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		urls = append(urls, sc.Text())
	}
	return urls, sc.Err()
}

type checkRequest struct {
	URLs    []string `json:"urls"`
	Threads int      `json:"threads"`
}

type checkResponse struct {
	Results []struct {
		URL    string `json:"url"`
		Status string `json:"status,omitempty"`
		Error  string `json:"error,omitempty"`
	} `json:"results"`
}

func main() {
	errFile, err := os.OpenFile("Error.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("failed to open error log: %v", err)
	}
	defer errFile.Close()
	log.SetOutput(io.MultiWriter(os.Stderr, errFile))
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic recovered: %v", r)
		}
	}()

	var file string
	var threadTotal int
	var serverList string
	flag.StringVar(&file, "file", "", "file with URLs (default stdin)")
	flag.IntVar(&threadTotal, "threads", 10, "total threads across servers")
	flag.StringVar(&serverList, "servers", "", "comma separated list of server addresses (host:port)")
	flag.Parse()

	var input io.Reader = os.Stdin
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			log.Fatalf("failed to open file: %v", err)
		}
		defer f.Close()
		input = f
	}

	urls, err := readURLs(input)
	if err != nil {
		log.Fatalf("failed to read urls: %v", err)
	}
	if len(urls) == 0 {
		log.Fatal("no urls provided")
	}

	servers := strings.Split(serverList, ",")
	var active []string
	for _, srv := range servers {
		srv = strings.TrimSpace(srv)
		if srv == "" {
			continue
		}
		resp, err := http.Get("http://" + srv + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			active = append(active, srv)
		} else {
			log.Printf("server %s unreachable", srv)
		}
	}
	if len(active) == 0 {
		log.Fatal("no active servers")
	}

	m := len(active)
	per := len(urls) / m
	extra := len(urls) % m
	threadsPer := threadTotal / m
	if threadsPer == 0 {
		threadsPer = 1
	}

	offset := 0
	remaining := len(urls)
	for i, srv := range active {
		count := per
		if i < extra {
			count++
		}
		batch := urls[offset : offset+count]
		offset += count
		reqBody, _ := json.Marshal(checkRequest{URLs: batch, Threads: threadsPer})
		resp, err := http.Post("http://"+srv+"/check", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			log.Printf("server %s failed: %v", srv, err)
			continue
		}
		var cr checkResponse
		if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
			log.Printf("bad response from %s: %v", srv, err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		goods := 0
		bads := 0
		for _, r := range cr.Results {
			if r.Error != "" {
				fmt.Printf("%s: %s\n", r.URL, r.Error)
				bads++
			} else if r.Status != "" && r.Status != "200 OK" {
				fmt.Printf("%s: %s\n", r.URL, r.Status)
				bads++
			} else {
				goods++
			}
		}
		remaining -= count
		fmt.Printf("server %s -> good:%d bad:%d remaining:%d\n", srv, goods, bads, remaining)
	}
}
