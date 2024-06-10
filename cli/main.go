package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/logrusorgru/aurora/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const version = "0.1.0"

var registryFilePath string

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
	log.Trace().Str("dst", dst).Msg(record.URI)

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

var errStopIteration = errors.New("stop iteration")


// listChangedFilesSinceCommit lists all files that have changed from the latest commit back to a specified commit
func listChangedFilesSinceCommit(repo *git.Repository, sinceCommitHash string) ([]string, error) {
	// Get the commit object for the specified commit hash
	sinceCommit, err := repo.CommitObject(plumbing.NewHash(sinceCommitHash))
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object for hash %s: %w", sinceCommitHash, err)
	}

	// Collect all changed files
	changedFiles := make(map[string]struct{})

	// Iterate through the commit history starting from HEAD
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit logs: %w", err)
	}

	err = commitIter.ForEach(func(commit *object.Commit) error {
		if commit.Hash == sinceCommit.Hash {
			return errStopIteration
		}

		// Get the parent commit
		parentCommit, err := commit.Parents().Next()
		if err != nil {
			return fmt.Errorf("failed to get parent commit: %w", err)
		}

		// Get the patch between the current commit and its parent
		patch, err := parentCommit.Patch(commit)
		if err != nil {
			return fmt.Errorf("failed to generate patch: %w", err)
		}

		// Extract the changed files from the patch
		for _, filePatch := range patch.FilePatches() {
			from, to := filePatch.Files()

			if from != nil && to != nil && from.Path() != to.Path() {
				// This is a rename operation
				delete(changedFiles, from.Path())
				changedFiles[to.Path()] = struct{}{}
			} else if to != nil {
				// This is an addition or modification
				changedFiles[to.Path()] = struct{}{}
			}
		}

		return nil
	})
	if err != nil && err != errStopIteration {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	// Convert the map keys to a slice
	var files []string
	for file := range changedFiles {
		files = append(files, file)
	}

	return files, nil
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

					// // get path from repo
					// repodir, err := repo.Worktree()
					// // to string
					// dst := repodir.Filesystem.Root()


					// print latest commit
					lastestHash, err := getLatestCommit(repo)
					if err != nil {
						log.Err(err).Msg("Failed to print latest commit")
					}

					if record.LastestHash == lastestHash {
						log.Debug().Str("uri", record.URI).Str("latest", lastestHash).Msg(aurora.BrightYellow("Skip").String())
						// no latest commit, skip
						continue
					}

					log.Debug().Str("uri", record.URI).Str("latest", lastestHash).Str("hash", record.LastestHash).Msg(aurora.BrightYellow("Update").String())

					commitHash := record.LastestHash
					// handle possible empty latest commit hash
					if commitHash == "" {
						commitHash = record.RootHash
					}

					// list commits since last processed commit
					files, err := listChangedFilesSinceCommit(repo, commitHash)
					if err != nil {
						log.Err(err).Msg("Failed to list files in latest commit")
						continue
					}

					// filter out files. remove *.json, *.yaml, *.yml, *.sum, *.mod
					files = filterFiles(files)

					PrintStruct(os.Stdout, files)


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
