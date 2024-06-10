package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

// RegistryRecord represents a record in the registry file. It contains the root hash, the latest hash, and the URI of the repository being tracked.
type RegistryRecord struct {
	RootHash    string
	LastestHash string
	URI         string
	// tr@ck: also track the branch
}

func loadRegistry() (*[]RegistryRecord, error) {
	file, err := os.Open(registryFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open registry file: %w", err)
	}
	defer file.Close()

	var records []RegistryRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		// invalid line
		if len(parts) > 3 {
			return nil, fmt.Errorf("invalid registry entry: %s", line)
		}

		// uri only
		if len(parts) == 1 {
			// tr@ck: validate git uri format. can be url or path
			uri := strings.Trim(line, " ")
			records = append(records, RegistryRecord{URI: uri})
			continue
		}

		// uri and root hash
		if len(parts) == 2 {
			// tr@ck: validate git uri format. can be url or path
			// tr@ck: validate commit hash format
			commitHash := parts[0]
			uri := strings.Join(parts[1:], " ") // Join the remaining parts to form the URL
			records = append(records, RegistryRecord{URI: uri, RootHash: commitHash})
			continue
		}

		// complete record
		commitHash := parts[0]
		lastProcessedCommit := parts[1]
		uri := strings.Join(parts[2:], " ") // Join the remaining parts to form the URL
		record := RegistryRecord{
			RootHash:    commitHash,
			LastestHash: lastProcessedCommit,
			URI:         uri,
		}
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading registry file: %w", err)
	}

	// PrintStruct(os.Stdout, records)

	return &records, nil
}

func appendToRegistry(record *RegistryRecord) error {
	file, err := os.OpenFile(registryFilePath, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open registry file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, record.URI) {
			return fmt.Errorf("URL %s already exists in the registry", record.URI)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading registry file: %w", err)
	}

	writer := bufio.NewWriter(file)
	_, err = writer.WriteString(fmt.Sprintf("%s    %s    %s\n", record.RootHash, record.LastestHash, record.URI))
	if err != nil {
		return fmt.Errorf("failed to write to registry file: %w", err)
	}
	return writer.Flush()
}

// updateRegistry updates a registry record for a given URI
func updateRegistry(rec RegistryRecord) error {
	records, err := loadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	updated := false
	for i, record := range *records {
		if record.URI == rec.URI {
			(*records)[i] = RegistryRecord{
				RootHash:    rec.RootHash,
				LastestHash: rec.LastestHash,
				URI:         rec.URI,
			}
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("URI %s not found in the registry", rec.URI)
	}

	file, err := os.OpenFile(registryFilePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open registry file for writing: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, record := range *records {
		_, err = writer.WriteString(fmt.Sprintf("%s    %s    %s\n", record.RootHash, record.LastestHash, record.URI))
		if err != nil {
			return fmt.Errorf("failed to write to registry file: %w", err)
		}
	}
	return writer.Flush()
}




// addToRegistry adds the given URI to the registry
func addToRegistry(uri string) error {
	// Open the registry file in read-write mode
	file, err := os.OpenFile(registryFilePath, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check if the URI already exists
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, uri) {
			return fmt.Errorf("URI %s already exists in the registry", uri)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	commitHash, err := getRootHashFromFirstCommit(uri)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %v", err)
	}

	log.Debug().Str("uri", uri).Str("commitHash", commitHash).Msg("Adding")

	err = appendToRegistry(&RegistryRecord{
		RootHash:    commitHash,
		LastestHash: commitHash,
		URI:         uri,
	})
	if err != nil {
		return fmt.Errorf("failed to update registry: %v", err)
	}

	return nil
}

func initRegistry() {
	// read registry file
	_, err := os.Stat(registryFilePath)
	if os.IsNotExist(err) {
		file, err := os.Create(registryFilePath)
		if err != nil {
			fmt.Printf("Error creating registry file %s: %v\n", registryFilePath, err)
			os.Exit(1)
		}
		defer file.Close()
		fmt.Printf("Registry file %s created\n", registryFilePath)
	} else {
		fmt.Printf("Registry file %s already exists\n", registryFilePath)
	}
}
