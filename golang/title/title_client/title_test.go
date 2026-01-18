package title_client

import (
	//"encoding/json"
	"fmt"
	"testing"

	"github.com/globulario/services/golang/testutil"
	//"github.com/globulario/services/golang/title/titlepb"
)

// newTitleClient creates a client for testing, skipping if external services are not available.
func newTitleClient(t *testing.T) *Title_Client {
	t.Helper()
	testutil.SkipIfNoExternalServices(t)

	addr := testutil.GetAddress()
	client, err := NewTitleService_Client(addr, "title.TitleService")
	if err != nil {
		t.Fatalf("NewTitleService_Client: %v", err)
	}
	return client
}

/*
// Test various function here.
func TestCreateTitle(t *testing.T) {
	if err != nil {
		fmt.Println("fail to connect to service with error: ", err)
		return
	}

	// Here I will create a new title.
	title := new(titlepb.Title)

	title.ID = "tt0390244"
	title.URL = "https://www.imdb.com/title/tt0390244"
	title.Name = "The Matrix Online"
	title.Type = "VideoGame"
	title.Year = 2005
	title.Rating = 6.7
	title.RatingCount = 501
	title.Directors = []*titlepb.Person{
		{
			ID:       "nm1893445",
			URL:      "https://www.imdb.com/name/nm1893445",
			FullName: "Nathan Hendrickson"},
		{
			ID:       "nm0905154",
			URL:      "https://www.imdb.com/name/nm0905154",
			FullName: "Lana Wachowski"},
		{
			ID:       "nm0905152",
			URL:      "https://www.imdb.com/name/nm0905152",
			FullName: "Lilly Wachowski"},
	}

	title.Writers = []*titlepb.Person{
		{
			ID:       "nm1715514",
			URL:      "https://www.imdb.com/name/nm1715514",
			FullName: "Brannon Boren"},
		{
			ID:       "nm2254412",
			URL:      "https://www.imdb.com/name/nm2254412",
			FullName: "Erik J. Caponi"},
		{
			ID:       "nm0149503",
			URL:      "https://www.imdb.com/name/nm0149503",
			FullName: "Paul Chadwick"},
	}

	title.Actors = []*titlepb.Person{
		{
			ID:       "nm0019569",
			URL:      "https://www.imdb.com/name/nm0019569",
			FullName: "Mary Alice"},
		{
			ID:       "nm1470128",
			URL:      "https://www.imdb.com/name/nm1470128",
			FullName: "Tanveer K. Atwal"},
		{
			ID:       "nm0000899",
			URL:      "https://www.imdb.com/name/nm0000899",
			FullName: "Monica Bellucci"},
	}

	title.Genres = []string{"Action", "Adventure", "Sci-Fi"}

	title.Language = []string{"English"}

	title.Nationalities = []string{"United States"}

	title.Description = "Set after 'The Matrix Revolutions' the Humans and Machines have peace but the Humans can jack into the Matrix and do missions and form factions/crews."

	title.AKA = []string{"MxO", "The Matrix Online"}

	title.Poster = &titlepb.Poster{
		ID:         "rm3997146368",
		TitleId:    "tt0390244",
		URL:        "https://www.imdb.com/title/tt0390244/mediaviewer/rm3997146368",
		ContentUrl: "https://m.media-amazon.com/images/M/MV5BMTYxNTM5MDkwMF5BMl5BanBnXkFtZTcwMTAzMTEzMQ@@._V1_.jpg",
	}

	// Test create a new title.
	err := client.CreateTitle("", "/tmp/titles", title)
	if err != nil {
		fmt.Println("---", err)
	}
}

func TestGetTitleById(t *testing.T) {
	title, paths, err := client.GetTitleById("/tmp/titles", "tt0390244")
	if err != nil {
		fmt.Println("---", err)
		return
	}

	fmt.Println("-------> find title ", title.ID, title.Name, paths)
}

func TestSearchTitles(t *testing.T) {
	summary, hits, facets,  err := client.SearchTitle("/tmp/titles", "revolutions peace Monica", []string{""})
	if err != nil {
		fmt.Println("---", err)
		return
	}

	fmt.Println("summary ", summary)
	fmt.Println("hits ", hits)
	fmt.Println("facets", facets)
}

func TestAssociateFile(t *testing.T) {
	path := "/media/dave/1F29-8099/movies/The Matrix (1999) [1080p]/The.Matrix.1999.1080p.BrRip.x264.YIFY.mp4"
	titleId := "tt0390244"

	// simple test.
	err := client.AssociateFileWithTitle("/tmp/titles", titleId, path )
	if err != nil {
		fmt.Println("fail to associate file with error: ", err)
	}

}

func TestGetTitleFiles(t *testing.T) {
	files, err := client.GetTitleFiles("/tmp/titles", "tt0390244")
	if err != nil {
		fmt.Println("---", err)
		return
	}
	fmt.Println("found files", files)

}

func TestGetFileTitles(t *testing.T) {
	titles, err := client.GetFileTitles("/tmp/titles", "/media/dave/1F29-8099/movies/The Matrix (1999) [1080p]/The.Matrix.1999.1080p.BrRip.x264.YIFY.mp4")
	if err != nil {
		fmt.Println("---", err)
		return
	}
	fmt.Println("found titles", titles)

}

func TestDeleteTitle(t *testing.T) {
	err := client.DeleteTitle("/tmp/titles", "tt0390244")
	if err != nil {
		fmt.Println("---", err)
		return
	}
	title, _, err := client.GetTitleById("/tmp/titles", "tt0390244")
	if err != nil {
		fmt.Println("---> title is deleted!")
		return
	}

	fmt.Println("fail to delete title ", title.Name)

}

func TestDissociateFile(t *testing.T) {
	path := "/media/dave/1F29-8099/movies/The Matrix (1999) [1080p]/The.Matrix.1999.1080p.BrRip.x264.YIFY.mp4"
	titleId := "tt0390244"

	// simple test.
	err := client.DissociateFileWithTitle("/tmp/titles", titleId, path )
	if err != nil {
		fmt.Println("fail to associate file with error: ", err)
	}

}*/

func TestGetFileVideos(t *testing.T) {
	client := newTitleClient(t)
	titles, err := client.GetFileVideos("/var/globular/search/videos", "/mnt/8e7a3e9a-8530-4b8e-9947-fb728c709cc2/movie/xxx/pornhub/ph604652e34b5d9.mp4")
	if err != nil {
		fmt.Println("---", err)
		return
	}
	fmt.Println("found titles", titles)
}
