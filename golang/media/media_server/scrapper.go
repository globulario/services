package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	ytDlpBinary              = "yt-dlp"
	ytDlpTimeout             = 30 * time.Second
	thumbWidth               = 300
	thumbHeight              = 180
	thumbDataURLFilename     = "data_url.txt"
	hiddenDirName            = ".hidden"
	thumbnailLeafFolderName  = "__thumbnail__"
	noEmbedEndpointTemplate  = "https://noembed.com/embed?url="
)

// buildThumbnailDir returns the hidden thumbnail directory for a given video file path.
// e.g. /path/movie.mp4 -> /path/.hidden/movie/__thumbnail__
func buildThumbnailDir(videoPath string) (string, string) {
	dir := filepath.Dir(videoPath)
	base := filepath.Base(videoPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return filepath.Join(dir, hiddenDirName, name, thumbnailLeafFolderName), name
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
func downloadThumbnail(video_id, video_url, video_path string) (string, error) {
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
	if Utility.Exists(cachePath) {
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
func indexPornhubVideo(token, id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting:  make([]*titlepb.Person, 0),
		Genres:   []string{"adult"},
		Tags:     []string{},
		Duration: int32(getVideoDuration(file_path)),
		URL:      video_url,
		ID:       id,
		Poster:   &titlepb.Poster{ID: id + "-thumnail"}, // keep original ID spelling for compatibility
	}

	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path)
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.TitleId = currentVideo.ID

	c := colly.NewCollector(colly.AllowedDomains("pornhub.com", "www.pornhub.com"))

	c.OnHTML(".inlineFree", func(e *colly.HTMLElement) {
		currentVideo.Description = strings.TrimSpace(e.Text)
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

	c.OnHTML("#hd-leftColVideoPage .userInfo a", func(e *colly.HTMLElement) {
		currentVideo.PublisherID = &titlepb.Publisher{
			ID:   e.Text,
			Name: e.Text,
			URL:  e.Attr("href"),
		}
	})

	c.OnHTML(".count", func(e *colly.HTMLElement) {
		currentVideo.Count = viewCountFromSuffix(e.Text)
	})

	c.OnHTML(".percent", func(e *colly.HTMLElement) {
		percent := strings.TrimSpace(strings.ReplaceAll(e.Text, "%", ""))
		currentVideo.Rating = float32(Utility.ToNumeric(percent) / 10)
	})

	c.OnHTML(".categoriesWrapper a", func(e *colly.HTMLElement) {
		tag := strings.TrimSpace(e.Text)
		if tag != "Suggest" {
			currentVideo.Tags = append(currentVideo.Tags, tag)
		}
	})

	if err := c.Visit(video_url); err != nil {
		return nil, err
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

func indexXhamsterVideo(token, video_id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting:  make([]*titlepb.Person, 0),
		Genres:   []string{"adult"},
		Tags:     []string{},
		URL:      video_url,
		ID:       video_id,
		Duration: int32(getVideoDuration(file_path)),
		Poster:   &titlepb.Poster{ID: video_id + "-thumnail"},
	}

	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path)
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = video_url
	currentVideo.Poster.TitleId = currentVideo.ID

	c := colly.NewCollector(colly.AllowedDomains("www.xhamster.com", "xhamster.com", "fr.xhamster.com"))

	c.OnHTML("body > div.main-wrap > main > div.width-wrap.with-player-container > h1", func(e *colly.HTMLElement) {
		currentVideo.Description = strings.TrimSpace(e.Text)
	})

	c.OnHTML("body > div.main-wrap > main > div.width-wrap.with-player-container > nav > ul > li > a", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		switch {
		case strings.Contains(href, "pornstars"):
			p := &titlepb.Person{
				URL:      href,
				ID:       strings.TrimSpace(e.Text),
				FullName: strings.TrimSpace(e.Text),
			}
			if err := IndexPersonInformation(p); err != nil {
				slog.Warn("IndexPersonInformation failed", "person", p.FullName, "err", err)
			}
			if len(p.ID) > 0 {
				currentVideo.Casting = append(currentVideo.Casting, p)
			}
		case strings.Contains(href, "categories"):
			if tag := strings.TrimSpace(e.Text); len(tag) > 3 {
				currentVideo.Tags = append(currentVideo.Tags, tag)
			}
		case strings.Contains(href, "channels"):
			currentVideo.PublisherID = &titlepb.Publisher{
				URL:  href,
				ID:   e.Text,
				Name: e.Text,
			}
		}
	})

	c.OnHTML(".header-icons", func(e *colly.HTMLElement) {
		e.ForEach("span", func(_ int, child *colly.HTMLElement) {
			txt := strings.TrimSpace(child.Text)
			if strings.Contains(txt, "%") {
				percent := strings.TrimSpace(strings.ReplaceAll(txt, "%", ""))
				currentVideo.Rating = float32(Utility.ToNumeric(percent) / 10)
			} else {
				currentVideo.Count = viewCountFromSuffix(txt)
			}
		})
	})

	if err := c.Visit(video_url); err != nil {
		return nil, err
	}
	return currentVideo, nil
}

// -----------------------------------------------------------------------------
// Indexers – XNXX
// -----------------------------------------------------------------------------

func indexXnxxVideo(token, video_id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting:  make([]*titlepb.Person, 0),
		Genres:   []string{"adult"},
		Tags:     []string{},
		URL:      video_url,
		Duration: int32(getVideoDuration(file_path)),
		ID:       video_id,
		Poster:   &titlepb.Poster{ID: video_id + "-thumnail"},
	}

	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path)
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = video_url
	currentVideo.Poster.TitleId = currentVideo.ID

	c := colly.NewCollector(colly.AllowedDomains("www.xnxx.com", "xnxx.com"))

	c.OnHTML(".clear-infobar", func(e *colly.HTMLElement) {
		currentVideo.Description = strings.TrimSpace(e.Text)

		e.ForEach("strong", func(_ int, ch *colly.HTMLElement) {
			currentVideo.Description = strings.TrimSpace(ch.Text)
		})

		e.ForEach("p", func(_ int, ch *colly.HTMLElement) {
			currentVideo.Description += "</br>" + strings.TrimSpace(ch.Text)
		})

		e.ForEach(".metadata", func(_ int, ch *colly.HTMLElement) {
			ch.ForEach(".gold-plate, .free-plate", func(_ int, ch2 *colly.HTMLElement) {
				currentVideo.PublisherID = &titlepb.Publisher{
					URL:  "https://www.xnxx.com" + ch2.Attr("href"),
					ID:   ch2.Text,
					Name: ch2.Text,
				}
			})

			parts := strings.Split(ch.Text, "-")
			if currentVideo.PublisherID != nil {
				txt := strings.TrimSpace(parts[0])
				if len(txt) > len(currentVideo.PublisherID.Name) {
					currentVideo.PublisherID.Name = txt[len(currentVideo.PublisherID.Name)+1:]
				}
			} else if len(parts) > 0 {
				name := strings.TrimSpace(parts[0])
				currentVideo.PublisherID = &titlepb.Publisher{ID: name, Name: name}
			}

			if len(parts) > 2 {
				currentVideo.Count = viewCountFromSuffix(parts[2])
			}
			if len(parts) > 1 {
				tag := strings.TrimSpace(parts[1]) // e.g., "720p"
				currentVideo.Tags = append(currentVideo.Tags, tag)
			}
		})
	})

	c.OnHTML(".metadata-row.video-description", func(e *colly.HTMLElement) {
		if len(currentVideo.Description) > 0 {
			currentVideo.Description += "</br>"
		}
		currentVideo.Description += strings.TrimSpace(e.Text)
	})

	c.OnHTML("#video-content-metadata > div.metadata-row.video-tags > a", func(e *colly.HTMLElement) {
		if strings.Contains(e.Attr("class"), "is-pornstar") {
			p := &titlepb.Person{
				URL:      "https://www.xnxx.com" + e.Attr("href"),
				ID:       strings.TrimSpace(e.Text),
				FullName: strings.TrimSpace(e.Text),
			}
			if err := IndexPersonInformation(p); err != nil {
				slog.Warn("IndexPersonInformation failed", "person", p.FullName, "err", err)
			}
			if len(p.ID) > 0 {
				currentVideo.Casting = append(currentVideo.Casting, p)
			}
		} else {
			if tag := strings.TrimSpace(e.Text); len(tag) > 3 {
				currentVideo.Tags = append(currentVideo.Tags, tag)
			}
		}
	})

	c.OnHTML(".vote-actions", func(e *colly.HTMLElement) {
		var like, unlike float32

		e.ForEach(".vote-action-good .value", func(_ int, ch *colly.HTMLElement) {
			like = float32(Utility.ToNumeric(strings.ReplaceAll(strings.TrimSpace(ch.Text), ",", "")))
		})
		e.ForEach(".vote-action-bad .value", func(_ int, ch *colly.HTMLElement) {
			unlike = float32(Utility.ToNumeric(strings.ReplaceAll(strings.TrimSpace(ch.Text), ",", "")))
		})

		if like+unlike > 0 {
			currentVideo.Rating = like / (like + unlike) * 10
		}
	})

	if err := c.Visit(video_url); err != nil {
		return nil, err
	}
	return currentVideo, nil
}

// -----------------------------------------------------------------------------
// Indexers – XVideos
// -----------------------------------------------------------------------------

func indexXvideosVideo(token, video_id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting:  make([]*titlepb.Person, 0),
		Genres:   []string{"adult"},
		Tags:     []string{},
		URL:      video_url,
		ID:       video_id,
		Poster:   &titlepb.Poster{ID: video_id + "-thumnail"},
		Duration: int32(getVideoDuration(file_path)),
	}

	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path)
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = video_url
	currentVideo.Poster.TitleId = currentVideo.ID

	c := colly.NewCollector(colly.AllowedDomains("www.xvideos.com", "xvideos.com"))

	c.OnHTML(".page-title", func(e *colly.HTMLElement) {
		currentVideo.Description = strings.TrimSpace(e.Text)
		e.ForEach(".video-hd-mark", func(_ int, ch *colly.HTMLElement) {
			if tag := strings.TrimSpace(ch.Text); len(tag) > 3 {
				currentVideo.Tags = append(currentVideo.Tags, tag)
			}
		})
	})

	c.OnHTML(".label.profile", func(e *colly.HTMLElement) {
		p := &titlepb.Person{
			URL: "https://www.xvideos.com" + e.Attr("href"),
		}
		e.ForEach(".name", func(_ int, ch *colly.HTMLElement) {
			p.ID = ch.Text
			p.FullName = ch.Text
			if err := IndexPersonInformation(p); err != nil {
				slog.Warn("IndexPersonInformation failed", "person", p.FullName, "err", err)
			}
		})
		if len(p.ID) > 0 {
			currentVideo.Casting = append(currentVideo.Casting, p)
		}
	})

	c.OnHTML(".uploader-tag", func(e *colly.HTMLElement) {
		pub := &titlepb.Publisher{URL: e.Attr("href")}
		e.ForEach(".name", func(_ int, ch *colly.HTMLElement) {
			pub.ID = ch.Text
			pub.Name = ch.Text
		})
		currentVideo.PublisherID = pub
	})

	c.OnHTML("#v-actions-left > div.vote-actions > div.rate-infos > span", func(e *colly.HTMLElement) {
		// first span typically: "1,234,567 views"
		fields := strings.SplitN(strings.TrimSpace(e.Text), " ", 2)
		if len(fields) > 0 {
			currentVideo.Count = viewCountFromSuffix(fields[0])
		}
	})

	c.OnHTML("#v-actions-left .rating-good-perc", func(e *colly.HTMLElement) {
		percent := strings.ReplaceAll(strings.TrimSpace(e.Text), "%", "")
		currentVideo.Rating = float32(Utility.ToNumeric(percent) / 10)
	})

	c.OnHTML("#main .video-metadata.video-tags-list.ordered-label-list.cropped a", func(e *colly.HTMLElement) {
		if strings.HasPrefix(e.Attr("href"), "/tags/") {
			currentVideo.Tags = append(currentVideo.Tags, strings.TrimSpace(e.Text))
		}
	})

	if err := c.Visit(video_url); err != nil {
		return nil, err
	}
	return currentVideo, nil
}

// -----------------------------------------------------------------------------
// Indexers – YouTube
// -----------------------------------------------------------------------------

func indexYoutubeVideo(token, video_id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {
	currentVideo := &titlepb.Video{
		Casting: make([]*titlepb.Person, 0),
		Genres:  []string{"youtube"},
		Tags:    []string{},
		URL:     video_url,
		ID:      video_id,
		Poster:  &titlepb.Poster{ID: video_id + "-thumnail"},
	}

	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path)
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

	currentVideo.Duration = int32(getVideoDuration(file_path))
	return currentVideo, nil
}
