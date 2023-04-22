package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/title/titlepb"
	colly "github.com/gocolly/colly/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// get the thumbnail fil with help of youtube dl...
func downloadThumbnail(video_id, video_url, video_path string) (string, error) {
	fmt.Println("download thumbnail for ", video_path)

	if len(video_id) == 0 {
		return "", errors.New("no video id was given")
	}
	if len(video_url) == 0 {
		return "", errors.New("no video url was given")
	}
	if len(video_path) == 0 {
		return "", errors.New("no video path was given")
	}

	lastIndex := -1
	if strings.Contains(video_path, ".mp4") {
		lastIndex = strings.LastIndex(video_path, ".")
	}

	// The hidden folder path...
	path_ := video_path[0:strings.LastIndex(video_path, "/")]

	name_ := video_path[strings.LastIndex(video_path, "/")+1:]
	if lastIndex != -1 {
		name_ = video_path[strings.LastIndex(video_path, "/")+1 : lastIndex]
	}

	thumbnail_path := path_ + "/.hidden/" + name_ + "/__thumbnail__"

	if Utility.Exists(thumbnail_path + "/" + "data_url.txt") {

		thumbnail, err := os.ReadFile(thumbnail_path + "/" + "data_url.txt")
		if err != nil {
			return "", err
		}

		return string(thumbnail), nil
	}

	Utility.CreateDirIfNotExist(thumbnail_path)
	files, _ := Utility.ReadDir(thumbnail_path)

	if len(files) == 0 {
		cmd := exec.Command("yt-dlp", video_url, "-o", video_id, "--write-thumbnail", "--skip-download")
		cmd.Dir = thumbnail_path

		err := cmd.Run()
		if err != nil {
			return "", err
		}

		files, err = Utility.ReadDir(thumbnail_path)
		if err != nil {
			return "", err
		}
	}

	if len(files) == 0 {
		return "", errors.New("no thumbernail found for url " + video_url)
	}

	thumbnail, err := Utility.CreateThumbnail(filepath.Join(thumbnail_path, files[0].Name()), 300, 180)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(thumbnail_path+"/"+"data_url.txt", []byte(thumbnail), 0664)
	if err != nil {
		return "", err
	}

	// cointain the data url...
	return thumbnail, nil
}

// Upload a video from porn hub and index it in the seach engine on the server side.
func indexPornhubVideo(token, id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {

	currentVideo := new(titlepb.Video)
	currentVideo.Casting = make([]*titlepb.Person, 0)
	currentVideo.Genres = []string{"adult"}
	currentVideo.Tags = []string{} // keep empty...
	currentVideo.Duration = int32(Utility.GetVideoDuration(file_path))
	currentVideo.URL = video_url
	currentVideo.ID = id

	// The poster
	currentVideo.Poster = new(titlepb.Poster)
	currentVideo.Poster.ID = currentVideo.ID + "-thumnail"
	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path) //e.Attr("src")
	if err != nil {
		return nil, err
	}

	currentVideo.Poster.TitleId = currentVideo.ID

	movieCollector := colly.NewCollector(
		colly.AllowedDomains("pornhub.com", "www.pornhub.com"),
	)

	/////////////////////////////////////////////////////////////////////////
	// Movie info
	/////////////////////////////////////////////////////////////////////////

	// function call on visition url...
	movieCollector.OnHTML(".inlineFree", func(e *colly.HTMLElement) {
		currentVideo.Description = strings.TrimSpace(e.Text)
	})

	movieCollector.OnHTML(".pstar-list-btn", func(e *colly.HTMLElement) {
		p := new(titlepb.Person)
		p.ID = e.Attr("data-id")
		p.FullName = strings.TrimSpace(e.Text)
		p.URL = "https://www.pornhub.com" + e.Attr("href")
		IndexPersonInformation(p)

		if len(p.ID) > 0 {
			currentVideo.Casting = append(currentVideo.Casting, p)
		}
	})

	movieCollector.OnHTML("#hd-leftColVideoPage > div:nth-child(1) > div.video-actions-container > div.video-actions-tabs > div.video-action-tab.about-tab.active > div.video-detailed-info > div.video-info-row.userRow > div.userInfo > div > a", func(e *colly.HTMLElement) {
		currentVideo.PublisherId = new(titlepb.Publisher)
		currentVideo.PublisherId.ID = e.Text
		currentVideo.PublisherId.Name = e.Text
		currentVideo.PublisherId.URL = e.Attr("href")

	})

	movieCollector.OnHTML(".count", func(e *colly.HTMLElement) {
		count := e.Text
		if strings.Contains(count, "K") {
			count = strings.ReplaceAll(count, "K", "")
			currentVideo.Count = int64(Utility.ToInt(e.Text)) * 100000
		} else if strings.Contains(count, "M") {
			count = strings.ReplaceAll(count, "M", "")
			currentVideo.Count = int64(Utility.ToInt(e.Text)) * 1000000
		}

	})

	movieCollector.OnHTML(".percent", func(e *colly.HTMLElement) {
		percent := e.Text
		percent = strings.ReplaceAll(percent, "%", "")
		currentVideo.Rating = float32(Utility.ToNumeric(percent) / 10)
	})

	movieCollector.OnHTML(".categoriesWrapper a", func(e *colly.HTMLElement) {
		tag := strings.TrimSpace(e.Text)
		if tag != "Suggest" {
			currentVideo.Tags = append(currentVideo.Tags, tag)
		}
	})

	movieCollector.Visit(video_url)

	return currentVideo, nil
}

