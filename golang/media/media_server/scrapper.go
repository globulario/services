package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	colly "github.com/gocolly/colly/v2"
)

// -----------------------------------------------------------------------------
// Constants & helpers
// -----------------------------------------------------------------------------

const (
	ytDlpBinary             = "yt-dlp"
	ytDlpTimeout            = 30 * time.Second
	thumbWidth              = 300
	thumbHeight             = 180
	thumbDataURLFilename    = "data_url.txt"
	hiddenDirName           = ".hidden"
	thumbnailLeafFolderName = "__thumbnail__"
	noEmbedEndpointTemplate = "https://noembed.com/embed?url="
)

// buildThumbnailDir returns the hidden thumbnail directory for a given video file path.
// e.g. /path/movie.mp4 -> /path/.hidden/movie/__thumbnail__
func buildThumbnailDir(videoPath string) (string, string) {
	dir := filepath.Dir(videoPath)
	base := filepath.Base(videoPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(dir, hiddenDirName, name, thumbnailLeafFolderName), name
}

var flashvarsPattern = regexp.MustCompile(`(?s)var\s+flashvars_[^=]+=\s*(\{.*?\});`)

func appendUnique(list *[]string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, existing := range *list {
		if strings.EqualFold(existing, value) {
			return
		}
	}
	*list = append(*list, value)
}

// viewCountFromSuffix parses simple K/M suffixed counts to an int64 (best-effort).
func viewCountFromSuffix(s string) int64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	switch {
	case strings.HasSuffix(s, "K"):
		return int64(Utility.ToNumeric(strings.TrimSuffix(s, "K"))) * 1000
	case strings.HasSuffix(s, "M"):
		return int64(Utility.ToNumeric(strings.TrimSuffix(s, "M"))) * 1_000_000
	default:
		return int64(Utility.ToNumeric(s))
	}
}

// runYtDlpThumbnail calls yt-dlp to fetch a thumbnail into the given directory.
func runYtDlpThumbnail(ctx context.Context, dir, videoID, videoURL string) error {
	if _, err := exec.LookPath(ytDlpBinary); err != nil {
		return errors.New("yt-dlp not found in PATH")
	}
	cmd := exec.CommandContext(ctx, ytDlpBinary, videoURL, "-o", videoID, "--write-thumbnail", "--skip-download")
	cmd.Dir = dir
	// optional: capture stderr to help debug
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("yt-dlp thumbnail fetch failed", "err", err, "stderr", string(out))
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Thumbnails
// -----------------------------------------------------------------------------

// downloadThumbnail fetches/creates a video thumbnail and returns a data URL string.
// It caches the generated data URL under the hidden thumbnail directory.
//   - video_id:   unique ID for the video (used as output name for yt-dlp)
//   - video_url:  source URL
//   - video_path: local path to the downloaded video file (used to build cache dir)
func (srv *server) downloadThumbnail(video_id, video_url, video_path string) (string, error) {
	// Validation
	if len(video_id) == 0 {
		return "", errors.New("no video id was given")
	}
	if len(video_url) == 0 {
		return "", errors.New("no video url was given")
	}
	if len(video_path) == 0 {
		return "", errors.New("no video path was given")
	}

	thumbDir, _ := buildThumbnailDir(video_path)
	cachePath := filepath.Join(thumbDir, thumbDataURLFilename)

	// Return cached data URL if present
	if srv.pathExists(cachePath) {
		b, err := os.ReadFile(cachePath)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	// Ensure folder exists
	if err := Utility.CreateDirIfNotExist(thumbDir); err != nil {
		return "", err
	}

	// If directory is empty, try to fetch a thumbnail via yt-dlp
	files, err := Utility.ReadDir(thumbDir)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), ytDlpTimeout)
		defer cancel()

		slog.Info("Fetching thumbnail with yt-dlp", "url", video_url, "dir", thumbDir, "video_id", video_id)
		if err := runYtDlpThumbnail(ctx, thumbDir, video_id, video_url); err != nil {
			return "", err
		}
		files, err = Utility.ReadDir(thumbDir)
		if err != nil {
			return "", err
		}
	}

	if len(files) == 0 {
		return "", errors.New("no thumbnail found for url " + video_url)
	}

	// Pick first file produced by yt-dlp and turn it into a data URL thumbnail
	src := filepath.Join(thumbDir, files[0].Name())
	dataURL, err := Utility.CreateThumbnail(src, thumbWidth, thumbHeight)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(cachePath, []byte(dataURL), 0o664); err != nil {
		return "", err
	}
	return dataURL, nil
}

