package main

import (
	"fmt"
	"os"
	"path/filepath"

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

type Registry struct {
	Repos map[string]string
}

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get home directory")
	}

	registryFilePath = filepath.Join(homeDir, ".tr4ck_registry.json")
}

// cloneRepo clones a repository at a specific commit hash or syncs it to the latest state if it already exists.
func cloneRepo(commitHash, repoURL string) (string, error) {
	dst := filepath.Join(os.TempDir(), "tr4ck", "archives", commitHash)
	log.Debug().Str("src", repoURL).Str("dst", dst).Msg(aurora.BrightYellow(repoURL).String())

	// Check if the destination directory already exists
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		// If the repository exists, open it and pull the latest changes
		repo, err := git.PlainOpen(dst)
		if err != nil {
			return "", fmt.Errorf("failed to open existing repository: %w", err)
		}

		w, err := repo.Worktree()
		if err != nil {
			return "", fmt.Errorf("failed to get worktree: %w", err)
		}

		err = w.Pull(&git.PullOptions{RemoteName: "origin"})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return "", fmt.Errorf("failed to pull updates: %w", err)
		}

		// Checkout the specific commit
		hash := plumbing.NewHash(commitHash)
		err = w.Checkout(&git.CheckoutOptions{
			Hash: hash,
		})
		if err != nil {
			return "", fmt.Errorf("failed to checkout commit: %w", err)
		}

		return dst, nil
	}

	// If the repository does not exist, clone it
	repo, err := git.PlainClone(dst, false, &git.CloneOptions{
		Progress: 	  os.Stdout,
		URL:          repoURL,
		SingleBranch: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	// Checkout the specific commit
	w, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	hash := plumbing.NewHash(commitHash)
	err = w.Checkout(&git.CheckoutOptions{
		Hash: hash,
	})
	if err != nil {
		return "", fmt.Errorf("failed to checkout commit: %w", err)
	}

	return dst, nil
}

func printLatestCommit(dst string) (string, error) {
	repo, err := git.PlainOpen(dst)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

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


func getRepoGUIDFromFirstCommit(repoURL string) (string, error) {
	// Initialize a new in-memory repository
	storer := memory.NewStorage()
	repo, err := git.Init(storer, nil)
	if err != nil {
		return "", fmt.Errorf("failed to initialize repository: %v", err)
	}

	// Add a new remote with the given URL
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
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

	// tr4ck: improve default branch detection algorithm

	return nil, fmt.Errorf("failed to find default branch")
}

// func listRefs(repo *git.Repository	) error {
// 	refs, err := repo.References()
// 	if err != nil {
// 		return fmt.Errorf("failed to get references: %w", err)
// 	}

// 	err = refs.ForEach(func(ref *plumbing.Reference) error {
// 		fmt.Println(ref.Name(), ref.Hash())
// 		return nil
// 	})
// 	if err != nil {
// 		return fmt.Errorf("failed to iterate references: %w", err)
// 	}

// 	return nil
// }

func main() {
	var rootCmd = &cobra.Command{
		Use:   "sync",
		Short: "sync repos",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				reg, err := loadRegistry()
				if err != nil {
					fmt.Printf("failed to load registry")
					os.Exit(1)
				}

				for url, commitHash := range reg.Repos {
					dst, err := cloneRepo(commitHash, url)
					if err != nil {
						log.Err(err).Str("dir", dst).Msg("Failed to clone repository")
					}

					// print latest commit
					lastestCommitHash, err := printLatestCommit(dst); 
					if err != nil {
						log.Err(err).Msg("Failed to print latest commit")
					}

					fmt.Printf("Latest commit hash: %s\n", lastestCommitHash)

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

			for url, commitHash := range reg.Repos {
				fmt.Printf("%s -> %s\n", aurora.Green(commitHash), aurora.Blue(url))
			}
		},
	}

	var addCmd = &cobra.Command{
		Use:   "add [url]",
		Short: "Add URL to the registry",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			url := args[0]
			err := addToRegistry(url)
			if err != nil {
				fmt.Printf("Failed to add URL to the registry: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("URL %s added to the registry\n", url)
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

// repoURL := "https://github.com/cyber-nic/tr4ck"
// cloneRepo(repoURL)
