// --- metadata.go ---
package main

import (
	"errors"
	"github.com/barasher/go-exiftool"
)

func ExtractMetada(path string) (map[string]interface{}, error) {
	et, err := exiftool.NewExiftool(); if err != nil { return nil, err }
	defer et.Close()
	infos := et.ExtractMetadata(path)
	if len(infos) > 0 { return infos[0].Fields, nil }
	return nil, errors.New("no metadata found for " + path)
}