// -----------------------------------------------------------------------------
// Indexers – Pornhub
// -----------------------------------------------------------------------------

// indexPornhubVideo extracts metadata for a Pornhub video and returns a titlepb.Video.
// token/index_path/video_path are preserved for compatibility with the wider system.
func (srv *server) indexPornhubVideo(token, id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting:  make([]*titlepb.Person, 0),
		Genres:   []string{"adult"},
		Tags:     []string{},
		Duration: int32(srv.getVideoDuration(file_path)),
		URL:      video_url,
		ID:       id,
		Poster:   &titlepb.Poster{ID: id + "-thumnail"}, // keep original ID spelling for compatibility
	}

	var err error
	currentVideo.Poster.ContentUrl, err = srv.downloadThumbnail(currentVideo.ID, video_url, file_path)
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.TitleId = currentVideo.ID

	c := colly.NewCollector(colly.AllowedDomains("pornhub.com", "www.pornhub.com"))

	c.OnResponse(func(r *colly.Response) {
		matches := flashvarsPattern.FindStringSubmatch(string(r.Body))
		if len(matches) < 2 {
			return
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(matches[1]), &payload); err != nil {
			slog.Warn("pornhub flashvars parse failed", "err", err)
			return
		}
		if titleRaw, ok := payload["video_title"].(string); ok {
			title := strings.TrimSpace(html.UnescapeString(titleRaw))
			if title != "" {
				currentVideo.Title = title
				if currentVideo.Description == "" {
					currentVideo.Description = title
				}
			}
		}
		if duration, ok := payload["video_duration"].(float64); ok && duration > 0 {
			currentVideo.Duration = int32(duration)
		}
		if img, ok := payload["image_url"].(string); ok {
			if currentVideo.Poster != nil && currentVideo.Poster.URL == "" {
				currentVideo.Poster.URL = img
			}
		}
	})

	c.OnHTML(".title-container .inlineFree", func(e *colly.HTMLElement) {
		text := strings.TrimSpace(e.Text)
		if text == "" {
			return
		}
		currentVideo.Title = text
		if currentVideo.Description == "" {
			currentVideo.Description = text
		}
	})

	c.OnHTML("meta[name=description]", func(e *colly.HTMLElement) {
		if currentVideo.Description == "" {
			currentVideo.Description = strings.TrimSpace(e.Attr("content"))
		}
	})

	c.OnHTML(".pstar-list-btn", func(e *colly.HTMLElement) {
		p := &titlepb.Person{
			ID:       e.Attr("data-id"),
			FullName: strings.TrimSpace(e.Text),
			URL:      "https://www.pornhub.com" + e.Attr("href"),
		}
		if err := IndexPersonInformation(p); err != nil {
			slog.Warn("IndexPersonInformation failed", "person", p.FullName, "err", err)
		}
		if len(p.ID) > 0 {
			currentVideo.Casting = append(currentVideo.Casting, p)
		}
	})

	c.OnHTML(".video-info-row .userInfo a", func(e *colly.HTMLElement) {
		name := strings.TrimSpace(e.Text)
		if name == "" {
			return
		}
		url := e.Attr("href")
		if !strings.HasPrefix(url, "http") {
			url = "https://www.pornhub.com" + url
		}
		currentVideo.PublisherID = &titlepb.Publisher{
			ID:   name,
			Name: name,
			URL:  url,
		}
	})

	c.OnHTML(".count", func(e *colly.HTMLElement) {
		currentVideo.Count = viewCountFromSuffix(e.Text)
	})

	c.OnHTML(".votesUp", func(e *colly.HTMLElement) {
		likes := strings.TrimSpace(e.Attr("data-rating"))
		if likes == "" {
			likes = strings.TrimSpace(e.Text)
		}
		if likes != "" {
			currentVideo.Likes = int64(Utility.ToNumeric(likes))
		}
	})

	c.OnHTML(".video-info-row .videoInfo", func(e *colly.HTMLElement) {
		if currentVideo.Date == "" {
			currentVideo.Date = strings.TrimSpace(e.Text)
		}
	})

	c.OnHTML(".categoriesWrapper a", func(e *colly.HTMLElement) {
		val := strings.TrimSpace(e.Text)
		if val == "" || val == "Suggest" {
			return
		}
		appendUnique(&currentVideo.Genres, val)
	})

	c.OnHTML(".tagsWrapper a", func(e *colly.HTMLElement) {
		val := strings.TrimSpace(e.Text)
		if val == "" {
			return
		}
		appendUnique(&currentVideo.Tags, val)
	})

	if err := c.Visit(video_url); err != nil {
		return nil, err
	}
	if currentVideo.Title == "" {
		currentVideo.Title = currentVideo.Description
	}
	if currentVideo.Title == "" {
		currentVideo.Title = filepath.Base(file_path)
	}
	return currentVideo, nil
}

