package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// fetchProfile retrieves the user profile and returns post count
func fetchProfile(profile *ProfileConfig) (*ProfileResponse, error) {
	// Construct the profile API URL
	apiURL := fmt.Sprintf("%s/api/v1/%s/user/%s/profile", profile.BaseURL, profile.Service, profile.UserID)

	// Create new request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/css")

	// Fetch request
	rateLimiter.Wait()
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call profile API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var profileResp ProfileResponse
	if err := json.Unmarshal(body, &profileResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &profileResp, nil
}

// fetchPostsWithPagination fetches all posts with pagination support
func fetchPostsWithPagination(profile *ProfileConfig) ([]Post, error) {
	var allPosts []Post
	const pageSize = 50
	offset, pageNumber := 0, 1
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	for {
		apiURL := fmt.Sprintf("%s/api/v1/%s/user/%s/posts", profile.BaseURL, profile.Service, profile.UserID)
		if offset > 0 {
			apiURL = fmt.Sprintf("%s?o=%d", apiURL, offset)
		}

		log.Printf("Fetching page %d (offset=%d)", pageNumber, offset)

		var posts []Post
		var err error

		for attempt := 1; attempt <= maxRetries; attempt++ {
			posts, err = fetchPostsPage(apiURL)
			if err == nil {
				break
			}
			if attempt < maxRetries {
				log.Printf("⚠️  Page %d failed (attempt %d/%d): %v. Retrying in %.0fs...",
					pageNumber, attempt, maxRetries, err, retryDelay.Seconds())
				time.Sleep(retryDelay)
			}
		}

		if err != nil {
			log.Printf("❌ Page %d failed after %d attempts: %v", pageNumber, maxRetries, err)
			return nil, fmt.Errorf("failed to fetch page %d after %d retries: %w", pageNumber, maxRetries, err)
		}

		if len(posts) == 0 {
			log.Printf("✓ Page %d: No more posts available.", pageNumber)
			break
		}

		log.Printf("✓ Page %d successfully fetched (%d/%d posts)", pageNumber, len(posts), pageSize)
		allPosts = append(allPosts, posts...)

		if len(posts) < pageSize {
			log.Printf("✓ Fetching pages complete.")
			break
		}

		offset += pageSize
		pageNumber++
	}

	log.Printf("====================================")
	log.Printf("Total posts fetched: %d", len(allPosts))
	log.Printf("====================================")

	return allPosts, nil
}

// fetchPostsPage fetches a single page of posts from the API
func fetchPostsPage(apiURL string) ([]Post, error) {
	const maxRetries = 5
	const initialBackoff = time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		rateLimiter.Wait()

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "text/css")
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to call API: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			if attempt < maxRetries {
				backoffDuration := time.Duration(1<<uint(attempt-1)) * initialBackoff
				log.Printf("⚠️  Rate limited (429). Attempt %d/%d. Waiting %.0fs before retry...",
					attempt, maxRetries, backoffDuration.Seconds())
				time.Sleep(backoffDuration)
				continue
			}
			return nil, fmt.Errorf("rate limited after %d attempts", maxRetries)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var posts []Post
		if err := json.Unmarshal(body, &posts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}

		return posts, nil
	}

	return nil, fmt.Errorf("failed to fetch posts after %d attempts", maxRetries)
}

// fetchDetailedPost fetches the detailed post data from the API
func fetchDetailedPost(profile *ProfileConfig, postID string) (*DetailedPostResponse, error) {
	const maxRetries = 5
	const initialBackoff = time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		rateLimiter.Wait()

		apiURL := fmt.Sprintf("%s/api/v1/%s/user/%s/post/%s", profile.BaseURL, profile.Service, profile.UserID, postID)
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "text/css")
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to call post API: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			if attempt < maxRetries {
				backoffDuration := time.Duration(1<<uint(attempt-1)) * initialBackoff
				log.Printf("⚠️  Rate limited (429) while fetching post %s. Attempt %d/%d. Waiting %.0fs before retry...",
					postID, attempt, maxRetries, backoffDuration.Seconds())
				time.Sleep(backoffDuration)
				continue
			}
			return nil, fmt.Errorf("rate limited while fetching post %s after %d attempts", postID, maxRetries)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var postResp DetailedPostResponse
		if err := json.Unmarshal(body, &postResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
		}

		return &postResp, nil
	}

	return nil, fmt.Errorf("failed to fetch post %s after %d attempts", postID, maxRetries)
}
