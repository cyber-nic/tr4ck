package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	"github.com/google/uuid"
)

// cloneRepo clones a git repository and uses the HEAD commit hash for the directory name.
func cloneRepo(repoURL string) {
	// Generate a temporary directory for initial clone
	tmpDir := filepath.Join(os.TempDir(), "tr4ck", "archives", uuid.New().String())

	// Clone the repository
	repo, err := git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL: repoURL,
	})
	if err != nil {
		log.Fatalf("Failed to clone the repository: %s", err)
	}

	// Get the HEAD commit hash
	ref, err := repo.Head()
	if err != nil {
		log.Fatalf("Failed to get HEAD reference: %s", err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		log.Fatalf("Failed to get commit object: %s", err)
	}
	commitHash := commit.Hash.String()

	// Define the final destination directory path
	destDir := filepath.Join(os.TempDir(), "tr4ck", "archives", commitHash)

	// Check if the destination directory already exists
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		log.Fatalf("Destination directory %s already exists", destDir)
	}

	// Rename the temporary directory to the final destination
	err = os.Rename(tmpDir, destDir)
	if err != nil {
		log.Fatalf("Failed to rename directory: %s", err)
	}

	fmt.Printf("Repository cloned to %s\n", destDir)
}

func main() {
	// Example repository URL
	repoURL := "https://github.com/cyber-nic/tr4ck"

	// Clone the repository
	cloneRepo(repoURL)
}
