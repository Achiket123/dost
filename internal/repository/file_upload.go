package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// FileUploadResponse represents the Gemini File API response
type FileUploadResponse struct {
	File struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		MimeType    string `json:"mimeType"`
		SizeBytes   string `json:"sizeBytes"`
		CreateTime  string `json:"createTime"`
		UpdateTime  string `json:"updateTime"`
		URI         string `json:"uri"`
		State       string `json:"state"`
	} `json:"file"`
}

// FileCacheEntry stores cached file upload info
type FileCacheEntry struct {
	FileURI     string    `json:"file_uri"`
	ContentHash string    `json:"content_hash"`
	UploadedAt  time.Time `json:"uploaded_at"`
	FilePath    string    `json:"file_path"`
}

const cacheDir = ".dost_cache"
const cacheFile = "file_cache.json"
const cacheMaxAge = 47 * time.Hour // Gemini keeps files for 48h, refresh at 47h

// hashFile computes SHA-256 hash of a file
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// loadCache reads the local file cache
func loadFileCache() (*FileCacheEntry, error) {
	data, err := os.ReadFile(filepath.Join(cacheDir, cacheFile))
	if err != nil {
		return nil, err
	}
	var entry FileCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// saveCache writes the local file cache
func saveFileCache(entry *FileCacheEntry) error {
	os.MkdirAll(cacheDir, 0755)
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, cacheFile), data, 0644)
}

// UploadFile uploads a file to the Gemini File API and returns the file URI.
func UploadFile(filePath, apiKey string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Detect MIME type
	ext := filepath.Ext(filePath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "text/plain"
	}

	// Build the upload URL
	uploadURL := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/upload/v1beta/files?key=%s",
		apiKey,
	)

	// Create multipart request — Gemini expects raw file body with metadata headers
	req, err := http.NewRequest("POST", uploadURL, file)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	displayName := filepath.Base(filePath)
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("X-Goog-Upload-Protocol", "raw")
	req.Header.Set("X-Goog-Upload-Display-Name", displayName)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var uploadResp FileUploadResponse
	if err := json.Unmarshal(body, &uploadResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if uploadResp.File.URI == "" {
		return "", fmt.Errorf("no file URI in response: %s", string(body))
	}

	// Check if already ACTIVE (small files are often ACTIVE immediately)
	if uploadResp.File.State != "ACTIVE" && uploadResp.File.Name != "" {
		if err := waitForFileActive(uploadResp.File.Name, apiKey); err != nil {
			return "", fmt.Errorf("file processing failed: %w", err)
		}
	}

	return uploadResp.File.URI, nil
}

// FileStatusResponse represents the GET /files/{id} response (flat, not wrapped)
type FileStatusResponse struct {
	Name  string `json:"name"`
	URI   string `json:"uri"`
	State string `json:"state"`
}

// waitForFileActive polls the file status until it becomes ACTIVE
func waitForFileActive(fileName, apiKey string) error {
	checkURL := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/%s?key=%s",
		fileName, apiKey,
	)

	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < 30; i++ { // Max 60 seconds (30 * 2s)
		resp, err := client.Get(checkURL)
		if err != nil {
			return fmt.Errorf("failed to check file status: %w", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var fileResp FileStatusResponse
		if err := json.Unmarshal(body, &fileResp); err != nil {
			return fmt.Errorf("failed to parse file status: %w", err)
		}

		if fileResp.State == "ACTIVE" {
			return nil
		}
		if fileResp.State == "FAILED" {
			return fmt.Errorf("file processing failed")
		}

		fmt.Printf("⏳ File processing (%s)...\n", fileResp.State)
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("file processing timed out")
}

// GetCachedFileURI returns a cached file URI if available, otherwise uploads and caches.
// It re-uploads only if the file content has changed or the cached URI has expired.
func GetCachedFileURI(filePath, apiKey string) (string, error) {
	// Hash the current file
	currentHash, err := hashFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	// Check cache
	cached, err := loadFileCache()
	if err == nil && cached.ContentHash == currentHash && cached.FilePath == filePath {
		// Check if URI is still valid (less than 47 hours old)
		if time.Since(cached.UploadedAt) < cacheMaxAge {
			fmt.Println("📎 Using cached context file (no changes detected)")
			return cached.FileURI, nil
		}
		fmt.Println("📎 Context cache expired, re-uploading...")
	} else if err == nil && cached.ContentHash != currentHash {
		fmt.Println("📎 Context changed, re-uploading...")
	}

	// Upload the file
	fmt.Printf("📤 Uploading context file: %s\n", filepath.Base(filePath))
	fileURI, err := UploadFile(filePath, apiKey)
	if err != nil {
		return "", err
	}
	fmt.Println("✅ Context file uploaded successfully")

	// Save to cache
	cacheEntry := &FileCacheEntry{
		FileURI:     fileURI,
		ContentHash: currentHash,
		UploadedAt:  time.Now(),
		FilePath:    filePath,
	}
	if err := saveFileCache(cacheEntry); err != nil {
		fmt.Printf("⚠️  Warning: could not save cache: %v\n", err)
	}

	return fileURI, nil
}