// -----------------------------------------------------------------------------
// Indexers – Common porn-person info
// -----------------------------------------------------------------------------

// IndexPersonInformation enriches a Person (name/aliases/picture/biography/etc)
// by attempting lookups across known sources. It tries multiple ID variants
// derived from the person's full name and, as a last resort, the provided Person.ID.
// On success, the Person is modified in place with fields like Biography, Picture,
// BirthDate, BirthPlace, CareerStatus, Aliases, URL, Gender, etc.
func IndexPersonInformation(p *titlepb.Person) error {
	if p == nil {
		return errors.New("person is nil")
	}
	// attempt 1: FullName with hyphens
	if err := _indexPersonInformation_(p, strings.ReplaceAll(p.FullName, " ", "-")); err == nil {
		return nil
	}

	// attempt 2: split full name variants
	values := strings.Split(p.FullName, " ")
	switch len(values) {
	case 1:
		if err := _indexPersonInformation_(p, values[0]); err == nil {
			return nil
		}
	case 2:
		if err := _indexPersonInformation_(p, values[1]); err == nil {
			return nil
		}
	}

	// attempt 3: use existing ID
	return _indexPersonInformation_(p, p.ID)
}

// _indexPersonInformation_ enriches data from freeones.com/<id>/bio (best-effort).
func _indexPersonInformation_(p *titlepb.Person, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("empty id")
	}

	c := colly.NewCollector(colly.AllowedDomains("www.freeones.com", "freeones.com"))

	// biography
	c.OnHTML(`#description > div > div.common-text`, func(e *colly.HTMLElement) {
		if html, err := e.DOM.Html(); err == nil {
			p.Biography = html
		}
	})

	// profile image
	c.OnHTML(`body div.dashboard-image-container > a > img`, func(e *colly.HTMLElement) {
		p.Picture = strings.TrimSpace(e.Attr("src"))
	})

	// birthdate
	c.OnHTML(`#search-result > section > form > div:nth-child(4) > ul > li:nth-child(5) > span.font-size-xs > a > span`, func(e *colly.HTMLElement) {
		p.BirthDate = strings.TrimSpace(e.Text)
	})

	// birthplace
	c.OnHTML(`#search-result > section > form > div:nth-child(4) > ul > li:nth-child(13) > span.font-size-xs`, func(e *colly.HTMLElement) {
		if html, err := e.DOM.Html(); err == nil {
			p.BirthPlace = html
		}
	})

	// career status
	c.OnHTML(`#search-result > section > form > div:nth-child(4) > ul > li:nth-child(9) > span.font-size-xs > a > span`, func(e *colly.HTMLElement) {
		p.CareerStatus = strings.TrimSpace(e.Text)
	})

	// aliases
	p.Aliases = make([]string, 0)
	c.OnHTML(`#search-result > section > form > div:nth-child(4) > ul > li:nth-child(2) > span.font-size-xs`, func(e *colly.HTMLElement) {
		e.ForEach(".text-underline-always", func(_ int, ch *colly.HTMLElement) {
			p.Aliases = append(p.Aliases, strings.TrimSpace(ch.Text))
		})
	})

	url := "https://www.freeones.com/" + id + "/bio"
	if err := c.Visit(url); err != nil {
		return err
	}

	// Set canonical fields if visit succeeded.
	p.ID = Utility.GenerateUUID(url)
	p.URL = url
	if p.Gender == "" {
		p.Gender = "female" // legacy default per original code
	}
	return nil
}

