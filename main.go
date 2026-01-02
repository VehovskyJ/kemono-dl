package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ProfileConfig holds the extracted profile information
type ProfileConfig struct {
	BaseURL string // e.g., "https://kemono.cr" or "https://coomer.st"
	Service string // e.g., "patreon", "onlyfans"
	UserID  string // e.g., "12345" or "username"
}

// Post represents a single post from the API response
type Post struct {
	Id        string `json:"id"`
	User      string `json:"user"`
	Service   string `json:"service"`
	Title     string `json:"title"`
	Substring string `json:"substring"`
	Published string `json:"published"`
	File      struct {
	} `json:"file"`
	Attachments []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"attachments"`
}

func main() {
	// Checks if URL was provided as an argument
	if len(os.Args) < 2 {
		log.Fatal("Please provide a url")
	}

	inputURL := os.Args[1]

	// Extract profile configuration from the provided URL
	profile, err := extractProfileConfig(inputURL)
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}

	log.Printf("Base URL: %s", profile.BaseURL)
	log.Printf("Service: %s", profile.Service)
	log.Printf("User ID: %s", profile.UserID)

	// Fetch posts from API
	posts, err := fetchPosts(profile)
	if err != nil {
		log.Fatalf("Failed to fetch posts: %s", err)
	}

	log.Printf("Retrieved %d posts", len(posts))

	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %s", err)
	}

	// Create folder structure and save post data
	for _, post := range posts {
		err := savePosts(wd, profile.Service, profile.UserID, &post)
		if err != nil {
			log.Printf("Failed to save post %s: %s", post.Id, err)
		}
	}

	log.Println("All posts saved successfully")
}

// ... existing extractProfileConfig function ...

// fetchPosts calls the API and returns the posts
func fetchPosts(profile *ProfileConfig) ([]Post, error) {
	// Construct the API URL
	apiURL := fmt.Sprintf("%s/api/v1/%s/user/%s/posts", profile.BaseURL, profile.Service, profile.UserID)
	log.Printf("Calling API: %s", apiURL)

	// Create a new HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set the required Accept header
	req.Header.Set("Accept", "text/css")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Unmarshal the JSON response into a slice of Post objects
	var posts []Post
	err = json.Unmarshal(body, &posts)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return posts, nil
}

// savePosts creates the folder structure and saves post data as JSON
func savePosts(baseDir string, service string, userID string, post *Post) error {
	// Create the directory structure: baseDir/service/userID/postID/
	postDir := filepath.Join(baseDir, service, userID, post.Id)

	err := os.MkdirAll(postDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create post directory: %w", err)
	}

	// Marshal the post struct to JSON
	postData, err := json.MarshalIndent(post, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal post data: %w", err)
	}

	// Write the JSON data to data.json file
	dataFilePath := filepath.Join(postDir, "data.json")
	err = os.WriteFile(dataFilePath, postData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write data.json: %w", err)
	}

	log.Printf("Saved post %s to %s", post.Id, postDir)

	return nil
}

// extractProfileConfig parses the profile URL and extracts base URL, service, and user ID
func extractProfileConfig(profileURL string) (*ProfileConfig, error) {
	// Parse the URL to validate it's a valid URL
	parsedURL, err := url.Parse(profileURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	// Remove query parameters from the path
	path := strings.Split(parsedURL.Path, "?")[0]
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	// Expected format: /{service}/user/{user_id}
	if len(pathParts) < 3 || pathParts[1] != "user" {
		return nil, errors.New("URL does not match expected format: /{service}/user/{user_id}")
	}

	service := pathParts[0]
	userID := pathParts[2]

	if service == "" || userID == "" {
		return nil, errors.New("service or user ID is empty")
	}

	// Reconstruct base URL (scheme + host)
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	return &ProfileConfig{
		BaseURL: baseURL,
		Service: service,
		UserID:  userID,
	}, nil
}
