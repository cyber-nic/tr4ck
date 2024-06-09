package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
)

func loadRegistry() (*Registry, error) {
	reg := &Registry{Repos: make(map[string]string)}

	// read registry file
	if _, err := os.Stat(registryFilePath); os.IsNotExist(err) {
		fmt.Printf("Registry file %s does not exist\n", registryFilePath)
		os.Exit(1)
	} else if err != nil {
		fmt.Printf("Error checking registry file %s: %v\n", registryFilePath, err)
		os.Exit(1)
	}

	file, err := os.Open(registryFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if len(record) == 2 {
			reg.Repos[record[1]] = record[0]
		}
	}

	return reg, nil
}

func saveRegistry(reg *Registry) error {
	file, err := os.Create(registryFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Comma = '\t'
	defer writer.Flush()

	for url, guid := range reg.Repos {
		err := writer.Write([]string{guid, url})
		if err != nil {
			return err
		}
	}

	return nil
}

func updateRegistry(url, guid string) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	reg.Repos[url] = guid
	return saveRegistry(reg)
}

func addToRegistry(url string) error {
	// Open the registry file in read-write mode
	file, err := os.OpenFile(registryFilePath, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check if the URL already exists
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if scanner.Text() == url {
			return fmt.Errorf("URL %s already exists in the registry", url)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	commitHash, err := getRepoGUIDFromFirstCommit(url)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %v", err)
	}

	err = updateRegistry(url, commitHash)
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
