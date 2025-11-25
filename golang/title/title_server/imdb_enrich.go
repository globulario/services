// imdb_enrich.go — build/enrich Title & Person from IMDb API + IMDb datasets.
// Put this file in the title service (same folder as title.go, casting.go, etc.)

package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/StalkR/imdb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
)

// imdbIDRE is already defined in your HTTP entrypoint, but we redefine locally
// here to avoid importing that file.
var imdbIDRE = regexp.MustCompile(`^tt\d+$`)

// User-Agent we send to IMDb; needs to look like a real browser.
const imdbUserAgent = "Mozilla/5.0 (compatible; GlobularTitleService/1.0; +https://globular.io)"

// imdbTransport wraps a RoundTripper to inject headers (User-Agent, etc.).
type imdbTransport struct {
	http.RoundTripper
}

func (t *imdbTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", imdbUserAgent)
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}
	return t.RoundTripper.RoundTrip(req)
}

// newIMDBClient creates an HTTP client with a timeout and a custom transport
// that sets appropriate headers for IMDb scraping.
func newIMDBClient(timeout time.Duration) *http.Client {
	// Clone default transport if possible so we don't mutate globals.
	var base http.RoundTripper = http.DefaultTransport
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		base = t.Clone()
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: &imdbTransport{RoundTripper: base},
	}
}

// ---------------------------------------------------------------------------
// BigCache-based helpers for IMDb data
// ---------------------------------------------------------------------------

var (
	imdbCache     *storage_store.BigCache_store
	imdbCacheOnce sync.Once

	imdbNamesIndexOnce   sync.Once
	imdbRatingsIndexOnce sync.Once
)

func getIMDBCache() *storage_store.BigCache_store {
	imdbCacheOnce.Do(func() {
		c := storage_store.NewBigCache_store()
		if err := c.Open(""); err != nil {
			logger.Error("imdb bigcache open failed", "err", err)
			imdbCache = nil
			return
		}
		imdbCache = c
	})
	return imdbCache
}

func imdbCacheGetBytes(key string) ([]byte, bool) {
	cache := getIMDBCache()
	if cache == nil {
		return nil, false
	}
	b, err := cache.GetItem(key)
	if err != nil {
		return nil, false
	}
	return b, true
}

func imdbCacheSetBytes(key string, val []byte) {
	cache := getIMDBCache()
	if cache == nil {
		return
	}
	_ = cache.SetItem(key, val)
}

func imdbCacheGetJSON(key string, dst interface{}) bool {
	b, ok := imdbCacheGetBytes(key)
	if !ok {
		return false
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return false
	}
	return true
}

func imdbCacheSetJSON(key string, val interface{}) {
	data, err := json.Marshal(val)
	if err != nil {
		return
	}
	imdbCacheSetBytes(key, data)
}

// ---------------- Shared IMDb HTML fetch ----------------

// getIMDBTitleHTML fetches and caches the raw HTML for a title page.
func getIMDBTitleHTML(client *http.Client, titleID string) (string, error) {
	key := "html:" + titleID

	if b, ok := imdbCacheGetBytes(key); ok {
		return string(b), nil
	}

	resp, err := client.Get("https://www.imdb.com/title/" + titleID)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "fail to close response body with error: %v\n", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getIMDBTitleHTML: http %d", resp.StatusCode)
	}

	page, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	pageStr := string(page)

	imdbCacheSetBytes(key, page)

	return pageStr, nil
}

// ---------------- IMDb Poster Fetch ----------------

