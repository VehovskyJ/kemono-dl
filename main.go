package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type ProfileConfig struct {
	BaseURL string
	Service string
	UserID  string
}

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

type DetailedPostResponse struct {
	Post        map[string]interface{} `json:"post"`
	Attachments []interface{}          `json:"attachments"`
	Previews    []interface{}          `json:"previews"`
	Videos      []interface{}          `json:"videos"`
	Props       map[string]interface{} `json:"props"`
}

type ProfileResponse struct {
	Id         string      `json:"id"`
	Name       string      `json:"name"`
	Service    string      `json:"service"`
	Indexed    string      `json:"indexed"`
	Updated    string      `json:"updated"`
	PublicId   string      `json:"public_id"`
	RelationId interface{} `json:"relation_id"`
	PostCount  int         `json:"post_count"`
	DmCount    int         `json:"dm_count"`
	ShareCount int         `json:"share_count"`
	ChatCount  int         `json:"chat_count"`
}

func main() {
	// Define the force flag
	forceUpdate := flag.Bool("force", false, "Force update even if profile timestamp hasn't changed")

	// Customize the help message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: kemono-dl [options] <url>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Get the positional argument (URL)
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	inputURL := args[0]

	// Extract profile configuration from the provided URL
	profile, err := extractProfileConfig(inputURL)
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}

	log.Printf("Base URL: %s", profile.BaseURL)
	log.Printf("Service: %s", profile.Service)
	log.Printf("User ID: %s", profile.UserID)

	// Fetch profile to get post count
	profileData, err := fetchProfile(profile)
	if err != nil {
		log.Fatalf("Failed to fetch profile: %s", err)
	}

	log.Printf("Total posts: %d", profileData.PostCount)

	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %s", err)
	}

	// Check if profile folder exists and compare updated timestamps
	profileDir := filepath.Join(wd, profile.Service, profileData.Id)
	shouldUpdate, err := shouldUpdateProfile(profileDir, profileData, *forceUpdate)
	if err != nil {
		log.Fatalf("Failed to check profile status: %s", err)
	}

	if !shouldUpdate {
		log.Println("Nothing to download")
		return
	}

	// Save profile details
	err = saveProfile(wd, profile.Service, profileData)
	if err != nil {
		log.Fatalf("Failed to save profile: %s", err)
	}

	// Fetch posts from API
	posts, err := fetchPosts(profile)
	if err != nil {
		log.Fatalf("Failed to fetch posts: %s", err)
	}

	// Fetch and save detailed post data
	err = fetchAndSaveDetailedPosts(wd, profile, posts)
	if err != nil {
		log.Fatalf("Failed to save posts: %s", err)
	}

	log.Println("All posts saved successfully")
}

// fetchAndSaveDetailedPosts fetches detailed post data and saves it to disk
func fetchAndSaveDetailedPosts(baseDir string, profile *ProfileConfig, posts []Post) error {
	for _, post := range posts {
		log.Printf("Fetching detailed data for post: %s", post.Id)

		// Fetch detailed post data
		detailedPost, err := fetchDetailedPost(profile, post.Id)
		if err != nil {
			log.Printf("Warning: Failed to fetch detailed post %s: %s", post.Id, err)
			continue
		}

		// Save the detailed post data
		err = savePost(baseDir, profile.Service, profile.UserID, post.Id, detailedPost)
		if err != nil {
			log.Printf("Warning: Failed to save post %s: %s", post.Id, err)
			continue
		}

		log.Printf("Saved post: %s", post.Id)
	}

	return nil
}