// -----------------------------------------------------------------------------
// Indexers – XHamster
// -----------------------------------------------------------------------------
func (srv *server) indexXhamsterVideo(token, videoID, videoURL, indexPath, videoPath, filePath string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting:  make([]*titlepb.Person, 0),
		Genres:   []string{"adult"},
		Tags:     []string{},
		URL:      videoURL,
		ID:       videoID,
		Duration: int32(srv.getVideoDuration(filePath)),
		Poster:   &titlepb.Poster{ID: videoID + "-thumbnail"},
	}

	fmt.Println("Indexing xhamster video:", videoURL)

	// Thumbnail
	contentURL, err := srv.downloadThumbnail(currentVideo.ID, videoURL, filePath)
	if err != nil {
		return nil, fmt.Errorf("download thumbnail: %w", err)
	}
	currentVideo.Poster.ContentUrl = contentURL
	currentVideo.Poster.URL = videoURL
	currentVideo.Poster.TitleId = currentVideo.ID

	// Colly collector
	c := colly.NewCollector(
		colly.AllowedDomains("www.xhamster.com", "xhamster.com", "fr.xhamster.com"),
	)
	c.SetRequestTimeout(30 * time.Second)

	// --- SELECTORS FOR NEW LAYOUT -----------------------------------------

	const (
		// <div data-role="video-title"><h1>...</h1>...</div>
		selectorTitle = `div[data-role="video-title"] h1`

		// Views + rating are inside:
		// <div data-role="video-title"> ... <p class="icons-a993a"> ... </p> ... </div>
		selectorStats = `div[data-role="video-title"] p.icons-a993a`

		// All tags / channel / pornstars / categories are inside:
		// <nav id="video-tags-list-container"> ... <a href="..."> ... </a> ...
		selectorMetaNavLinks = `nav#video-tags-list-container a`
	)

	// --- Title / description ----------------------------------------------

	c.OnHTML(selectorTitle, func(e *colly.HTMLElement) {
		text := strings.TrimSpace(e.Text)
		if text == "" {
			return
		}
		// Old code was using that as Description; keep same behavior.
		currentVideo.Description = text

		// If your proto has a Title field, you can also set it here:
		// currentVideo.Title = text
	})

	// --- Views & rating (from icons / aria-labels) ------------------------

	c.OnHTML(selectorStats, func(e *colly.HTMLElement) {
		// First, parse aria-label from <i> icons:
		e.ForEach("i", func(_ int, child *colly.HTMLElement) {
			label := strings.TrimSpace(child.Attr("aria-label"))
			if label == "" {
				return
			}

			// Example: "22812 views"
			if strings.Contains(strings.ToLower(label), "views") {
				fields := strings.Fields(label)
				if len(fields) > 0 {
					n := Utility.ToNumeric(strings.ReplaceAll(fields[0], ",", ""))
					if n > 0 {
						// Adjust type to whatever Count is in your proto.
						currentVideo.Count = int64(n)
					}
				}
				return
			}

			// Example: "100% likes"
			if strings.Contains(strings.ToLower(label), "likes") && strings.Contains(label, "%") {
				parts := strings.SplitN(label, "%", 2)
				if len(parts) > 0 {
					p := strings.TrimSpace(parts[0])
					if p != "" {
						percent := Utility.ToNumeric(p)
						currentVideo.Rating = float32(percent / 10.0) // keep your previous /10 convention
					}
				}
			}
		})

		// Fallback: parse the <span> numbers if for some reason aria-labels change.
		e.ForEach("span", func(_ int, child *colly.HTMLElement) {
			txt := strings.TrimSpace(child.Text)
			if txt == "" {
				return
			}

			// If it contains %, treat as rating
			if strings.Contains(txt, "%") {
				p := strings.TrimSpace(strings.ReplaceAll(txt, "%", ""))
				if p != "" {
					percent := Utility.ToNumeric(p)
					if percent > 0 {
						currentVideo.Rating = float32(percent / 10.0)
					}
				}
				return
			}

			// Otherwise maybe it's the views ("22,812")
			if strings.IndexFunc(txt, func(r rune) bool { return r >= '0' && r <= '9' }) != -1 {
				n := Utility.ToNumeric(strings.ReplaceAll(txt, ",", ""))
				if n > 0 && currentVideo.Count == 0 {
					currentVideo.Count = int64(n)
				}
			}
		})
	})

	// --- Pornstars / categories / channels / tags -------------------------

	c.OnHTML(selectorMetaNavLinks, func(e *colly.HTMLElement) {
		href := e.Attr("href")
		if href == "" {
			return
		}

		txt := strings.TrimSpace(e.Text)
		if txt == "" {
			return
		}

		switch {
		// Pornstars
		case strings.Contains(href, "/pornstars/"):
			p := &titlepb.Person{
				URL:      href,
				ID:       txt,
				FullName: txt,
			}
			if err := IndexPersonInformation(p); err != nil {
				slog.Warn("IndexPersonInformation failed", "person", p.FullName, "err", err)
			}
			if p.ID != "" {
				currentVideo.Casting = append(currentVideo.Casting, p)
			}

		// Channel / publisher (first "channel" entry will usually be the main one)
		case strings.Contains(href, "/channels/"):
			// Only set if not already set, so we prefer the first channel tag
			if currentVideo.PublisherID == nil {
				currentVideo.PublisherID = &titlepb.Publisher{
					URL:  href,
					ID:   txt,
					Name: txt,
				}
			}

		// Categories (e.g. /categories/big-cock)
		case strings.Contains(href, "/categories/"):
			if len(txt) > 1 {
				currentVideo.Tags = append(currentVideo.Tags, txt)
			}

		// Tags (e.g. /tags/fucked)
		case strings.Contains(href, "/tags/"):
			if len(txt) > 1 {
				currentVideo.Tags = append(currentVideo.Tags, txt)
			}
		}
	})

	// Optional debugging hooks
	c.OnScraped(func(r *colly.Response) {
		if currentVideo.Description == "" {
			slog.Warn("xhamster: description/title not found", "url", videoURL)
		}
		if len(currentVideo.Casting) == 0 {
			slog.Debug("xhamster: no casting found", "url", videoURL)
		}
		if len(currentVideo.Tags) == 0 {
			slog.Debug("xhamster: no tags/tags found", "url", videoURL)
		}
		if currentVideo.PublisherID == nil {
			slog.Debug("xhamster: no publisher found", "url", videoURL)
		}
	})

	if err := c.Visit(videoURL); err != nil {
		return nil, err
	}

	return currentVideo, nil
}

