package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// fetchAndSaveDetailedPosts fetches detailed post data and saves it to disk
func AppendFailedDownload(baseDir, service, userID, url string) error {
	failedDir := filepath.Join(baseDir, service, userID)
	failedFile := filepath.Join(failedDir, "failed.json")
	var failedList []string

	// Try to read existing failed.json
	data, err := os.ReadFile(failedFile)
	if err == nil {
		_ = json.Unmarshal(data, &failedList)
	}

	// Add only if not already present
	for _, u := range failedList {
		if u == url {
			return nil
		}
	}
	failedList = append(failedList, url)

	out, _ := json.MarshalIndent(failedList, "", "  ")
	if err := os.WriteFile(failedFile, out, 0644); err != nil {
		return fmt.Errorf("failed to write failed.json: %w", err)
	}
	return nil
}

func fetchAndSaveDetailedPosts(baseDir string, profile *ProfileConfig, posts []Post, skipDownload bool) error {
	totalPosts := len(posts)
	log.Printf("Processing %d posts", totalPosts)

	for idx, post := range posts {
		log.Printf("[%d/%d] Fetching detailed data for post: %s", idx+1, totalPosts, post.Id)

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

		log.Printf("Saved post metadata: %s", post.Id)

		// Skip downloading files if flag is set
		if skipDownload {
			log.Printf("⏭️  Skipping file download for post %s (skip-download mode)", post.Id)
			continue
		}

		// Download post file
		err = downloadPostFile(baseDir, profile.Service, profile.UserID, post.Id, detailedPost, profile)
		if err != nil {
			log.Printf("Warning: Failed to download file for post %s: %s", post.Id, err)
		}

		// Download attachments from the post
		err = downloadPostAttachments(baseDir, profile.Service, profile.UserID, post.Id, detailedPost, profile)
		if err != nil {
			log.Printf("Warning: Failed to download attachments for post %s: %s", post.Id, err)
			continue
		}
	}

	return nil
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

// downloadPostFile downloads the main file from post.file field
func downloadPostFile(baseDir string, service string, userID string, postID string, detailedPost *DetailedPostResponse, profile *ProfileConfig) error {
	// Extract file from the post field
	postFile, ok := detailedPost.Post["file"]
	if !ok {
		log.Printf("No file field in post %s", postID)
		return nil
	}

	fileMap, ok := postFile.(map[string]interface{})
	if !ok {
		log.Printf("File field is not a valid object in post %s", postID)
		return nil
	}

	// Check if file has name and path
	name, nameOk := fileMap["name"].(string)
	path, pathOk := fileMap["path"].(string)
	if !nameOk || !pathOk {
		log.Printf("File missing name or path in post %s", postID)
		return nil
	}

	// If name is empty, skip downloading
	if name == "" {
		log.Printf("File name is empty for post %s", postID)
		return nil
	}

	// Download to post directory directly
	postDir := filepath.Join(baseDir, service, userID, postID)

	err := downloadFileFromPath(postDir, name, path, profile.BaseURL)
	if err != nil {
		// Save failed URL to failed.json
		_ = AppendFailedDownload(baseDir, service, userID, profile.BaseURL+path)
		return fmt.Errorf("failed to download file %s: %w", name, err)
	}

	log.Printf("Downloaded file: %s", name)
	return nil
}

// ProgressWriter wraps io.Writer to track download progress
type ProgressWriter struct {
	fileName       string
	totalSize      int64
	downloadedSize int64
	lastLogTime    time.Time
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.downloadedSize += int64(n)

	// Log progress every second
	now := time.Now()
	if now.Sub(pw.lastLogTime) >= time.Second || pw.downloadedSize == pw.totalSize {
		pw.lastLogTime = now
		if pw.totalSize > 0 {
			percentage := float64(pw.downloadedSize) / float64(pw.totalSize) * 100
			downloadedMB := float64(pw.downloadedSize) / 1024 / 1024
			totalMB := float64(pw.totalSize) / 1024 / 1024
			log.Printf("[%s] Progress: %.1f%% (%.2f MB / %.2f MB)",
				pw.fileName, percentage, downloadedMB, totalMB)
		} else {
			downloadedMB := float64(pw.downloadedSize) / 1024 / 1024
			log.Printf("[%s] Downloaded: %.2f MB (unknown total size)",
				pw.fileName, downloadedMB)
		}
	}

	return n, nil
}

// downloadFileFromPath downloads a file using the base URL and file path with progress tracking
func downloadFileFromPath(destDir string, fileName string, filePath string, baseURL string) error {
	// Check if file already exists
	outputPath := filepath.Join(destDir, fileName)
	if _, err := os.Stat(outputPath); err == nil {
		log.Printf("File already exists, skipping: %s", fileName)
		return nil
	}

	maxRetries := 5
	initialBackoff := time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Apply rate limiting before making the request
		rateLimiter.Wait()

		// Construct full download URL using base URL
		downloadURL := baseURL + filePath

		// Create request
		req, err := http.NewRequest("GET", downloadURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request for %s: %w", fileName, err)
		}

		// Set User-Agent header
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		// Create HTTP client with timeout
		client := &http.Client{
			Timeout: 2 * time.Minute,
		}

		log.Printf("Starting download: %s", fileName)
		startTime := time.Now()

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", fileName, err)
		}

		// Check for rate limit error (429 Too Many Requests)
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()

			if attempt < maxRetries {
				backoffDuration := time.Duration(1<<uint(attempt-1)) * initialBackoff
				log.Printf("⚠️  Rate limited (429) while downloading %s. Attempt %d/%d. Waiting %.0fs before retry...",
					fileName, attempt, maxRetries, backoffDuration.Seconds())
				time.Sleep(backoffDuration)
				continue
			}

			return fmt.Errorf("rate limited while downloading %s after %d attempts", fileName, maxRetries)
		}

		// Check HTTP status code
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("download failed for %s with status %d", fileName, resp.StatusCode)
		}

		// Create output file
		outputPath := filepath.Join(destDir, fileName)
		file, err := os.Create(outputPath)
		if err != nil {
			resp.Body.Close()
			return fmt.Errorf("failed to create file %s: %w", fileName, err)
		}

		// Get total file size from Content-Length header
		totalSize := resp.ContentLength

		// Create progress writer
		progressWriter := &ProgressWriter{
			fileName:    fileName,
			totalSize:   totalSize,
			lastLogTime: startTime,
		}

		// Copy with progress tracking
		_, err = io.Copy(file, io.TeeReader(resp.Body, progressWriter))
		file.Close()
		resp.Body.Close()

		if err != nil {
			os.Remove(outputPath) // Clean up incomplete file
			return fmt.Errorf("failed to write file %s: %w", fileName, err)
		}

		// Log completion with speed
		duration := time.Since(startTime)
		speedMBs := float64(progressWriter.downloadedSize) / 1024 / 1024 / duration.Seconds()
		log.Printf("Completed download: %s (%.2f MB in %.1fs at %.2f MB/s)",
			fileName, float64(progressWriter.downloadedSize)/1024/1024, duration.Seconds(), speedMBs)

		return nil
	}

	return fmt.Errorf("failed to download %s after %d attempts", fileName, maxRetries)
}