// Index the person information...
func IndexPersonInformation(p *titlepb.Person) error {
	err := _indexPersonInformation_(p, strings.ReplaceAll(p.FullName, " ", "-"))
	if err != nil {
		values := strings.Split(p.FullName, " ")
		if len(values) == 1 {
			err = _indexPersonInformation_(p, values[0])
		} else if len(values) == 2 {
			err = _indexPersonInformation_(p, values[1])
		}
	}

	// try a last time with the id itself...
	if err != nil {
		return _indexPersonInformation_(p, p.ID)
	}

	return err
}

// That here I will try to get more information about a given person...
func _indexPersonInformation_(p *titlepb.Person, id string) error {
	movieCollector := colly.NewCollector(
		colly.AllowedDomains("www.freeones.com", "freeones.com"),
	)

	// about

	// So here I will define collector's...
	biographySelector := `#description > div > div.common-text`
	movieCollector.OnHTML(biographySelector, func(e *colly.HTMLElement) {
		html, err := e.DOM.Html()
		if err == nil {
			p.Biography = html
		}
	})

	// The profile image.
	profilePictureSelector := `body > div.flex-footer-wrapper > div > div.right-container.flex-m-column.d-m-flex.flex-1 > main > div.px-2.px-md-3 > section > header > div.dashboard-image-container > a > img`
	movieCollector.OnHTML(profilePictureSelector, func(e *colly.HTMLElement) {
		p.Picture = strings.TrimSpace(e.Attr("src"))
	})

	// The birthdate
	birthdateSelector := `#search-result > section > form > div:nth-child(4) > ul > li:nth-child(5) > span.font-size-xs > a > span`
	movieCollector.OnHTML(birthdateSelector, func(e *colly.HTMLElement) {
		p.BirthDate = strings.TrimSpace(e.Text)
	})

	// The birtplace.
	birthplaceSelector := `#search-result > section > form > div:nth-child(4) > ul > li:nth-child(13) > span.font-size-xs`
	movieCollector.OnHTML(birthplaceSelector, func(e *colly.HTMLElement) {
		html, err := e.DOM.Html()
		if err == nil {
			p.BirthPlace = html
		}
	})

	// The carrer status
	carrerStatusSelector := `#search-result > section > form > div:nth-child(4) > ul > li:nth-child(9) > span.font-size-xs > a > span`
	movieCollector.OnHTML(carrerStatusSelector, func(e *colly.HTMLElement) {
		p.CareerSatus = strings.TrimSpace(e.Text)
	})

	// The aliases.
	aliasesSelector := `#search-result > section > form > div:nth-child(4) > ul > li:nth-child(2) > span.font-size-xs`
	p.Aliases = make([]string, 0)
	movieCollector.OnHTML(aliasesSelector, func(e *colly.HTMLElement) {
		e.ForEach(".text-underline-always", func(index int, child_ *colly.HTMLElement) {
			p.Aliases = append(p.Aliases, child_.Text)
		})
	})

	url := "https://www.freeones.com/" + id + "/bio"
	err := movieCollector.Visit(url)
	if err == nil {
		p.ID = Utility.GenerateUUID(url) // so i will set the id to the url...
		p.URL = url
		p.Gender = "female"
	}

	return err
}

