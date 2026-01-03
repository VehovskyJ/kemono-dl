package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var DownloadTimeout = 2 * time.Minute

func main() {
	// Define flags
	forceUpdate := flag.Bool("force", false, "Force update even if profile timestamp hasn't changed")
	skipDownload := flag.Bool("skip-download", false, "Only fetch and save metadata, skip downloading files")
	timeoutStr := flag.String("timeout", "2m", "File download timeout (e.g. 10s, 2m, 5m)")

	// Customize the help message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: kemono-dl [options] <url>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Parse download timeout from flag
	t, err := time.ParseDuration(*timeoutStr)
	if err != nil {
		log.Fatalf("Invalid timeout: %v", err)
	}
	DownloadTimeout = t

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

	if *skipDownload {
		log.Printf("⏭️  Skip download mode enabled - will only fetch metadata")
	}

	// Fetch profile to get post count
	profileData, err := fetchProfile(profile)
	if err != nil {
		log.Fatalf("Failed to fetch profile: %s", err)
	}

	log.Printf("Total posts on profile: %d", profileData.PostCount)

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

	// Fetch posts from API with pagination
	posts, err := fetchPostsWithPagination(profile)
	if err != nil {
		log.Fatalf("Failed to fetch posts: %s", err)
	}

	// Fetch and save detailed post data
	err = fetchAndSaveDetailedPosts(wd, profile, posts, *skipDownload)
	if err != nil {
		log.Fatalf("Failed to save posts: %s", err)
	}

	log.Println("All posts saved successfully")
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