// fetchDetailedPost fetches the detailed post data from the API
func fetchDetailedPost(profile *ProfileConfig, postID string) (*DetailedPostResponse, error) {
	// Construct the detailed post API URL
	apiURL := fmt.Sprintf("%s/api/v1/%s/user/%s/post/%s", profile.BaseURL, profile.Service, profile.UserID, postID)

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
		return nil, fmt.Errorf("failed to call post API: %w", err)
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

	// Unmarshal the JSON response into DetailedPostResponse
	var postResp DetailedPostResponse
	err = json.Unmarshal(body, &postResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &postResp, nil
}

// savePost saves the detailed post data to {service}/{user}/{post}/{post}.json
func savePost(baseDir string, service string, userID string, postID string, postData *DetailedPostResponse) error {
	// Create the directory structure: baseDir/service/userID/postID/
	postDir := filepath.Join(baseDir, service, userID, postID)

	err := os.MkdirAll(postDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create post directory: %w", err)
	}

	// Marshal the post struct to JSON with indentation
	jsonData, err := json.MarshalIndent(postData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal post data: %w", err)
	}

	// Write the JSON data to {postID}.json file
	postFilePath := filepath.Join(postDir, postID+".json")
	err = os.WriteFile(postFilePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write post JSON: %w", err)
	}

	return nil
}

// shouldUpdateProfile checks if profile folder exists and compares the updated timestamp
func shouldUpdateProfile(profileDir string, newProfile *ProfileResponse, forceUpdate bool) (bool, error) {
	// If force update is enabled, always update
	if forceUpdate {
		log.Println("Force update enabled, skipping timestamp check")
		return true, nil
	}

	// Check if profile directory exists
	_, err := os.Stat(profileDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Directory doesn't exist, so we should update
			return true, nil
		}
		return false, fmt.Errorf("failed to check profile directory: %w", err)
	}

	// Directory exists, find and read the profile JSON file
	files, err := os.ReadDir(profileDir)
	if err != nil {
		return false, fmt.Errorf("failed to read profile directory: %w", err)
	}

	var profileFile string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			profileFile = filepath.Join(profileDir, file.Name())
			break
		}
	}

	if profileFile == "" {
		// No profile JSON file found, should update
		return true, nil
	}

	// Read the existing profile JSON
	data, err := os.ReadFile(profileFile)
	if err != nil {
		return false, fmt.Errorf("failed to read profile file: %w", err)
	}

	var existingProfile ProfileResponse
	err = json.Unmarshal(data, &existingProfile)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal existing profile: %w", err)
	}

	// Compare the updated timestamps
	if existingProfile.Updated == newProfile.Updated {
		return false, nil
	}

	return true, nil
}

// fetchProfile retrieves the user profile and returns post count
func fetchProfile(profile *ProfileConfig) (*ProfileResponse, error) {
	// Construct the profile API URL
	apiURL := fmt.Sprintf("%s/api/v1/%s/user/%s/profile", profile.BaseURL, profile.Service, profile.UserID)

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
		return nil, fmt.Errorf("failed to call profile API: %w", err)
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

	// Unmarshal the JSON response into ProfileResponse
	var profileResp ProfileResponse
	err = json.Unmarshal(body, &profileResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &profileResp, nil
}

// fetchPosts calls the API and returns the posts
func fetchPosts(profile *ProfileConfig) ([]Post, error) {
	// Construct the API URL
	apiURL := fmt.Sprintf("%s/api/v1/%s/user/%s/posts", profile.BaseURL, profile.Service, profile.UserID)
	log.Printf("Fetching posts: %s", apiURL)

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

// saveProfile saves the profile details as a JSON file in the service/userID directory
func saveProfile(baseDir string, service string, profile *ProfileResponse) error {
	// Create the directory structure: baseDir/service/userID/
	profileDir := filepath.Join(baseDir, service, profile.Id)

	err := os.MkdirAll(profileDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}

	// Marshal the profile struct to JSON
	profileData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile data: %w", err)
	}

	// Write the JSON data to profile.json file (using the profile name/id as filename)
	profileFilePath := filepath.Join(profileDir, profile.Name+".json")
	err = os.WriteFile(profileFilePath, profileData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write profile JSON: %w", err)
	}

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