// GetIMDBPoster fetches the poster URL directly from IMDb HTML.
// It now reuses cached HTML and caches the poster URL itself.
func GetIMDBPoster(imdbID string) (string, error) {
	if !imdbIDRE.MatchString(imdbID) {
		return "", fmt.Errorf("GetIMDBPoster: invalid imdb id %q", imdbID)
	}

	key := "poster:" + imdbID
	if b, ok := imdbCacheGetBytes(key); ok {
		return string(b), nil
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &imdbTransport{
			RoundTripper: http.DefaultTransport,
		},
	}

	html, err := getIMDBTitleHTML(client, imdbID)
	if err != nil {
		return "", fmt.Errorf("GetIMDBPoster: fetch error: %w", err)
	}

	// Primary: <meta property="og:image" content="(url)">
	reOgImage := regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`)
	if m := reOgImage.FindStringSubmatch(html); len(m) == 2 {
		imdbCacheSetBytes(key, []byte(m[1]))
		return m[1], nil
	}

	// Secondary fallback: <img ... src="...jpg">
	reImg := regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	if m := reImg.FindStringSubmatch(html); len(m) == 2 {
		// Only accept .jpg or .jpeg
		if strings.Contains(m[1], ".jpg") || strings.Contains(m[1], ".jpeg") {
			imdbCacheSetBytes(key, []byte(m[1]))
			return m[1], nil
		}
	}

	// If no poster is found, just return empty (and cache that fact).
	imdbCacheSetBytes(key, []byte(""))
	return "", nil
}

// ---------------- TV Episode helpers ----------------

type episodeInfo struct {
	Season  int
	Episode int
	Serie   string
}

// getSeasonEpisodeAndSerie returns season, episode, and parent serie IMDb ID.
// It uses cached HTML and BigCache to avoid multiple HTTP calls for the same title.
func getSeasonEpisodeAndSerie(client *http.Client, titleID string) (int, int, string, error) {
	key := "episode:" + titleID

	var cached episodeInfo
	if imdbCacheGetJSON(key, &cached) {
		return cached.Season, cached.Episode, cached.Serie, nil
	}

	pageStr, err := getIMDBTitleHTML(client, titleID)
	if err != nil {
		return -1, -1, "", err
	}

	season := 0
	episode := 0
	serie := ""

	// Example HTML fragment: >S01<!-- -->.<!-- -->E02<
	regexSE := regexp.MustCompile(`>S\d{1,2}<!-- -->\.<!-- -->E\d{1,2}<`)
	matchsSE := regexSE.FindStringSubmatch(pageStr)
	if len(matchsSE) > 0 {
		regexS := regexp.MustCompile(`S\d{1,2}`)
		matchsS := regexS.FindStringSubmatch(matchsSE[0])
		if len(matchsS) > 0 {
			season = Utility.ToInt(matchsS[0][1:])
		}

		regexE := regexp.MustCompile(`E\d{1,2}`)
		matchsE := regexE.FindStringSubmatch(matchsSE[0])
		if len(matchsE) > 0 {
			episode = Utility.ToInt(matchsE[0][1:])
		}
	}

	// Regex to find the series ID in the href attribute
	re := regexp.MustCompile(`data-testid="hero-title-block__series-link".*?href="/title/(tt\d{7,8})/`)
	matches := re.FindStringSubmatch(pageStr)
	if len(matches) > 1 {
		serie = matches[1] // e.g. "tt3032476"
	}

	info := episodeInfo{Season: season, Episode: episode, Serie: serie}
	imdbCacheSetJSON(key, info)

	return season, episode, serie, nil
}

// ---------------- IMDb Title build/enrich ----------------

