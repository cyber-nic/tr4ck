package main

import (
	"encoding/csv"
	"os"
)

func loadRegistry() (*Registry, error) {
	reg := &Registry{Entries: make(map[string]string)}

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
			reg.Entries[record[1]] = record[0]
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

	for url, guid := range reg.Entries {
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

	reg.Entries[url] = guid
	return saveRegistry(reg)
}
