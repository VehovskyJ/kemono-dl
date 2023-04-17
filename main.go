package main

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/hashicorp/go-getter"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() {
	// Checks if URL was provided as an argument
	if len(os.Args) < 2 {
		log.Fatal("Please provide a url")
	}

	url := os.Args[1]

	// Validates the format of the provided URL to ensure it matches tyhe pattern for kemono.party URLs.
	pattern := `https://(kemono\.party/[^/]+/user/\d+|coomer\.party/[^/]+/user/\w+)`
	regex := regexp.MustCompile(pattern)

	if !regex.MatchString(url) {
		log.Fatal("Provided url is not in a correct format")
	}

	// Cleans the URL from any query parameters
	urlParts := strings.Split(url, "?")
	url = urlParts[0]

	// Extracts the service name from the url
	service := strings.TrimPrefix(url, "https://")
	service = strings.Split(service, "/")[0]
	service = strings.TrimSuffix(service, ".party")

	// Gets the creator's name
	name, err := getName(url)
	if err != nil {
		log.Fatalf("Failed to fetch user: %s", err)
	}

	// Gets the current working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %s", err)
	}

	// Creates a directory for the downloaded media
	dir := fmt.Sprintf("%s/%s/%s", wd, service, name)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatalf("Failed to create downlaod directory: %s", err)
	}

	// Retrieves teh list of all posts from the creator's page
	posts, err := getAllPosts(url)
	if err != nil {
		log.Fatalf("Failed to fetch all posts: %s", err)
	}

	// Downloads every post's content
	for _, post := range posts {
		postUrl := fmt.Sprintf("https://%s.party%s", service, post)
		err := downloadPost(postUrl, dir, name, service)
		if err != nil {
			log.Printf("Failed to download post: %s", err)
		}
		// Adds a delay between each request to prevent HTTP 429: Too many requests
		time.Sleep(300 * time.Millisecond)
	}
}

// Downloads media content from a post
func downloadPost(url string, directory string, name string, service string) error {
	log.Printf("Downloading post: %s", url)
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return err
	}

	var files []string
	// Extracts the media URLs from the Downloads section of the post
	doc.Find("h2:contains('Downloads')").Next().Find("a.post__attachment-link").Each(func(i int, selection *goquery.Selection) {
		file, exists := selection.Attr("href")
		if exists {
			files = append(files, file)
		}
	})

	// Extracts the media URLs from the Files section of the post
	doc.Find("h2:contains('Files')").Next().Find("a.fileThumb").Each(func(i int, selection *goquery.Selection) {
		file, exists := selection.Attr("href")
		if exists {
			files = append(files, file)
		}
	})

	// Matches the creator's id from the url using regex
	regex := regexp.MustCompile(`.*\/\w+\/post\/(\d+)`)
	match := regex.FindStringSubmatch(url)

	// Download all media from the post
	for _, file := range files {
		if service == "coomer" {
			file = strings.Split(file, "?")[0]
			file = fmt.Sprintf("https://coomer.party%s", file)
		}

		err := downloadFile(file, directory, name, match[1])
		if err != nil {
			log.Printf("Failed to download file: %s", err)
		}
	}

	return nil
}

// Downloads a file from a URL
func downloadFile(url string, directory string, name string, postID string) error {
	// Constructs the file path for the resulting file
	file := fmt.Sprintf("%s/%s_%s_%s", directory, name, postID, path.Base(url))

	if _, err := os.Stat(file); os.IsNotExist(err) {
		client := &getter.Client{
			Src:  url,
			Dst:  file,
			Mode: getter.ClientModeFile,
		}

		return client.Get()
	}

	return nil
}

// Returns array of links to all posts from teh creator
func getAllPosts(url string) ([]string, error) {
	pages, err := numberOfPages(url)
	if err != nil {
		return nil, err
	}

	// Iterates through every page and extracts all posts
	var posts []string
	for i := 0; i < pages; i++ {
		page := fmt.Sprintf("%s?o=%d", url, i*50)
		log.Println(page)
		res, err := http.Get(page)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		doc, err := goquery.NewDocumentFromReader(res.Body)
		if err != nil {
			return nil, err
		}

		// Searches for the post links in the HTML
		doc.Find("article.post-card").Each(func(i int, selection *goquery.Selection) {
			postUrl, _ := selection.Find("a").Attr("href")
			posts = append(posts, postUrl)
		})
	}

	return posts, nil
}

// Returns the total number of pages for a creator
func numberOfPages(url string) (int, error) {
	res, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return 0, err
	}

	// Searches for the HTML part containing the total number of posts
	postText := doc.Find("div.paginator small").Text()

	// Matches the number of posts from the element
	pattern := `Showing \d+ - \d+ of (\d+)`
	regex := regexp.MustCompile(pattern)
	matches := regex.FindStringSubmatch(postText)
	if len(matches) < 2 {
		return 0, errors.New("could not extract the number of posts")
	}

	posts, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, errors.New("could not extract the number of posts")
	}

	// Adds 49 to the total number of posts to account for rounding up when calculating the number of pages.
	// Then divides teh adjusted total by 50 to calculate the total number of pages
	pageCount := (posts + 49) / 50

	return pageCount, nil
}

// Returns name of the creator
func getName(url string) (string, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return "", err
	}

	name := doc.Find("span[itemprop='name']").Text()
	return name, nil
}