// Upload a video from porn hub and index it in the seach engine on the server side.
func indexXhamsterVideo(token, video_id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {

	currentVideo := new(titlepb.Video)
	currentVideo.Casting = make([]*titlepb.Person, 0)
	currentVideo.Genres = []string{"adult"}
	currentVideo.Tags = []string{} // keep empty...
	currentVideo.URL = video_url
	currentVideo.ID = video_id
	currentVideo.Duration = int32(Utility.GetVideoDuration(file_path))

	currentVideo.Poster = new(titlepb.Poster)
	currentVideo.Poster.ID = currentVideo.ID + "-thumnail"
	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path) //e.Attr("src")
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = video_url
	currentVideo.Poster.TitleId = currentVideo.ID

	movieCollector := colly.NewCollector(
		colly.AllowedDomains("www.xhamster.com", "xhamster.com", "fr.xhamster.com"),
	)

	// Video title
	movieCollector.OnHTML("body > div.main-wrap > main > div.width-wrap.with-player-container > h1", func(e *colly.HTMLElement) {
		currentVideo.Description = strings.TrimSpace(e.Text)
	})

	// Casting, Categories, Channels
	movieCollector.OnHTML("body > div.main-wrap > main > div.width-wrap.with-player-container > nav > ul > li > a", func(e *colly.HTMLElement) {
		if strings.Contains(e.Attr("href"), "pornstars") {
			p := new(titlepb.Person)
			p.URL = e.Attr("href")
			p.ID = strings.TrimSpace(e.Text)
			p.FullName = strings.TrimSpace(e.Text)
			IndexPersonInformation(p)

			if len(p.ID) > 0 {
				currentVideo.Casting = append(currentVideo.Casting, p)
			}
		} else if strings.Contains(e.Attr("href"), "categories") {
			tag := strings.TrimSpace(e.Text)
			if len(tag) > 3 {
				currentVideo.Tags = append(currentVideo.Tags, tag)
			}
		} else if strings.Contains(e.Attr("href"), "channels") {
			currentVideo.PublisherId = new(titlepb.Publisher)
			currentVideo.PublisherId.URL = e.Attr("href")
			currentVideo.PublisherId.ID = e.Text
			currentVideo.PublisherId.Name = e.Text
		}
	})

	movieCollector.OnHTML(".header-icons", func(e *colly.HTMLElement) {

		e.ForEach("span", func(index int, child *colly.HTMLElement) {
			// The poster
			str := strings.TrimSpace(child.Text)

			if strings.Contains(str, "%") {
				percent := strings.TrimSpace(strings.ReplaceAll(str, "%", ""))
				currentVideo.Rating = float32(Utility.ToNumeric(percent) / 10)
			} else {
				str = strings.ReplaceAll(str, ",", "")
				currentVideo.Count = int64(Utility.ToInt(str))
			}

		})
	})

	movieCollector.Visit(video_url)

	return currentVideo, nil
}