// -----------------------------------------------------------------------------
// Indexers – XNXX
// -----------------------------------------------------------------------------

func (srv *server) indexXnxxVideo(token, videoID, videoURL, indexPath, videoPath, filePath string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting:  make([]*titlepb.Person, 0),
		Genres:   []string{"adult"},
		Tags:     []string{},
		URL:      videoURL,
		Duration: int32(srv.getVideoDuration(filePath)),
		ID:       videoID,
		Poster:   &titlepb.Poster{ID: videoID + "-thumbnail"},
	}

	var err error
	currentVideo.Poster.ContentUrl, err = srv.downloadThumbnail(currentVideo.ID, videoURL, filePath)
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = videoURL
	currentVideo.Poster.TitleId = currentVideo.ID

	c := colly.NewCollector(colly.AllowedDomains("www.xnxx.com", "xnxx.com"))

	// Title + duration / resolution / views
	c.OnHTML(".clear-infobar", func(e *colly.HTMLElement) {
		// Title: <strong>...</strong> inside .video-title
		titleSel := e.DOM.Find(".video-title strong")
		title := strings.TrimSpace(titleSel.Text())
		if title != "" {
			currentVideo.Description = title
		}

		// Metadata: "7min - 720p - 677,572"
		e.ForEach(".metadata", func(_ int, ch *colly.HTMLElement) {
			metaText := strings.TrimSpace(ch.Text)
			if metaText == "" {
				return
			}

			parts := strings.Split(metaText, "-")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}

			// parts[0] = "7min"
			if len(parts) > 0 {
				// optionally parse duration if you want:
				// e.g. "7min" -> 7 * 60
				// but you already have local duration from file, so we can ignore or use as fallback
			}

			// parts[1] = "720p" -> treat as quality tag
			if len(parts) > 1 {
				if quality := parts[1]; quality != "" {
					currentVideo.Tags = append(currentVideo.Tags, quality)
				}
			}

			// parts[2] = "677,572" (maybe plus icon text)
			if len(parts) > 2 {
				viewsPart := parts[2]
				fields := strings.Fields(viewsPart)
				if len(fields) > 0 {
					currentVideo.Count = viewCountFromSuffix(fields[0])
				}
			}
		})
	})

	// Extra description block if present
	c.OnHTML(".metadata-row.video-description", func(e *colly.HTMLElement) {
		txt := strings.TrimSpace(e.Text)
		if txt == "" {
			return
		}
		if len(currentVideo.Description) > 0 {
			currentVideo.Description += "</br>"
		}
		currentVideo.Description += txt
	})

	// Tags + pornstars
	c.OnHTML("#video-content-metadata .metadata-row.video-tags a", func(e *colly.HTMLElement) {
		classAttr := e.Attr("class")

		// pornstar / model links
		if strings.Contains(classAttr, "is-pornstar") {
			name := strings.TrimSpace(e.Text)
			if name == "" {
				return
			}
			p := &titlepb.Person{
				URL:      "https://www.xnxx.com" + e.Attr("href"),
				ID:       name,
				FullName: name,
			}
			if err := IndexPersonInformation(p); err != nil {
				slog.Warn("IndexPersonInformation failed", "person", p.FullName, "err", err)
			}
			currentVideo.Casting = append(currentVideo.Casting, p)
			return
		}

		// regular tags
		tag := strings.TrimSpace(e.Text)
		if len(tag) > 0 {
			currentVideo.Tags = append(currentVideo.Tags, tag)
		}
	})

	// Rating from explicit percentage: <span class="rating-box value">99.01%</span>
	c.OnHTML("#video-votes .rating-box.value", func(e *colly.HTMLElement) {
		percentStr := strings.TrimSpace(e.Text) // "99.01%"
		percentStr = strings.TrimSuffix(percentStr, "%")
		if percentStr == "" {
			return
		}
		r := Utility.ToNumeric(percentStr) // 99.01
		currentVideo.Rating = float32(r / 10.0)
	})

	// Fallback rating from up/down votes if needed
	c.OnHTML("#video-votes .vote-actions", func(e *colly.HTMLElement) {
		// only compute if we didn't get rating above
		if currentVideo.Rating > 0 {
			return
		}

		var like, unlike float32

		e.ForEach(".vote-action-good .value", func(_ int, ch *colly.HTMLElement) {
			like = float32(Utility.ToNumeric(strings.ReplaceAll(strings.TrimSpace(ch.Text), ",", "")))
		})
		e.ForEach(".vote-action-bad .value", func(_ int, ch *colly.HTMLElement) {
			unlike = float32(Utility.ToNumeric(strings.ReplaceAll(strings.TrimSpace(ch.Text), ",", "")))
		})

		if like+unlike > 0 {
			currentVideo.Rating = like / (like + unlike) * 10.0
		}
	})

	if err := c.Visit(videoURL); err != nil {
		return nil, err
	}
	return currentVideo, nil
}

