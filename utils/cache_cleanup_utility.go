package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println("DOST Cache Cleanup Utility")
	
	// Backup the original file
	if err := copyFile("./.dost/cache.txt", "./.dost/cache_backup.txt"); err != nil {
		fmt.Printf("Warning: Could not create backup: %v\n", err)
	} else {
		fmt.Println("✓ Backup created: ./.dost/cache_backup.txt")
	}
	
	// Clean up duplicates
	if err := removeDuplicates("./.dost/cache.txt"); err != nil {
		fmt.Printf("Error cleaning cache: %v\n", err)
		return
	}
	
	fmt.Println("✓ Cache cleanup completed successfully!")
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	
	return os.WriteFile(dst, input, 0644)
}

func removeDuplicates(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	var uniqueLines []string
	seenLines := make(map[string]bool)
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		// Validate JSON and normalize it
		var jsonObj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &jsonObj); err != nil {
			fmt.Printf("Skipping invalid JSON line: %s\n", line[:min(50, len(line))])
			continue
		}
		
		// Re-marshal to get consistent formatting
		normalizedJSON, err := json.Marshal(jsonObj)
		if err != nil {
			continue
		}
		
		normalizedLine := string(normalizedJSON)
		if !seenLines[normalizedLine] {
			seenLines[normalizedLine] = true
			uniqueLines = append(uniqueLines, normalizedLine)
		}
	}
	
	if err := scanner.Err(); err != nil {
		return err
	}
	
	// Write cleaned lines back to file
	newFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer newFile.Close()
	
	for _, line := range uniqueLines {
		if _, err := newFile.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	
	fmt.Printf("✓ Removed %d duplicate entries\n", scanner.Err())
	fmt.Printf("✓ Kept %d unique entries\n", len(uniqueLines))
	
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