func indexXnxxVideo(token, video_id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {

	currentVideo := new(titlepb.Video)
	currentVideo.Casting = make([]*titlepb.Person, 0)
	currentVideo.Genres = []string{"adult"}
	currentVideo.Tags = []string{} // keep empty...
	currentVideo.URL = video_url
	currentVideo.Duration = int32(Utility.GetVideoDuration(file_path))

	currentVideo.ID = video_id

	currentVideo.Poster = new(titlepb.Poster)
	currentVideo.Poster.ID = currentVideo.ID + "-thumnail"
	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path) //e.Attr("src")
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = video_url
	currentVideo.Poster.TitleId = currentVideo.ID

	movieCollector := colly.NewCollector(
		colly.AllowedDomains("www.xnxx.com", "xnxx.com"),
	)

	// Video title
	movieCollector.OnHTML(".clear-infobar", func(e *colly.HTMLElement) {

		currentVideo.Description = strings.TrimSpace(e.Text)
		e.ForEach("strong", func(index int, child *colly.HTMLElement) {
			// The description
			currentVideo.Description = strings.TrimSpace(child.Text)
		})

		e.ForEach("p", func(index int, child *colly.HTMLElement) {
			// The description
			currentVideo.Description += "</br>" + strings.TrimSpace(child.Text)
		})

		e.ForEach(".metadata", func(index int, child *colly.HTMLElement) {
			child.ForEach(".gold-plate, .free-plate", func(index int, child_ *colly.HTMLElement) {
				currentVideo.PublisherId = new(titlepb.Publisher)
				currentVideo.PublisherId.URL = "https://www.xnxx.com" + child_.Attr("href")
				currentVideo.PublisherId.ID = child_.Text
				currentVideo.PublisherId.Name = child_.Text
			})

			values := strings.Split(child.Text, "-")
			if currentVideo.PublisherId != nil {
				txt := strings.TrimSpace(values[0])
				currentVideo.PublisherId.Name = txt[len(currentVideo.PublisherId.Name)+1:]
			} else {
				currentVideo.PublisherId = &titlepb.Publisher{ID: strings.TrimSpace(values[0]), Name: strings.TrimSpace(values[0])}
			}

			// The number of view
			str := strings.TrimSpace(strings.ReplaceAll(values[2], ",", ""))
			currentVideo.Count = int64(Utility.ToInt(str))

			// The resolution...
			tag := strings.TrimSpace(values[1]) // the resolution 360p, 720p
			currentVideo.Tags = append(currentVideo.Tags, tag)

		})
	})

	movieCollector.OnHTML(".metadata-row.video-description", func(e *colly.HTMLElement) {
		if len(currentVideo.Description) > 0 {
			currentVideo.Description += "</br>"
		}

		currentVideo.Description += strings.TrimSpace(e.Text)
	})

	// Casting, Categories
	movieCollector.OnHTML("#video-content-metadata > div.metadata-row.video-tags > a", func(e *colly.HTMLElement) {
		if strings.Contains(e.Attr("class"), "is-pornstar") {
			p := new(titlepb.Person)
			p.URL = "https://www.xnxx.com" + e.Attr("href")
			p.ID = strings.TrimSpace(e.Text)
			p.FullName = strings.TrimSpace(e.Text)

			IndexPersonInformation(p)

			if len(p.ID) > 0 {
				currentVideo.Casting = append(currentVideo.Casting, p)
			}
		} else {
			tag := strings.TrimSpace(e.Text)
			if len(tag) > 3 {
				currentVideo.Tags = append(currentVideo.Tags, tag)
			}
		}
	})

	movieCollector.OnHTML(".vote-actions", func(e *colly.HTMLElement) {
		var like, unlike float32

		e.ForEach(".vote-action-good", func(index int, child *colly.HTMLElement) {

			child.ForEach(".value", func(index int, child_ *colly.HTMLElement) {
				str := strings.TrimSpace(strings.ReplaceAll(child_.Text, ",", ""))
				like = float32(Utility.ToNumeric(str))
			})
		})

		e.ForEach(".vote-action-bad", func(index int, child *colly.HTMLElement) {
			// The poster
			child.ForEach(".value", func(index int, child_ *colly.HTMLElement) {
				str := strings.TrimSpace(strings.ReplaceAll(child_.Text, ",", ""))
				unlike = float32(Utility.ToNumeric(str))
			})
		})

		currentVideo.Rating = like / (like + unlike) * 10 // create a score on 10
	})

	movieCollector.Visit(video_url)
	return currentVideo, nil
}

