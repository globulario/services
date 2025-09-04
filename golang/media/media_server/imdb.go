
package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
)

// GetIMDBPoster returns the direct URL to the highest-resolution poster image for the given IMDb ID.
// It navigates to the title page, finds the "mediaviewer" link, then locates the image with the correct rmID.
// Example: "tt0111161" -> "https://m.media-amazon.com/images/..."
func GetIMDBPoster(imdbID string) (string, error) {
	mainURL := "https://www.imdb.com/title/" + imdbID + "/"
	var posterViewerURL string
	var posterURL string

	c := colly.NewCollector()

	// Step 1: Find media viewer URL
	c.OnHTML("a.ipc-lockup-overlay", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		if strings.Contains(href, "/mediaviewer/") && posterViewerURL == "" {
			posterViewerURL = "https://www.imdb.com" + href
		}
	})

	if err := c.Visit(mainURL); err != nil {
		logger.Error("imdb visit failed", "url", mainURL, "err", err)
		return "", err
	}
	if posterViewerURL == "" {
		return "", fmt.Errorf("could not find media viewer URL for %s", imdbID)
	}

	// Step 2: Extract rmID from URL
	reRM := regexp.MustCompile(`/mediaviewer/(rm\d+)/`)
	match := reRM.FindStringSubmatch(posterViewerURL)
	if len(match) < 2 {
		return "", fmt.Errorf("could not extract rmID")
	}
	rmID := match[1] + "-curr"

	// Step 3: Visit media viewer and find correct image
	imgCollector := colly.NewCollector()

	imgCollector.OnHTML("img", func(e *colly.HTMLElement) {
		if e.Attr("data-image-id") == rmID {
			srcset := e.Attr("srcset")
			if srcset != "" {
				// Parse srcset and get highest resolution
				maxResURL := ""
				maxWidth := 0
				for _, part := range strings.Split(srcset, ",") {
					part = strings.TrimSpace(part)
					if items := strings.Split(part, " "); len(items) == 2 {
						u := items[0]
						widthStr := items[1]
						if strings.HasSuffix(widthStr, "w") {
							if w, err := strconv.Atoi(strings.TrimSuffix(widthStr, "w")); err == nil && w > maxWidth {
								maxWidth = w
								maxResURL = u
							}
						}
					}
				}
				if maxResURL != "" {
					posterURL = maxResURL
					return
				}
			}
			// fallback to src
			if posterURL == "" {
				posterURL = e.Attr("src")
			}
		}
	})

	if err := imgCollector.Visit(posterViewerURL); err != nil {
		logger.Error("imdb viewer visit failed", "url", posterViewerURL, "err", err)
		return "", fmt.Errorf("failed to visit viewer page: %v", err)
	}
	if posterURL == "" {
		return "", fmt.Errorf("poster image not found")
	}
	return posterURL, nil
}