// buildTitleFromIMDB fetches a title from the IMDb API and completes it with
// data from IMDb datasets (title.crew, title.principals, name.basics, title.ratings).
// It does NOT index the title; callers decide whether to persist it.
func (srv *server) buildTitleFromIMDB(imdbID string) (*titlepb.Title, error) {
	imdbID = strings.TrimSpace(imdbID)
	if imdbID == "" {
		return nil, fmt.Errorf("empty imdb id")
	}
	if !imdbIDRE.MatchString(imdbID) {
		return nil, fmt.Errorf("invalid imdb id %q", imdbID)
	}

	client := newIMDBClient(10 * time.Second)
	it, err := imdb.NewTitle(client, imdbID)
	if err != nil {
		fmt.Println("Error fetching IMDb title:", err)
		return nil, fmt.Errorf("imdb.NewTitle(%q): %w", imdbID, err)
	}

	// Parse rating string -> float32
	var rating float32
	if it.Rating != "" {
		if f, err := strconv.ParseFloat(it.Rating, 32); err == nil {
			rating = float32(f)
		}
	}

	// Base mapping from StalkR/imdb.Title → titlepb.Title
	title := &titlepb.Title{
		ID:            it.ID,
		URL:           it.URL,
		Name:          it.Name,
		Type:          it.Type,
		Year:          int32(it.Year),
		Rating:        rating,
		RatingCount:   int32(it.RatingCount),
		Description:   it.Description,
		Genres:        append([]string(nil), it.Genres...),
		Language:      append([]string(nil), it.Languages...),
		Nationalities: append([]string(nil), it.Nationalities...),
		Duration:      it.Duration,
	}

	// Episode-specific metadata
	if it.Type == "TVEpisode" {
		season, episode, serie, err := getSeasonEpisodeAndSerie(client, imdbID)
		if err != nil {
			logger.Warn("getSeasonEpisodeAndSerie failed", "imdbID", imdbID, "err", err)
		} else {
			title.Season = int32(season)
			title.Episode = int32(episode)
			// NOTE: We store the parent series IMDb ID in Serie, as you expect (e.g. "tt3032476").
			title.Serie = serie
		}
	}

	// Poster: reuse your HTTP helper (same logic) but we store only the URL
	if posterURL, err := GetIMDBPoster(it.ID); err == nil && posterURL != "" {
		title.Poster = &titlepb.Poster{
			URL:        posterURL,
			ContentUrl: "", // will be filled with a thumbnail on CreateTitle
		}
	}

	// Map directors/writers/actors from API first (often already complete)
	for _, d := range it.Directors {
		title.Directors = append(title.Directors, &titlepb.Person{
			ID:       d.ID,
			FullName: d.FullName,
		})
	}
	for _, w := range it.Writers {
		title.Writers = append(title.Writers, &titlepb.Person{
			ID:       w.ID,
			FullName: w.FullName,
		})
	}
	for _, a := range it.Actors {
		title.Actors = append(title.Actors, &titlepb.Person{
			ID:       a.ID,
			FullName: a.FullName,
		})
	}

	// IMDb datasets dir: data/imdb
	datasetsDir := filepath.Join(config.GetDataDir(), "imdb")

	// Fallback rating from IMDb ratings dataset if missing from API
	if title.Rating == 0 || title.RatingCount == 0 {
		if r, c, err := readIMDBRating(datasetsDir, imdbID); err == nil {
			if title.Rating == 0 {
				title.Rating = r
			}
			if title.RatingCount == 0 {
				title.RatingCount = c
			}
		} else {
			logger.Warn("readIMDBRating failed", "imdbID", imdbID, "err", err)
		}
	}

	// Now enrich missing pieces from IMDb datasets
	if err := srv.enrichTitleFromIMDBDatasets(datasetsDir, imdbID, title); err != nil {
		logger.Warn("enrichTitleFromIMDBDatasets failed", "imdbID", imdbID, "err", err)
	}

	return title, nil
}

// enrichTitleFromIMDBDatasets looks in title.crew.tsv.gz + title.principals.tsv.gz
// and name.basics.tsv.gz for any extra people not present in the API result.
func (srv *server) enrichTitleFromIMDBDatasets(dir, imdbID string, title *titlepb.Title) error {
	// 1) Crew: directors + writers
	crew, err := readIMDBCrew(dir, imdbID)
	if err != nil {
		return err
	}
	// 2) Principals (main cast)
	principals, err := readIMDBPrincipals(dir, imdbID)
	if err != nil {
		return err
	}

	// We will lookup all nconst -> (primaryName) via name.basics in one pass
	needed := make(map[string]struct{})

	for _, n := range crew.Directors {
		needed[n] = struct{}{}
	}
	for _, n := range crew.Writers {
		needed[n] = struct{}{}
	}
	for _, p := range principals {
		needed[p.nconst] = struct{}{}
	}

	nameMap, err := readIMDBNames(dir, needed)
	if err != nil {
		return err
	}

	// Helper to test if a person ID is already present
	hasPerson := func(list []*titlepb.Person, id string) bool {
		for _, p := range list {
			if p.ID == id {
				return true
			}
		}
		return false
	}

	// Merge directors
	for _, nconst := range crew.Directors {
		if hasPerson(title.Directors, nconst) {
			continue
		}
		if name, ok := nameMap[nconst]; ok {
			title.Directors = append(title.Directors, &titlepb.Person{
				ID:       nconst,
				FullName: name,
			})
		}
	}

	// Merge writers
	for _, nconst := range crew.Writers {
		if hasPerson(title.Writers, nconst) {
			continue
		}
		if name, ok := nameMap[nconst]; ok {
			title.Writers = append(title.Writers, &titlepb.Person{
				ID:       nconst,
				FullName: name,
			})
		}
	}

	// Merge cast (actors/actresses/self)
	for _, p := range principals {
		if p.category != "actor" && p.category != "actress" && p.category != "self" {
			continue
		}
		if hasPerson(title.Actors, p.nconst) {
			continue
		}
		if name, ok := nameMap[p.nconst]; ok {
			title.Actors = append(title.Actors, &titlepb.Person{
				ID:       p.nconst,
				FullName: name,
			})
		}
	}

	return nil
}

