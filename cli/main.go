package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/logrusorgru/aurora/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

var registryFilePath string
var markers = []string{"tr@ck", "todo"}

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get home directory")
	}

	registryFilePath = filepath.Join(homeDir, ".tr4ck.registry")
}

// cloneRepo clones a repository at a specific commit hash or syncs it to the latest state if it already exists.
func cloneRepo(record *RegistryRecord) (*git.Repository, error) {
	dst := filepath.Join(os.TempDir(), "tr4ck", "archives", record.RootHash)

	// Check if the destination directory already exists
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		// If the repository exists, open it and pull the latest changes
		repo, err := git.PlainOpen(dst)
		if err != nil {
			return nil, fmt.Errorf("failed to open existing repository: %w", err)
		}

		w, err := repo.Worktree()
		if err != nil {
			return nil, fmt.Errorf("failed to get worktree: %w", err)
		}

		err = w.Pull(&git.PullOptions{RemoteName: "origin"})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return nil, fmt.Errorf("failed to pull updates: %w", err)
		}

		// Checkout the specific commit
		hash := plumbing.NewHash(record.RootHash)
		err = w.Checkout(&git.CheckoutOptions{
			Hash: hash,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to checkout commit: %w", err)
		}

		return repo, nil
	}

	// If the repository does not exist, clone it
	repo, err := git.PlainClone(dst, false, &git.CloneOptions{
		// Progress:     os.Stdout,
		URL:          record.URI,
		SingleBranch: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Checkout the specific commit
	w, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	hash := plumbing.NewHash(record.RootHash)
	err = w.Checkout(&git.CheckoutOptions{
		Hash: hash,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to checkout commit: %w", err)
	}

	return repo, nil
}

func getLatestCommit(repo *git.Repository) (string, error) {
	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get latest commit: %w", err)
	}

	return commit.Hash.String(), nil
}

// listChangedFilesSinceCommit lists all files that have changed between two commits
func listChangedFilesSinceCommit(repo *git.Repository, oldCommitHash, newCommitHash string) ([]string, []string, error) {
	// Get the commit objects for the specified commit hashes
	oldCommit, err := repo.CommitObject(plumbing.NewHash(oldCommitHash))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get commit object for old hash %s: %w", oldCommitHash, err)
	}

	newCommit, err := repo.CommitObject(plumbing.NewHash(newCommitHash))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get commit object for new hash %s: %w", newCommitHash, err)
	}

	// Get the patch between the two commits
	patch, err := oldCommit.Patch(newCommit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate patch: %w", err)
	}

	// Extract the changed and removed files from the patch
	changedFiles := make(map[string]struct{})
	removedFiles := make(map[string]struct{})

	for _, filePatch := range patch.FilePatches() {
		from, to := filePatch.Files()

		if from != nil && to != nil && from.Path() != to.Path() {
			// This is a rename operation
			delete(changedFiles, from.Path())
			changedFiles[to.Path()] = struct{}{}
			log.Trace().Str("from", from.Path()).Str("to", to.Path()).Msg("rename")
		} else if to != nil {
			// This is an addition or modification
			changedFiles[to.Path()] = struct{}{}
			log.Trace().Str("to", to.Path()).Msg("add")
		} else if from != nil {
			// This is a deletion
			removedFiles[from.Path()] = struct{}{}
			log.Trace().Str("from", from.Path()).Msg("delete")
		}
	}

	// Convert the map keys to slices
	var changed []string
	for file := range changedFiles {
		changed = append(changed, file)
	}

	var removed []string
	for file := range removedFiles {
		removed = append(removed, file)
	}

	// filter out files. remove *.json, *.yaml, *.yml, *.sum, *.mod
	filteredChanged := filterFiles(changed)
	filteredRemoved := filterFiles(removed)

	return filteredChanged, filteredRemoved, nil
}

func getRootHashFromFirstCommit(repoURI string) (string, error) {
	// Initialize a new in-memory repository
	storer := memory.NewStorage()
	repo, err := git.Init(storer, nil)
	if err != nil {
		return "", fmt.Errorf("failed to initialize repository: %v", err)
	}

	// Add a new remote with the given URI
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURI},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create remote: %v", err)
	}

	// Fetch the very first commit
	fetchOptions := &git.FetchOptions{
		RemoteName: "origin",
		Depth:      1,
		RefSpecs:   []config.RefSpec{"refs/heads/*:refs/heads/*"},
	}
	err = repo.Fetch(fetchOptions)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", fmt.Errorf("failed to fetch the repository: %v", err)
	}

	ref, err := findDefaultRef(repo)
	if err != nil {
		return "", fmt.Errorf("failed to find default branch: %v", err)
	}

	return ref.Hash().String(), nil
}

