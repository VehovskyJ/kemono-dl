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
	if len(os.Args) < 2 {
		log.Fatal("Please provide a url")
	}

	url := os.Args[1]

	pattern := `https://kemono\.party/patreon/user/\d+`
	regex := regexp.MustCompile(pattern)

	if !regex.MatchString(url) {
		log.Fatal("Provided url is not in a correct format")
	}

	urlParts := strings.Split(url, "?")
	url = urlParts[0]

	name, err := getName(url)
	if err != nil {
		log.Fatalf("Failed to fetch user: %s", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %s", err)
	}

	dir := fmt.Sprintf("%s/kemono/%s", wd, name)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatalf("Failed to create downlaod directory: %s", err)
	}

	posts, err := getAllPosts(url)
	if err != nil {
		log.Fatalf("Failed to fetch all posts: %s", err)
	}

	for _, post := range posts {
		postUrl := fmt.Sprintf("https://kemono.party%s", post)
		err := downloadPost(postUrl, dir, name)
		if err != nil {
			log.Printf("Failed to download post: %s", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func downloadPost(url string, directory string, name string) error {
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

	var downloads []string
	doc.Find("h2:contains('Downloads')").Next().Find("a.post__attachment-link").Each(func(i int, selection *goquery.Selection) {
		download, exists := selection.Attr("href")
		if exists {
			downloads = append(downloads, download)
		}
	})

	var files []string
	doc.Find("h2:contains('Files')").Next().Find("a.fileThumb").Each(func(i int, selection *goquery.Selection) {
		file, exists := selection.Attr("href")
		if exists {
			files = append(files, file)
		}
	})

	regex := regexp.MustCompile(`.*\/\d+\/post\/(\d+)`)
	match := regex.FindStringSubmatch(url)

	for _, download := range downloads {
		err := downloadFile(download, directory, name, match[1])
		if err != nil {
			log.Printf("Faield to download file: %s", err)
		}
	}

	for _, file := range files {
		err := downloadFile(file, directory, name, match[1])
		if err != nil {
			log.Printf("Failed to download file: %s", err)
		}
	}

	return nil
}

func downloadFile(url string, directory string, name string, postID string) error {
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

func getAllPosts(url string) ([]string, error) {
	pages, err := numberOfPages(url)
	if err != nil {
		return nil, err
	}

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

		doc.Find("article.post-card").Each(func(i int, selection *goquery.Selection) {
			postUrl, _ := selection.Find("a").Attr("href")
			posts = append(posts, postUrl)
		})
	}

	return posts, nil
}

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

	postText := doc.Find("div.paginator small").Text()

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

	pageCount := (posts + 49) / 50

	return pageCount, nil
}

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
