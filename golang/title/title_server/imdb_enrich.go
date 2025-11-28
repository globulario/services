// imdb_enrich.go — build/enrich Title & Person from IMDb API + IMDb datasets.
// Put this file in the title service (same folder as title.go, casting.go, etc.)

package main

import (
	"encoding/base64"
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
	tmdb "github.com/cyruzin/golang-tmdb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
)

// imdbIDRE is already defined in your HTTP entrypoint, but we redefine locally
// here to avoid importing that file.
var (
	imdbIDRE           = regexp.MustCompile(`^tt\d+$`)
	imdbNameIDRE       = regexp.MustCompile(`^nm\d+$`)
	imdbPersonLDJSONRe = regexp.MustCompile(
		`(?s)<script[^>]+type=["']application/ld\+json["'][^>]*>(.*?)</script>`,
	)
)

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

var (
	tmdbClient     *tmdb.Client
	tmdbClientOnce sync.Once
)

func getTMDBClient() *tmdb.Client {
	tmdbClientOnce.Do(func() {
		apiKey := strings.TrimSpace(os.Getenv("TMDB_API_KEY"))
		if apiKey == "" {
			logger.Warn("TMDb enrichment disabled: TMDB_API_KEY env var not set")
			return
		}
		client, err := tmdb.Init(apiKey)
		if err != nil {
			logger.Warn("TMDb init failed", "err", err)
			return
		}
		client.SetClientAutoRetry()
		tmdbClient = client
	})
	return tmdbClient
}

type tmdbTitleInfo struct {
	ID            int
	Kind          string
	ShowID        int
	SeasonNumber  int
	EpisodeNumber int
}

