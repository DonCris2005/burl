package checker

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Result struct {
	URL    string `json:"url"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

func CheckURLs(urls []string, concurrency int) []Result {
	if concurrency < 1 {
		concurrency = 1
	}
	jobs := make(chan string)
	res := make(chan Result)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for u := range jobs {
				res <- checkURL(u)
			}
		}()
	}

	go func() {
		for _, u := range urls {
			jobs <- u
		}
		close(jobs)
		wg.Wait()
		close(res)
	}()

	var results []Result
	for r := range res {
		results = append(results, r)
	}
	return results
}

func checkURL(raw string) Result {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return Result{URL: raw, Error: "invalid url"}
	}

	if !resolves(u) {
		return Result{URL: u.String(), Error: "does not resolve"}
	}

	resp, err := fetchURL(u)
	if err != nil {
		return Result{URL: u.String(), Error: fmt.Sprintf("failed to fetch: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return Result{URL: u.String(), Status: resp.Status, Error: "non-200 response"}
	}

	return Result{URL: u.String(), Status: resp.Status}
}

func resolves(u *url.URL) bool {
	addrs, _ := net.LookupHost(u.Hostname())
	return len(addrs) != 0
}

func fetchURL(u *url.URL) (*http.Response, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{
		Transport: tr,
		Timeout:   5 * time.Second,
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Close = true
	req.Header.Set("User-Agent", "burl/0.1")

	resp, err := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	if err != nil {
		return nil, err
	}

	return resp, err
}