func findDefaultRef(repo *git.Repository) (*plumbing.Reference, error) {
	// Get the reference to the fetched commit
	ref, err := repo.Reference(plumbing.ReferenceName("refs/heads/main"), true)
	if err == nil {
		return ref, nil
	}

	ref, err = repo.Reference(plumbing.ReferenceName("refs/heads/master"), true)
	if err == nil {
		return ref, nil
	}

	// tr@ck: improve default branch detection algorithm

	return nil, fmt.Errorf("failed to find default branch")
}

// removes file types for which tracking is disabled
func filterFiles(files []string) []string {
	var filtered []string
	for _, file := range files {
		ext := filepath.Ext(file)
		if ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".sum" || ext == ".mod" {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

// containsMarker checks if a file contains any of the specified markers
func containsMarker(filePath string, markers []string) (bool, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		for _, marker := range markers {
			if strings.Contains(line, marker) {
				return true, marker, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return false, "", fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	return false, "", nil
}

// listFilesWithMarkersSinceCommit lists files that contain any markers and have changed since the specified commit
func listFilesWithMarkersSinceCommit(repo *git.Repository, firstHash, latestHash string, markers []string) ([]string, []string, error) {
	changedFiles, removedFiles, err := listChangedFilesSinceCommit(repo, firstHash, latestHash)
	if err != nil {
		return nil, nil, err
	}

	w, err := repo.Worktree()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	var filesWithMarkers []string
	for _, file := range changedFiles {
		absFilePath := filepath.Join(w.Filesystem.Root(), file)
		hit, mark, err := containsMarker(absFilePath, markers)
		if err != nil {
			return nil, nil, err
		}
		if hit {
			log.Trace().Str("file",file).Str("marker", mark).Msg(aurora.BrightGreen("tr4ck").String())
			filesWithMarkers = append(filesWithMarkers, file)
		}
	}

	return filesWithMarkers, removedFiles, nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "sync",
		Short: "sync repos",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				registry, err := loadRegistry()
				if err != nil {
					fmt.Printf("failed to load registry\n")
					os.Exit(1)
				}

				for _, record := range *registry {
					repo, err := cloneRepo(&record)
					if err != nil {
						log.Err(err).Str("uri", record.URI).Msg("Failed to clone repository")
					}

					// latest commit
					lastestHash, err := getLatestCommit(repo)
					if err != nil {
						log.Err(err).Msg("Failed to get latest commit")
					}

					if record.LastestHash == lastestHash {
						log.Debug().Str("uri", record.URI).Str("latest", lastestHash).Msg(aurora.BrightYellow("Skip").String())
						// no latest commit, skip
						continue
					}

					firstHash := record.LastestHash
					// handle possible empty latest commit hash
					if firstHash == "" {
						firstHash = record.RootHash
					}

					// list commits since last processed commit
					changed, removed, err := listFilesWithMarkersSinceCommit(repo, firstHash, lastestHash, markers)
					if err != nil {
						log.Err(err).Msg("Failed to list files in latest commit")
						continue
					}

					if changed == nil && removed == nil {
						log.Debug().Str("uri", record.URI).Str("latest", lastestHash).Msg(aurora.BrightYellow("Skip").String())
						// update registry
						record.LastestHash = lastestHash
						if err = updateRegistry(record); err != nil {
							log.Err(err).Msg("Failed to update registry")
						}

						// no changed files, skip
						continue
					}

					log.Debug().Int("changed", len(changed)).Int("removed", len(removed)).Str("uri", record.URI).Str("latest", lastestHash).Str("hash", record.LastestHash).Msg(aurora.BrightYellow("Update").String())

					// // update registry
					// record.LastestHash = lastestHash
					// if err = updateRegistry(record); err != nil {
					// 	log.Err(err).Msg("Failed to update registry")
					// }

				}
			}
		},
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}

	var registryCmd = &cobra.Command{
		Use:     "registry",
		Aliases: []string{"reg"},
		Short:   "Manage registry entries",
	}

	var listCmd = &cobra.Command{
		Use:   "ls",
		Short: "List the registry entries",
		Run: func(cmd *cobra.Command, args []string) {
			reg, err := loadRegistry()
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to load registry")
			}

			for _, record := range *reg {
				fmt.Printf("%s	%s	%s\n", aurora.Green(record.RootHash), record.LastestHash, aurora.Blue(record.URI))
			}
		},
	}

	var addCmd = &cobra.Command{
		Use:   "add [uri]",
		Short: "Add URI to the registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			uri := args[0]
			err := addToRegistry(uri)
			if err != nil {
				fmt.Printf("Failed to add URI to the registry: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("URI %s added to the registry\n", uri)
		},
	}

	var initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize registry file",
		Run: func(cmd *cobra.Command, args []string) {
			initRegistry()
		},
	}

	registryCmd.AddCommand(addCmd, listCmd)
	rootCmd.AddCommand(versionCmd, initCmd, registryCmd)
	rootCmd.Execute()
}