// Upload a video from porn hub and index it in the seach engine on the server side.
func indexXvideosVideo(token, video_id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {

	currentVideo := new(titlepb.Video)
	currentVideo.Casting = make([]*titlepb.Person, 0)
	currentVideo.Genres = []string{"adult"}
	currentVideo.Tags = []string{} // keep empty...
	currentVideo.URL = video_url
	currentVideo.ID = video_id
	currentVideo.Poster = new(titlepb.Poster)
	currentVideo.Poster.ID = currentVideo.ID + "-thumnail"
	currentVideo.Duration = int32(Utility.GetVideoDuration(file_path))
	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path) //e.Attr("src")
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = video_url
	currentVideo.Poster.TitleId = currentVideo.ID

	movieCollector := colly.NewCollector(
		colly.AllowedDomains("www.xvideos.com", "xvideos.com"),
	)

	// function call on visition url...
	movieCollector.OnHTML(".page-title", func(e *colly.HTMLElement) {

		currentVideo.Description = strings.TrimSpace(e.Text)
		e.ForEach(".video-hd-mark", func(index int, child *colly.HTMLElement) {
			// The poster
			tag := strings.TrimSpace(child.Text)
			if len(tag) > 3 {
				currentVideo.Tags = append(currentVideo.Tags, tag)
			}
		})
	})

	movieCollector.OnHTML(".label.profile", func(e *colly.HTMLElement) {
		p := new(titlepb.Person)
		p.URL = "https://www.xvideos.com" + e.Attr("href")

		e.ForEach(".name", func(index int, child *colly.HTMLElement) {
			// The poster
			p.ID = child.Text
			p.FullName = child.Text
			IndexPersonInformation(p)
		})
		if len(p.ID) > 0 {
			currentVideo.Casting = append(currentVideo.Casting, p)
		}
	})

	movieCollector.OnHTML(".uploader-tag ", func(e *colly.HTMLElement) {
		currentVideo.PublisherId = new(titlepb.Publisher)
		currentVideo.PublisherId.URL = e.Attr("href")

		e.ForEach(".name", func(index int, child *colly.HTMLElement) {
			// The poster
			currentVideo.PublisherId.ID = child.Text
			currentVideo.PublisherId.Name = child.Text

		})

	})

	// The number of view
	movieCollector.OnHTML("#v-actions-left > div.vote-actions > div.rate-infos > span", func(e *colly.HTMLElement) {
		fmt.Println(e.Text)
		str := strings.Split(e.Text, " ")[0]
		str = strings.ReplaceAll(str, ",", "")
		currentVideo.Count = int64(Utility.ToInt(str))
	})

	// The raiting
	movieCollector.OnHTML("#v-actions-left > div.vote-actions > div.rate-infos > div.perc > div.good > span.rating-good-perc", func(e *colly.HTMLElement) {
		percent := e.Text
		percent = strings.ReplaceAll(percent, "%", "")
		currentVideo.Rating = float32(Utility.ToNumeric(percent) / 10)
	})

	// The tags
	movieCollector.OnHTML("#main > div.video-metadata.video-tags-list.ordered-label-list.cropped > ul > li > a", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		if strings.HasPrefix(href, "/tags/") {
			currentVideo.Tags = append(currentVideo.Tags, strings.TrimSpace(e.Text))
		}
	})

	movieCollector.Visit(video_url)
	return currentVideo, nil
}

////////////////////////////////// Youtube ////////////////////////////////////////////////

func indexYoutubeVideo(token, video_id, video_url, index_path, video_path, file_path string) (*titlepb.Video, error) {

	currentVideo := new(titlepb.Video)
	currentVideo.Casting = make([]*titlepb.Person, 0)
	currentVideo.Genres = []string{"youtube"}
	currentVideo.Tags = []string{} // keep empty...
	currentVideo.URL = video_url
	currentVideo.ID = video_id

	currentVideo.Poster = new(titlepb.Poster)
	currentVideo.Poster.ID = currentVideo.ID + "-thumnail"
	var err error
	currentVideo.Poster.ContentUrl, err = downloadThumbnail(currentVideo.ID, video_url, file_path) //e.Attr("src")
	if err != nil {
		return nil, err
	}
	currentVideo.Poster.URL = video_url
	currentVideo.Poster.TitleId = currentVideo.ID

	// For that one I will made use of web-api from https://noembed.com/embed
	url := `https://noembed.com/embed?url=` + video_url
	var myClient = &http.Client{Timeout: 5 * time.Second}
	r, err := myClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	target := make(map[string]interface{})
	json.NewDecoder(r.Body).Decode(&target)

	currentVideo.PublisherId = new(titlepb.Publisher)
	if target["author_url"] != nil {
		currentVideo.PublisherId.URL = target["author_url"].(string)
		if target["author_name"] != nil {
			currentVideo.PublisherId.Name = target["author_name"].(string)
			currentVideo.Description = target["title"].(string)
		}
		if strings.Contains(currentVideo.PublisherId.URL, "@") {
			currentVideo.PublisherId.ID = strings.Split(currentVideo.PublisherId.URL, "@")[1]
		} else if len(currentVideo.PublisherId.URL) > 0 {
			currentVideo.PublisherId.ID = currentVideo.PublisherId.URL[strings.LastIndex(currentVideo.PublisherId.URL, "/")+1:]
		} else {
			currentVideo.PublisherId.ID = currentVideo.PublisherId.Name
		}
	}

	currentVideo.Duration = int32(Utility.GetVideoDuration(file_path))

	return currentVideo, nil
}