// ---------------- IMDb dataset readers ----------------

// We only read the single row we need for imdbID/nconst from each TSV,
// to avoid indexing the whole dump in memory *except* for the two big ones
// we explicitly index once: name.basics.tsv.gz and title.ratings.tsv.gz.

const imdbDatasetBaseURL = "https://datasets.imdbws.com/"

func ensureIMDBGZ(dir, name string) (string, error) {
	if err := Utility.CreateIfNotExists(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, name)
	if Utility.Exists(dst) {
		return dst, nil
	}

	url := imdbDatasetBaseURL + name
	logger.Info("downloading IMDb dataset", "file", name, "url", url)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return "", err
	}
	out.Close()

	if err := os.Rename(tmp, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func openIMDBGZ(dir, name string) (*gzip.Reader, *os.File, error) {
	path, err := ensureIMDBGZ(dir, name)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	gr, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return gr, f, nil
}

// ---- title.crew.tsv.gz ----

type imdbCrew struct {
	Directors []string
	Writers   []string
}

func readIMDBCrew(dir, imdbID string) (*imdbCrew, error) {
	key := "crew:" + imdbID

	var cached imdbCrew
	if imdbCacheGetJSON(key, &cached) {
		return &cached, nil
	}

	gr, f, err := openIMDBGZ(dir, "title.crew.tsv.gz")
	if err != nil {
		return &imdbCrew{}, nil // soft fail
	}
	defer f.Close()
	defer gr.Close()

	scanner := bufio.NewScanner(gr)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue // header
		}
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 3 {
			continue
		}
		if cols[0] != imdbID {
			continue
		}
		c := &imdbCrew{}
		if cols[1] != `\N` {
			c.Directors = strings.Split(cols[1], ",")
		}
		if cols[2] != `\N` {
			c.Writers = strings.Split(cols[2], ",")
		}

		imdbCacheSetJSON(key, c)
		return c, nil
	}

	empty := &imdbCrew{}
	imdbCacheSetJSON(key, empty)
	return empty, nil
}

// ---- title.principals.tsv.gz ----

type imdbPrincipal struct {
	nconst   string
	category string
}

func readIMDBPrincipals(dir, imdbID string) ([]imdbPrincipal, error) {
	key := "principals:" + imdbID

	var cached []imdbPrincipal
	if imdbCacheGetJSON(key, &cached) {
		return cached, nil
	}

	gr, f, err := openIMDBGZ(dir, "title.principals.tsv.gz")
	if err != nil {
		return nil, nil // soft fail
	}
	defer f.Close()
	defer gr.Close()

	out := make([]imdbPrincipal, 0, 8)
	scanner := bufio.NewScanner(gr)
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue // header
		}
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 4 {
			continue
		}
		if cols[0] != imdbID {
			continue
		}
		out = append(out, imdbPrincipal{
			nconst:   cols[2],
			category: cols[3],
		})
	}

	imdbCacheSetJSON(key, out)
	return out, nil
}

// ---- name.basics.tsv.gz ----

