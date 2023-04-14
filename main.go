package main

import (
	"errors"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
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

	_, err := getAllPosts(url)
	if err != nil {
		log.Fatalf("Failed to fetch all posts: %s", err)
	}
}

func getAllPosts(url string) ([]string, error) {
	posts, err := numberOfPages(url)
	if err != nil {
		return nil, err
	}

	return nil, nil
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
