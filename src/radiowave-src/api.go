package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type RadioBrowser struct {
	client  *http.Client
	servers []string
	current string
}

func NewRadioBrowser() *RadioBrowser {
	rb := &RadioBrowser{
		client: &http.Client{Timeout: 10 * time.Second},
		servers: []string{
			"de1.api.radio-browser.info",
			"nl1.api.radio-browser.info",
		},
	}
	rb.discoverMirrors()
	return rb
}

func (r *RadioBrowser) discoverMirrors() {
	ips, err := net.LookupHost("all.api.radio-browser.info")
	if err != nil {
		return
	}
	seen := map[string]bool{}
	var names []string
	for _, ip := range ips {
		ptrs, err := net.LookupAddr(ip)
		if err != nil {
			continue
		}
		for _, ptr := range ptrs {
			name := strings.TrimSuffix(ptr, ".")
			if strings.HasSuffix(name, ".api.radio-browser.info") && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	if len(names) > 0 {
		sort.Strings(names)
		r.servers = names
	}
}

func (r *RadioBrowser) request(path string, q url.Values, out any) error {
	var lastErr error
	for _, server := range r.servers {
		u := url.URL{Scheme: "https", Host: server, Path: path, RawQuery: q.Encode()}
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "radiowave/"+Version)
		req.Header.Set("Accept", "application/json")
		resp, err := r.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			lastErr = fmt.Errorf("radio-browser %s: HTTP %s", server, resp.Status)
			continue
		}
		err = json.NewDecoder(resp.Body).Decode(out)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		r.current = server
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no Radio Browser mirrors available")
	}
	return lastErr
}

func (r *RadioBrowser) Stations(mode browseMode, query string) ([]Station, error) {
	stations, err := r.StationsPage(mode, query, 0, 100)
	if err != nil {
		return nil, err
	}
	return dedupeStations(stations), nil
}

func (r *RadioBrowser) StationsPage(mode browseMode, query string, offset, limit int) ([]Station, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	q := url.Values{}
	q.Set("hidebroken", "true")
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("bitrateMin", "64")

	switch mode {
	case modeTrending:
		q.Set("order", "clicktrend")
		q.Set("reverse", "true")
	case modePopular:
		q.Set("order", "votes")
		q.Set("reverse", "true")
	case modeRandom:
		q.Set("order", "random")
	case modeSearch:
		q.Set("name", query)
		q.Set("order", "votes")
		q.Set("reverse", "true")
	case modeGenre:
		q.Set("tag", query)
		q.Set("tagExact", "true")
		q.Set("order", "votes")
		q.Set("reverse", "true")
	}

	var stations []Station
	if err := r.request("/json/stations/search", q, &stations); err != nil {
		return nil, err
	}
	return stations, nil
}

// Tag is one entry from Radio Browser's tag catalogue, used to drive the
// genre picker without requiring the user to know an exact tag spelling.
type Tag struct {
	Name         string `json:"name"`
	StationCount int    `json:"stationcount"`
}

// Tags fetches the live tag catalogue, most popular first. The API caps
// this at 1000 entries server-side, small enough to fetch once per session
// and filter locally rather than re-querying on every keystroke.
func (r *RadioBrowser) Tags() ([]Tag, error) {
	q := url.Values{}
	q.Set("order", "stationcount")
	q.Set("reverse", "true")
	q.Set("hidebroken", "true")
	var tags []Tag
	if err := r.request("/json/tags", q, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *RadioBrowser) CountClick(uuid string) {
	if uuid == "" {
		return
	}
	var ignored any
	_ = r.request("/json/url/"+url.PathEscape(uuid), nil, &ignored)
}

func dedupeStations(in []Station) []Station {
	seenUUID := map[string]bool{}
	seenURL := map[string]bool{}
	out := make([]Station, 0, len(in))
	for _, s := range in {
		if s.StationUUID == "" || s.URLResolved == "" || s.Name == "" {
			continue
		}
		if seenUUID[s.StationUUID] || seenURL[s.URLResolved] {
			continue
		}
		seenUUID[s.StationUUID] = true
		seenURL[s.URLResolved] = true
		out = append(out, s)
	}
	return out
}