// -----------------------------------------------------------------------------
// Indexers – XVideos
// -----------------------------------------------------------------------------

func (srv *server) indexXvideosVideo(token, videoID, videoURL, indexPath, videoPath, filePath string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting:  make([]*titlepb.Person, 0),
		Genres:   []string{"adult"},
		Tags:     []string{},
		URL:      videoURL,
		ID:       videoID,
		Poster:   &titlepb.Poster{ID: videoID + "-thumbnail"},
		Duration: int32(srv.getVideoDuration(filePath)), // keep your local duration
	}

	var err error
	currentVideo.Poster.ContentUrl, err = srv.downloadThumbnail(currentVideo.ID, videoURL, filePath)
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = videoURL
	currentVideo.Poster.TitleId = currentVideo.ID

	c := colly.NewCollector(
		colly.AllowedDomains("www.xvideos.com", "xvideos.com"),
	)

	// Title + HD mark
	c.OnHTML("h2.page-title", func(e *colly.HTMLElement) {
		// Remove duration / hd spans from the title text
		titleSel := e.DOM.Clone()
		titleSel.Find("span").Remove()
		title := strings.TrimSpace(titleSel.Text())

		// Store it as description (or Title if you have that field)
		currentVideo.Description = title

		// e.g. <span class="video-hd-mark">1440p</span>
		e.ForEach("span.video-hd-mark", func(_ int, ch *colly.HTMLElement) {
			if tag := strings.TrimSpace(ch.Text); tag != "" {
				currentVideo.Tags = append(currentVideo.Tags, tag)
			}
		})
	})

	// Casting models
	c.OnHTML("a.label.profile", func(e *colly.HTMLElement) {
		p := &titlepb.Person{
			URL: "https://www.xvideos.com" + e.Attr("href"),
		}
		e.ForEach("span.name", func(_ int, ch *colly.HTMLElement) {
			name := strings.TrimSpace(ch.Text)
			if name == "" {
				return
			}
			p.ID = name
			p.FullName = name
			if err := IndexPersonInformation(p); err != nil {
				slog.Warn("IndexPersonInformation failed", "person", p.FullName, "err", err)
			}
		})
		if p.ID != "" {
			currentVideo.Casting = append(currentVideo.Casting, p)
		}
	})

	// Uploader / publisher
	c.OnHTML("a.uploader-tag", func(e *colly.HTMLElement) {
		pub := &titlepb.Publisher{
			URL: "https://www.xvideos.com" + e.Attr("href"),
		}
		e.ForEach("span.name", func(_ int, ch *colly.HTMLElement) {
			name := strings.TrimSpace(ch.Text)
			if name == "" {
				return
			}
			pub.ID = name
			pub.Name = name
		})
		currentVideo.PublisherID = pub
	})

	c.OnHTML(".video-metadata.video-tags-list.ordered-label-list.cropped a.is-keyword", func(e *colly.HTMLElement) {
		tag := strings.TrimSpace(e.Text)
		if tag == "" {
			return
		}
		currentVideo.Tags = append(currentVideo.Tags, tag)
	})

	// Views – use #v-views, not rate-infos/votes
	c.OnHTML("#v-views strong.mobile-hide", func(e *colly.HTMLElement) {
		text := strings.TrimSpace(e.Text) // e.g. "367,157"
		if text == "" {
			return
		}
		currentVideo.Count = viewCountFromSuffix(text)
	})

	// Rating % (e.g. 100.0%)
	c.OnHTML("#v-actions-left .rating-good-perc", func(e *colly.HTMLElement) {
		percent := strings.TrimSpace(strings.TrimSuffix(e.Text, "%"))
		if percent == "" {
			return
		}
		// Utility.ToNumeric("100.0") -> 100.0, /10 -> 10.0 (rating over 10)
		currentVideo.Rating = float32(Utility.ToNumeric(percent) / 10.0)
	})

	// Tags list
	c.OnHTML(".video-metadata.video-tags-list.ordered-label-list.cropped a.is-keyword", func(e *colly.HTMLElement) {
		// Only /tags/... links
		if !strings.HasPrefix(e.Attr("href"), "/tags/") {
			return
		}
		tag := strings.TrimSpace(e.Text)
		if tag != "" {
			currentVideo.Tags = append(currentVideo.Tags, tag)
		}
	})

	if err := c.Visit(videoURL); err != nil {
		return nil, err
	}

	return currentVideo, nil
}