func getTMDBTitleInfo(imdbID string, client *tmdb.Client) (*tmdbTitleInfo, error) {
	if client == nil {
		return nil, fmt.Errorf("tmdb client is nil")
	}
	if !imdbIDRE.MatchString(imdbID) {
		return nil, fmt.Errorf("invalid imdb id %q", imdbID)
	}

	key := "tmdb_title:" + imdbID
	var cached tmdbTitleInfo
	if imdbCacheGetJSON(key, &cached) && cached.ID != 0 && cached.Kind != "" {
		return &cached, nil
	}

	res, err := client.GetFindByID(imdbID, map[string]string{
		"external_source": "imdb_id",
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("tmdb find: empty response for %s", imdbID)
	}

	if len(res.MovieResults) > 0 {
		info := tmdbTitleInfo{ID: int(res.MovieResults[0].ID), Kind: "movie"}
		imdbCacheSetJSON(key, info)
		return &info, nil
	}
	if len(res.TvResults) > 0 {
		info := tmdbTitleInfo{ID: int(res.TvResults[0].ID), Kind: "tv"}
		imdbCacheSetJSON(key, info)
		return &info, nil
	}
	if len(res.TvEpisodeResults) > 0 {
		r := res.TvEpisodeResults[0]
		if r.ShowID != 0 && r.SeasonNumber != 0 && r.EpisodeNumber != 0 {
			info := tmdbTitleInfo{
				ID:            int(r.ShowID),
				Kind:          "tv_episode",
				ShowID:        int(r.ShowID),
				SeasonNumber:  r.SeasonNumber,
				EpisodeNumber: r.EpisodeNumber,
			}
			imdbCacheSetJSON(key, info)
			return &info, nil
		}
	}

	return nil, fmt.Errorf("tmdb find: no movie/tv result for %s", imdbID)
}

func getIMDBNameIDFromTMDBPerson(tmdbPersonID int, client *tmdb.Client) (string, error) {
	if client == nil {
		return "", fmt.Errorf("tmdb client is nil")
	}
	key := "tmdb_person_imdb:" + strconv.Itoa(tmdbPersonID)
	var cached string
	if imdbCacheGetJSON(key, &cached) && cached != "" {
		return cached, nil
	}
	ids, err := client.GetPersonExternalIDs(tmdbPersonID, nil)
	if err != nil {
		return "", err
	}
	if ids == nil || ids.IMDbID == "" {
		return "", fmt.Errorf("tmdb person %d missing imdb id", tmdbPersonID)
	}
	imdbCacheSetJSON(key, ids.IMDbID)
	return ids.IMDbID, nil
}

func getTMDBPersonIDForIMDB(imdbNameID string, client *tmdb.Client) (int, error) {
	if client == nil {
		return 0, fmt.Errorf("tmdb client is nil")
	}
	if !imdbNameIDRE.MatchString(imdbNameID) {
		return 0, fmt.Errorf("invalid imdb name id %q", imdbNameID)
	}

	key := "tmdb_person_id:" + imdbNameID
	var cached int
	if imdbCacheGetJSON(key, &cached) && cached != 0 {
		return cached, nil
	}

	res, err := client.GetFindByID(imdbNameID, map[string]string{
		"external_source": "imdb_id",
	})
	if err != nil {
		return 0, err
	}
	if res == nil || len(res.PersonResults) == 0 {
		return 0, fmt.Errorf("tmdb find: no person results for %s", imdbNameID)
	}
	id := res.PersonResults[0].ID
	imdbCacheSetJSON(key, int(id))
	return int(id), nil
}

type tmdbPersonCacheInfo struct {
	ProfilePath  string   `json:"profile_path"`
	Biography    string   `json:"biography"`
	Birthday     string   `json:"birthday"`
	PlaceOfBirth string   `json:"place_of_birth"`
	Gender       int      `json:"gender"`
	AlsoKnownAs  []string `json:"also_known_as"`
}

func getTMDBPersonCacheInfo(imdbNameID string, client *tmdb.Client) (*tmdbPersonCacheInfo, error) {
	if client == nil {
		return nil, fmt.Errorf("tmdb client is nil")
	}
	key := "tmdb_person_info:" + imdbNameID
	var cached tmdbPersonCacheInfo
	if imdbCacheGetJSON(key, &cached) {
		return &cached, nil
	}

	tmdbID, err := getTMDBPersonIDForIMDB(imdbNameID, client)
	if err != nil {
		return nil, err
	}
	details, derr := client.GetPersonDetails(tmdbID, nil)
	if derr != nil {
		return nil, derr
	}
	if details == nil {
		return nil, fmt.Errorf("tmdb person %d missing details", tmdbID)
	}

	info := &tmdbPersonCacheInfo{
		ProfilePath:  strings.TrimSpace(details.ProfilePath),
		Biography:    strings.TrimSpace(details.Biography),
		Birthday:     strings.TrimSpace(details.Birthday),
		PlaceOfBirth: strings.TrimSpace(details.PlaceOfBirth),
		Gender:       details.Gender,
		AlsoKnownAs:  details.AlsoKnownAs,
	}
	imdbCacheSetJSON(key, info)
	return info, nil
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

	// Enrich with TMDb data when available.
	if err := srv.enrichTitleFromTMDB(title); err != nil {
		logger.Warn("enrichTitleFromTMDB failed", "imdbID", imdbID, "err", err)
	}

	// 1) Enrich persons from IMDb HTML (ld+json)
	if err := srv.enrichPersonsFromIMDB(client, title); err != nil {
		logger.Warn("enrichPersonsFromIMDB failed", "imdbID", imdbID, "err", err)
	}

	// 2) Enrich persons from TMDb (biography, photo, birth date/place, aliases)
	if err := srv.enrichPersonsFromTMDB(title); err != nil {
		logger.Warn("enrichPersonsFromTMDB failed", "imdbID", imdbID, "err", err)
	}

	return title, nil
}

type tmdbCastMember struct {
	ID          int
	Name        string
	ProfilePath string
}

type tmdbCrewMember struct {
	ID          int
	Name        string
	ProfilePath string
	Department  string
	Job         string
}

func (srv *server) enrichTitleFromTMDB(title *titlepb.Title) error {
	if title == nil {
		return nil
	}

	client := getTMDBClient()
	if client == nil {
		return nil
	}

	httpClient := newIMDBClient(15 * time.Second)

	info, err := getTMDBTitleInfo(title.ID, client)
	if err != nil {
		return err
	}
	if info == nil {
		return nil
	}

	switch info.Kind {
	case "movie":
		details, derr := client.GetMovieDetails(info.ID, nil)
		if derr == nil && details != nil {
			applyMovieDetailsToTitle(title, details, httpClient)
		} else if derr != nil {
			logger.Warn("GetMovieDetails failed", "tmdbID", info.ID, "err", derr)
		}
		if credits, cerr := client.GetMovieCredits(info.ID, nil); cerr == nil && credits != nil {
			cast := make([]tmdbCastMember, 0, len(credits.Cast))
			for _, c := range credits.Cast {
				cast = append(cast, tmdbCastMember{
					ID:          int(c.ID),
					Name:        c.Name,
					ProfilePath: c.ProfilePath,
				})
			}
			crew := make([]tmdbCrewMember, 0, len(credits.Crew))
			for _, c := range credits.Crew {
				crew = append(crew, tmdbCrewMember{
					ID:          int(c.ID),
					Name:        c.Name,
					ProfilePath: c.ProfilePath,
					Department:  c.Department,
					Job:         c.Job,
				})
			}
			srv.applyTMDBCredits(title, cast, crew, client)
		}
	case "tv":
		details, derr := client.GetTVDetails(info.ID, nil)
		if derr == nil && details != nil {
			applyTVDetailsToTitle(title, details, httpClient)
		} else if derr != nil {
			logger.Warn("GetTVDetails failed", "tmdbID", info.ID, "err", derr)
		}
		if credits, cerr := client.GetTVCredits(info.ID, nil); cerr == nil && credits != nil {
			cast := make([]tmdbCastMember, 0, len(credits.Cast))
			for _, c := range credits.Cast {
				cast = append(cast, tmdbCastMember{
					ID:          int(c.ID),
					Name:        c.Name,
					ProfilePath: c.ProfilePath,
				})
			}
			crew := make([]tmdbCrewMember, 0, len(credits.Crew))
			for _, c := range credits.Crew {
				crew = append(crew, tmdbCrewMember{
					ID:          int(c.ID),
					Name:        c.Name,
					ProfilePath: c.ProfilePath,
					Department:  c.Department,
					Job:         c.Job,
				})
			}
			srv.applyTMDBCredits(title, cast, crew, client)
		}
	case "tv_episode":
		if info.ShowID != 0 && info.SeasonNumber != 0 && info.EpisodeNumber != 0 {
			details, derr := client.GetTVEpisodeDetails(info.ShowID, info.SeasonNumber, info.EpisodeNumber, nil)
			if derr == nil && details != nil {
				applyTVEpisodeDetailsToTitle(title, details, httpClient)
			} else if derr != nil {
				logger.Warn("GetTVEpisodeDetails failed", "showID", info.ShowID, "season", info.SeasonNumber, "episode", info.EpisodeNumber, "err", derr)
			}

			if credits, cerr := client.GetTVEpisodeCredits(info.ShowID, info.SeasonNumber, info.EpisodeNumber); cerr == nil && credits != nil {
				cast := make([]tmdbCastMember, 0, len(credits.Cast)+len(credits.GuestStars))
				for _, c := range credits.Cast {
					cast = append(cast, tmdbCastMember{
						ID:          int(c.ID),
						Name:        c.Name,
						ProfilePath: c.ProfilePath,
					})
				}
				for _, c := range credits.GuestStars {
					cast = append(cast, tmdbCastMember{
						ID:          int(c.ID),
						Name:        c.Name,
						ProfilePath: c.ProfilePath,
					})
				}
				crew := make([]tmdbCrewMember, 0, len(credits.Crew))
				for _, c := range credits.Crew {
					crew = append(crew, tmdbCrewMember{
						ID:          int(c.ID),
						Name:        c.Name,
						ProfilePath: c.ProfilePath,
						Department:  c.Department,
						Job:         c.Job,
					})
				}
				srv.applyTMDBCredits(title, cast, crew, client)
			}
		}
	}

	return nil
}

func applyMovieDetailsToTitle(title *titlepb.Title, details *tmdb.MovieDetails, httpClient *http.Client) {
	if title == nil || details == nil {
		return
	}
	if title.Rating == 0 && details.VoteAverage != 0 {
		title.Rating = details.VoteAverage
	}
	if title.RatingCount == 0 && details.VoteCount != 0 {
		title.RatingCount = int32(details.VoteCount)
	}
	if title.Description == "" && strings.TrimSpace(details.Overview) != "" {
		title.Description = strings.TrimSpace(details.Overview)
	}
	if len(title.Genres) == 0 && len(details.Genres) > 0 {
		genres := make([]string, 0, len(details.Genres))
		for _, g := range details.Genres {
			if g.Name != "" {
				genres = append(genres, g.Name)
			}
		}
		title.Genres = genres
	}
	if len(title.Language) == 0 && len(details.SpokenLanguages) > 0 {
		langs := make([]string, 0, len(details.SpokenLanguages))
		for _, l := range details.SpokenLanguages {
			if l.Name != "" {
				langs = append(langs, l.Name)
			}
		}
		title.Language = langs
	}
	if len(title.Nationalities) == 0 && len(details.ProductionCountries) > 0 {
		nats := make([]string, 0, len(details.ProductionCountries))
		for _, p := range details.ProductionCountries {
			if p.Name != "" {
				nats = append(nats, p.Name)
			}
		}
		title.Nationalities = nats
	}
	if title.Duration == "" && details.Runtime > 0 {
		title.Duration = fmt.Sprintf("%dm", details.Runtime)
	}
	if details.PosterPath != "" {
		setTitlePosterFromTMDB(title, details.PosterPath, httpClient)
	}
}

func applyTVDetailsToTitle(title *titlepb.Title, details *tmdb.TVDetails, httpClient *http.Client) {
	if title == nil || details == nil {
		return
	}
	if title.Rating == 0 && details.VoteAverage != 0 {
		title.Rating = details.VoteAverage
	}
	if title.RatingCount == 0 && details.VoteCount != 0 {
		title.RatingCount = int32(details.VoteCount)
	}
	if title.Description == "" && strings.TrimSpace(details.Overview) != "" {
		title.Description = strings.TrimSpace(details.Overview)
	}
	if len(title.Genres) == 0 && len(details.Genres) > 0 {
		genres := make([]string, 0, len(details.Genres))
		for _, g := range details.Genres {
			if g.Name != "" {
				genres = append(genres, g.Name)
			}
		}
		title.Genres = genres
	}
	if len(title.Language) == 0 {
		langs := make([]string, 0, len(details.Languages))
		for _, l := range details.Languages {
			if strings.TrimSpace(l) != "" {
				langs = append(langs, l)
			}
		}
		if len(langs) > 0 {
			title.Language = langs
		}
	}
	if len(title.Nationalities) == 0 && len(details.ProductionCountries) > 0 {
		nats := make([]string, 0, len(details.ProductionCountries))
		for _, p := range details.ProductionCountries {
			if p.Name != "" {
				nats = append(nats, p.Name)
			}
		}
		title.Nationalities = nats
	}
	if title.Duration == "" && len(details.EpisodeRunTime) > 0 {
		title.Duration = fmt.Sprintf("%dm", details.EpisodeRunTime[0])
	}
	if details.PosterPath != "" {
		setTitlePosterFromTMDB(title, details.PosterPath, httpClient)
	}
}

func applyTVEpisodeDetailsToTitle(title *titlepb.Title, details *tmdb.TVEpisodeDetails, httpClient *http.Client) {
	if title == nil || details == nil {
		return
	}
	if title.Rating == 0 && details.VoteAverage != 0 {
		title.Rating = details.VoteAverage
	}
	if title.RatingCount == 0 && details.VoteCount != 0 {
		title.RatingCount = int32(details.VoteCount)
	}
	if title.Description == "" && strings.TrimSpace(details.Overview) != "" {
		title.Description = strings.TrimSpace(details.Overview)
	}
	if title.Duration == "" && details.Runtime > 0 {
		title.Duration = fmt.Sprintf("%dm", details.Runtime)
	}
	if details.StillPath != "" {
		setTitlePosterFromTMDB(title, details.StillPath, httpClient)
	}
}

func setTitlePosterFromTMDB(title *titlepb.Title, posterPath string, httpClient *http.Client) {
	if title == nil || posterPath == "" {
		return
	}
	if httpClient == nil {
		httpClient = newIMDBClient(15 * time.Second)
	}
	if title.Poster == nil {
		title.Poster = &titlepb.Poster{}
	}
	title.Poster.URL = tmdb.GetImageURL(posterPath, tmdb.W500)
	if title.Poster.ContentUrl == "" {
		if dataURL := tmdbImageToDataURL(posterPath, httpClient); dataURL != "" {
			title.Poster.ContentUrl = dataURL
		}
	}
}

func (srv *server) applyTMDBCredits(title *titlepb.Title, cast []tmdbCastMember, crew []tmdbCrewMember, client *tmdb.Client) {
	if title == nil || client == nil {
		return
	}
	httpClient := newIMDBClient(15 * time.Second)

	for _, member := range cast {
		id, err := getIMDBNameIDFromTMDBPerson(member.ID, client)
		if err != nil || id == "" {
			continue
		}
		if hasPerson(title.Actors, id) {
			continue
		}
		person := &titlepb.Person{
			ID:       id,
			FullName: member.Name,
			URL:      normalizeIMDBPersonURL("", id),
		}
		if member.ProfilePath != "" {
			if dataURL := tmdbImageToDataURL(member.ProfilePath, httpClient); dataURL != "" {
				person.Picture = dataURL
			}
		}
		title.Actors = append(title.Actors, person)
	}

	for _, member := range crew {
		id, err := getIMDBNameIDFromTMDBPerson(member.ID, client)
		if err != nil || id == "" {
			continue
		}
		person := &titlepb.Person{
			ID:       id,
			FullName: member.Name,
			URL:      normalizeIMDBPersonURL("", id),
		}
		if member.ProfilePath != "" {
			if dataURL := tmdbImageToDataURL(member.ProfilePath, httpClient); dataURL != "" {
				person.Picture = dataURL
			}
		}
		switch strings.ToLower(member.Job) {
		case "director":
			if !hasPerson(title.Directors, id) {
				title.Directors = append(title.Directors, person)
			}
		case "writer", "screenplay", "story", "author":
			if !hasPerson(title.Writers, id) {
				title.Writers = append(title.Writers, person)
			}
		}
	}
}

func tmdbImageToDataURL(path string, client *http.Client) string {
	if path == "" || client == nil {
		return ""
	}
	url := tmdb.GetImageURL(path, tmdb.W500)
	if data, err := getImageDataURL(client, url); err == nil {
		return data
	}
	return ""
}

func hasPerson(list []*titlepb.Person, id string) bool {
	for _, p := range list {
		if p != nil && p.ID == id {
			return true
		}
	}
	return false
}

type imdbPersonInfo struct {
	URL        string   `json:"url"`
	Picture    string   `json:"picture"`
	Biography  string   `json:"biography"`
	BirthDate  string   `json:"birthDate"`
	BirthPlace string   `json:"birthPlace"`
	Gender     string   `json:"gender"`
	Aliases    []string `json:"aliases"`
}

func (srv *server) enrichPersonsFromIMDB(client *http.Client, title *titlepb.Title) error {
	if title == nil || client == nil {
		return nil
	}

	lists := [][]*titlepb.Person{
		title.Directors,
		title.Writers,
		title.Actors,
	}

	seen := make(map[string]*titlepb.Person)
	for _, group := range lists {
		for _, person := range group {
			if person == nil || person.ID == "" {
				continue
			}
			if _, ok := seen[person.ID]; ok {
				continue
			}
			ensureIMDBPersonURL(person, person.ID)
			seen[person.ID] = person
		}
	}

	for id, person := range seen {
		info, err := getIMDBPersonInfo(client, id)
		if err != nil {
			logger.Debug("getIMDBPersonInfo failed", "imdbID", id, "err", err)
			continue
		}
		applyIMDBPersonInfo(person, info)
	}
	return nil
}

func ensureIMDBPersonURL(person *titlepb.Person, id string) {
	if person == nil {
		return
	}
	person.URL = normalizeIMDBPersonURL(person.URL, id)
}

func applyIMDBPersonInfo(person *titlepb.Person, info *imdbPersonInfo) {
	if person == nil || info == nil {
		return
	}
	if info.URL != "" {
		person.URL = normalizeIMDBPersonURL(info.URL, person.ID)
	}
	if person.Picture == "" && info.Picture != "" {
		person.Picture = info.Picture
	}
	if person.Biography == "" && info.Biography != "" {
		person.Biography = info.Biography
	}
	if person.BirthDate == "" && info.BirthDate != "" {
		person.BirthDate = info.BirthDate
	}
	if person.BirthPlace == "" && info.BirthPlace != "" {
		person.BirthPlace = info.BirthPlace
	}
	if person.Gender == "" && info.Gender != "" {
		person.Gender = info.Gender
	}
	person.Aliases = info.Aliases
}

func getIMDBPersonInfo(client *http.Client, personID string) (*imdbPersonInfo, error) {
	if !imdbNameIDRE.MatchString(personID) {
		return nil, fmt.Errorf("getIMDBPersonInfo: invalid imdb name id %q", personID)
	}

	key := "person:" + personID
	var cached imdbPersonInfo
	if imdbCacheGetJSON(key, &cached) {
		return &cached, nil
	}

	html, err := getIMDBPersonHTML(client, personID)
	if err != nil {
		return nil, err
	}

	data, err := parseIMDBPersonLDJSON(html)
	if err != nil {
		return nil, err
	}

	rawPicture := anyToString(data["image"])
	pictureData, picErr := getImageDataURL(client, rawPicture)
	if picErr != nil {
		logger.Debug("getImageDataURL failed", "personID", personID, "err", picErr)
	}
	if pictureData == "" {
		pictureData = rawPicture
	}

	info := &imdbPersonInfo{
		URL:        normalizeIMDBPersonURL(anyToString(data["url"]), personID),
		Picture:    pictureData,
		Biography:  strings.TrimSpace(anyToString(data["description"])),
		BirthDate:  strings.TrimSpace(anyToString(data["birthDate"])),
		BirthPlace: strings.TrimSpace(parseBirthPlace(data["birthPlace"])),
		Gender:     strings.TrimSpace(anyToString(data["gender"])),
		Aliases:    anyToStringSlice(data["alternateName"]),
	}

	imdbCacheSetJSON(key, info)
	return info, nil
}

func getIMDBPersonHTML(client *http.Client, personID string) (string, error) {
	if !imdbNameIDRE.MatchString(personID) {
		return "", fmt.Errorf("getIMDBPersonHTML: invalid imdb name id %q", personID)
	}

	key := "personhtml:" + personID
	if b, ok := imdbCacheGetBytes(key); ok {
		return string(b), nil
	}

	resp, err := client.Get("https://www.imdb.com/name/" + personID + "/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getIMDBPersonHTML: http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	imdbCacheSetBytes(key, body)
	return string(body), nil
}

func parseIMDBPersonLDJSON(html string) (map[string]interface{}, error) {
	matches := imdbPersonLDJSONRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		raw := strings.TrimSpace(match[1])

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &obj); err == nil {
			if typ, ok := obj["@type"].(string); ok && typ == "Person" {
				return obj, nil
			}
			if graph, ok := obj["@graph"].([]interface{}); ok {
				for _, node := range graph {
					if m, ok := node.(map[string]interface{}); ok {
						if typ, ok := m["@type"].(string); ok && typ == "Person" {
							return m, nil
						}
					}
				}
			}
		}

		var arr []interface{}
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			for _, node := range arr {
				if m, ok := node.(map[string]interface{}); ok {
					if typ, ok := m["@type"].(string); ok && typ == "Person" {
						return m, nil
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("parseIMDBPersonLDJSON: person block not found")
}

func getImageDataURL(client *http.Client, imageURL string) (string, error) {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return "", nil
	}
	if strings.HasPrefix(imageURL, "data:") {
		return imageURL, nil
	}

	key := "imagedata:" + imageURL
	if b, ok := imdbCacheGetBytes(key); ok {
		return string(b), nil
	}

	resp, err := client.Get(imageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getImageDataURL: http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	data := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(body)
	imdbCacheSetBytes(key, []byte(data))
	return data, nil
}

func normalizeIMDBPersonURL(raw string, id string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "https://www.imdb.com/name/" + id + "/"
	}
	if strings.HasPrefix(raw, "/") {
		return "https://www.imdb.com" + raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return "https://www.imdb.com/name/" + id + "/"
}

func parseBirthPlace(value interface{}) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	if m, ok := value.(map[string]interface{}); ok {
		if name, ok := m["name"].(string); ok && name != "" {
			return name
		}
		if loc, ok := m["address"].(map[string]interface{}); ok {
			if locality, ok := loc["addressLocality"].(string); ok && locality != "" {
				return locality
			}
		}
	}
	return ""
}

func anyToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if url, ok := v["url"].(string); ok {
			return url
		}
		if name, ok := v["name"].(string); ok {
			return name
		}
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				return str
			}
		}
	}
	return ""
}