// downloadPostAttachments downloads all attachments from the post.attachments field
func downloadPostAttachments(baseDir string, service string, userID string, postID string, detailedPost *DetailedPostResponse, profile *ProfileConfig) error {
	// Extract attachments from the post field
	postData, ok := detailedPost.Post["attachments"]
	if !ok {
		log.Printf("No attachments field in post %s", postID)
		return nil
	}

	attachmentsList, ok := postData.([]interface{})
	if !ok {
		log.Printf("Attachments field is not a list in post %s", postID)
		return nil
	}

	if len(attachmentsList) == 0 {
		log.Printf("No attachments found in post %s", postID)
		return nil
	}

	// Download to post directory directly
	postDir := filepath.Join(baseDir, service, userID, postID)

	// Download each attachment
	for _, attachment := range attachmentsList {
		attachmentMap, ok := attachment.(map[string]interface{})
		if !ok {
			log.Printf("Attachment is not a valid object in post %s", postID)
			continue
		}

		name, nameOk := attachmentMap["name"].(string)
		path, pathOk := attachmentMap["path"].(string)
		if !nameOk || !pathOk {
			log.Printf("Attachment missing name or path in post %s", postID)
			continue
		}

		// Download the attachment to post directory
		err := downloadFileFromPath(postDir, name, path, profile.BaseURL)
		if err != nil {
			// Save failed URL to failed.json
			_ = AppendFailedDownload(baseDir, service, userID, profile.BaseURL+path)
			log.Printf("Warning: Failed to download attachment %s for post %s: %s", name, postID, err)
			continue
		}

		log.Printf("Downloaded attachment: %s", name)
	}

	return nil
}
