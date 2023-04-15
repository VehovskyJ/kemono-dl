package main

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
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