func anyToStringSlice(value interface{}) []string {
	switch v := value.(type) {
	case string:
		if v = strings.TrimSpace(v); v != "" {
			return []string{v}
		}
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				str = strings.TrimSpace(str)
				if str != "" {
					out = append(out, str)
				}
			}
		}
		return out
	}
	return nil
}

func (srv *server) enrichPersonsFromTMDB(title *titlepb.Title) error {
	if title == nil {
		return nil
	}

	client := getTMDBClient()
	if client == nil {
		return nil
	}

	lists := [][]*titlepb.Person{
		title.Directors,
		title.Writers,
		title.Actors,
	}

	seen := make(map[string]*titlepb.Person)
	for _, group := range lists {
		for _, person := range group {
			if person == nil || person.ID == "" {
				continue
			}
			if !imdbNameIDRE.MatchString(person.ID) {
				continue
			}
			if _, ok := seen[person.ID]; ok {
				continue
			}
			seen[person.ID] = person
		}
	}

	for imdbNameID, person := range seen {
		if !personNeedsTMDB(person) {
			continue
		}

		info, err := getTMDBPersonCacheInfo(imdbNameID, client)
		if err != nil {
			logger.Debug("TMDb person info failed", "imdbNameID", imdbNameID, "err", err)
			continue
		}

		if person.Picture == "" && info.ProfilePath != "" {
			person.Picture = tmdbImageToDataURL(info.ProfilePath, newIMDBClient(15*time.Second))
		}

		if person.Biography == "" && info.Biography != "" {
			person.Biography = info.Biography
		}

		if person.BirthDate == "" && info.Birthday != "" {
			person.BirthDate = info.Birthday
		}

		if person.BirthPlace == "" && info.PlaceOfBirth != "" {
			person.BirthPlace = info.PlaceOfBirth
		}

		if person.Gender == "" {
			switch info.Gender {
			case 1:
				person.Gender = "Female"
			case 2:
				person.Gender = "Male"
			default:
				person.Gender = ""
			}
		}

		if len(info.AlsoKnownAs) > 0 || len(person.Aliases) > 0 {
			if aliases, _ := mergeUniqueStrings(info.AlsoKnownAs, person.Aliases); aliases != nil {
				person.Aliases = aliases
			}
		}
	}

	return nil
}

func personNeedsTMDB(person *titlepb.Person) bool {
	if person == nil {
		return true
	}
	if strings.TrimSpace(person.Picture) == "" {
		return true
	}
	if strings.TrimSpace(person.Biography) == "" {
		return true
	}
	if strings.TrimSpace(person.BirthDate) == "" {
		return true
	}
	if strings.TrimSpace(person.BirthPlace) == "" {
		return true
	}
	if person.Gender == "" {
		return true
	}
	if len(person.Aliases) == 0 {
		return true
	}
	return false
}

// ---------------- Prewarm helper ----------------

func (srv *server) prewarmIMDBDatasets() {
	dir := filepath.Join(config.GetDataDir(), "imdb")
	if err := Utility.CreateIfNotExists(dir, 0o755); err != nil {
		logger.Warn("ensure imdb data dir failed", "dir", dir, "err", err)
	}
}