// -----------------------------------------------------------------------------
// Indexers – YouTube
// -----------------------------------------------------------------------------

func (srv *server) indexYoutubeVideo(token, video_id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting: make([]*titlepb.Person, 0),
		Genres:  []string{"youtube"},
		Tags:    []string{},
		URL:     video_url,
		ID:      video_id,
		Poster:  &titlepb.Poster{ID: video_id + "-thumnail"},
	}

	var err error
	currentVideo.Poster.ContentUrl, err = srv.downloadThumbnail(currentVideo.ID, video_url, file_path)
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = video_url
	currentVideo.Poster.TitleId = currentVideo.ID

	// Use noembed (best-effort) to fetch channel/title
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(noEmbedEndpointTemplate + video_url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	target := make(map[string]interface{})
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		return nil, err
	}

	currentVideo.PublisherID = &titlepb.Publisher{}
	if v := target["author_url"]; v != nil {
		currentVideo.PublisherID.URL, _ = v.(string)
		if name, ok := target["author_name"].(string); ok {
			currentVideo.PublisherID.Name = name
		}
		if title, ok := target["title"].(string); ok {
			currentVideo.Description = title
		}

		url := currentVideo.PublisherID.URL
		switch {
		case strings.Contains(url, "@"):
			parts := strings.Split(url, "@")
			currentVideo.PublisherID.ID = parts[len(parts)-1]
		case len(url) > 0:
			currentVideo.PublisherID.ID = url[strings.LastIndex(url, "/")+1:]
		default:
			currentVideo.PublisherID.ID = currentVideo.PublisherID.Name
		}
	}

	currentVideo.Duration = int32(srv.getVideoDuration(file_path))
	return currentVideo, nil
}
