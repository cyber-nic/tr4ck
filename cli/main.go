package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/google/uuid"
	"github.com/logrusorgru/aurora/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)
	
var registryFilePath string

type Registry struct {
	Entries map[string]string
}

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get home directory")
	}

	registryFilePath = filepath.Join(homeDir, ".tr4ck_registry.json")
}


func cloneRepo(repoURL string) {
	tmpDir := filepath.Join(os.TempDir(), "tr4ck", "archives", uuid.New().String())

	repo, err := git.PlainClone(tmpDir, false, &git.CloneOptions{
		URL: repoURL,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to clone the repository")
		return
	}

	ref, err := repo.Head()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get HEAD reference")
		return
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get commit object")
		return
	}
	commitHash := commit.Hash.String()

	destDir := filepath.Join(os.TempDir(), "tr4ck", "archives", commitHash)
	log.Info().Str("src", repoURL).Str("tmp", tmpDir).Str("dst", destDir).Msg(aurora.Green("Cloning").String())

	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		log.Info().Str("dir", destDir).Msg("Directory already exists, performing re-sync")
		repo, err = git.PlainOpen(destDir)
		if err != nil {
			log.Error().Err(err).Msg("Failed to open existing repository")
			return
		}
		w, err := repo.Worktree()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get worktree")
			return
		}
		err = w.Pull(&git.PullOptions{RemoteName: "origin"})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			log.Error().Err(err).Msg("Failed to pull updates")
			return
		}
		log.Info().Msg("Repository re-synced successfully")
	} else {
		err = os.Rename(tmpDir, destDir)
		if err != nil {
			log.Error().Err(err).Msg("Failed to rename directory")
			return
		}
		log.Info().Str("dir", destDir).Msg("Repository cloned successfully")
	}

	err = updateRegistry(repoURL, commitHash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update registry")
	}
}


func main() {
	var rootCmd = &cobra.Command{Use: "app"}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("0.1.0")
		},
	}

	var registryCmd = &cobra.Command{
		Use:   "registry",
		Aliases: []string{"reg"},
		Short: "Manage registry entries",
	}

	var listCmd = &cobra.Command{
		Use:   "ls",
		Short: "List the registry entries",
		Run: func(cmd *cobra.Command, args []string) {
			reg, err := loadRegistry()
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to load registry")
			}

			for url, guid := range reg.Entries {
				fmt.Printf("%s -> %s\n", aurora.Green(guid), aurora.Blue(url))
			}
		},
	}

	registryCmd.AddCommand(listCmd)
	rootCmd.AddCommand(versionCmd, registryCmd)
	rootCmd.Execute()
}
	

// repoURL := "https://github.com/cyber-nic/tr4ck"
// cloneRepo(repoURL)