// indexAllIMDBNames builds a full index nconst -> primaryName in BigCache.
// It is called once via ensureIMDBNamesIndex (sync.Once).
func indexAllIMDBNames(dir string) {
	gr, f, err := openIMDBGZ(dir, "name.basics.tsv.gz")
	if err != nil {
		logger.Warn("indexAllIMDBNames open failed", "err", err)
		return
	}
	defer f.Close()
	defer gr.Close()

	scanner := bufio.NewScanner(gr)
	first := true
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue // header
		}
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 2 {
			continue
		}
		nconst := cols[0]
		primaryName := cols[1]

		key := "name:" + nconst
		imdbCacheSetBytes(key, []byte(primaryName))
		count++
	}
	if err := scanner.Err(); err != nil {
		logger.Warn("indexAllIMDBNames scan error", "err", err)
	}
	logger.Info("indexAllIMDBNames done", "count", count)
}

func ensureIMDBNamesIndex(dir string) {
	imdbNamesIndexOnce.Do(func() {
		indexAllIMDBNames(dir)
	})
}

// readIMDBNames loads primaryName for each nconst in "needed" from BigCache.
// The full name.basics index is built once by ensureIMDBNamesIndex.
func readIMDBNames(dir string, needed map[string]struct{}) (map[string]string, error) {
	names := make(map[string]string, len(needed))
	if len(needed) == 0 {
		return names, nil
	}

	ensureIMDBNamesIndex(dir)

	for nconst := range needed {
		key := "name:" + nconst
		if b, ok := imdbCacheGetBytes(key); ok {
			names[nconst] = string(b)
		}
	}

	return names, nil
}

// ---- title.ratings.tsv.gz ----

type ratingInfo struct {
	Rating float32
	Count  int32
}

// indexAllIMDBRatings builds a full index imdbID -> {rating,count} in BigCache.
func indexAllIMDBRatings(dir string) {
	gr, f, err := openIMDBGZ(dir, "title.ratings.tsv.gz")
	if err != nil {
		logger.Warn("indexAllIMDBRatings open failed", "err", err)
		return
	}
	defer f.Close()
	defer gr.Close()

	scanner := bufio.NewScanner(gr)
	first := true
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if first {
			first = false
			continue // header
		}
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 3 {
			continue
		}
		id := cols[0]

		var r float32
		var c int32

		if cols[1] != `\N` {
			if f64, err := strconv.ParseFloat(cols[1], 32); err == nil {
				r = float32(f64)
			}
		}
		if cols[2] != `\N` {
			if n, err := strconv.Atoi(cols[2]); err == nil {
				c = int32(n)
			}
		}

		ri := ratingInfo{Rating: r, Count: c}
		key := "rating:" + id
		imdbCacheSetJSON(key, ri)
		count++
	}

	if err := scanner.Err(); err != nil {
		logger.Warn("indexAllIMDBRatings scan error", "err", err)
	}
	logger.Info("indexAllIMDBRatings done", "count", count)
}

func ensureIMDBRatingsIndex(dir string) {
	imdbRatingsIndexOnce.Do(func() {
		indexAllIMDBRatings(dir)
	})
}

// readIMDBRating loads the rating + vote count from BigCache.
// The full ratings index is built once by ensureIMDBRatingsIndex.
func readIMDBRating(dir, imdbID string) (float32, int32, error) {
	ensureIMDBRatingsIndex(dir)

	key := "rating:" + imdbID
	var cached ratingInfo
	if imdbCacheGetJSON(key, &cached) {
		return cached.Rating, cached.Count, nil
	}

	// Not found in ratings dataset.
	return 0, 0, nil
}

// ---------------- Prewarm helper ----------------

// prewarmIMDBDatasets downloads the IMDb TSVs once in the background so that
// the first user request does not pay that cost, and builds the name+rating
// indexes in BigCache.
func (srv *server) prewarmIMDBDatasets() {
	dir := filepath.Join(config.GetDataDir(), "imdb")
	files := []string{
		"title.crew.tsv.gz",
		"title.principals.tsv.gz",
		"name.basics.tsv.gz",
		"title.ratings.tsv.gz",
	}

	for _, name := range files {
		if _, err := ensureIMDBGZ(dir, name); err != nil {
			logger.Warn("prewarm imdb dataset failed", "file", name, "err", err)
		} else {
			logger.Info("prewarm imdb dataset ok", "file", name)
		}
	}

	// Build heavy indexes in the same background goroutine.
	ensureIMDBNamesIndex(dir)
	ensureIMDBRatingsIndex(dir)
}
