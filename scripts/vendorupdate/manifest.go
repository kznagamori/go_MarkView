package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// 本ファイルは vendor.json（BR-042）の読み書きを持つ。
//
// main.go が 400 行の目安（IMP-011）を超えたため分けた。

// entry は vendor.json の 1 件（IMP-181 の VendorEntry と同じ形）。
//
// **buildinfo を import しない。** あちらは埋め込み済みの JSON を読む側で
// あり、書く側がその都合に縛られる理由がない（IMP-012）。
type entry struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	SPDX      string `json:"spdx"`
	License   string `json:"license"`
	Source    string `json:"source"`
	Fetched   string `json:"fetched"`
	BundledIn string `json:"bundledIn,omitempty"`
}

func readManifest(path string) ([]entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("vendor.json を読めない: %w", err)
	}

	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("vendor.json を解析できない: %w", err)
	}

	return entries, nil
}

func writeManifest(path string, entries []entry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func versionOf(entries []entry, name string) string {
	for _, e := range entries {
		if e.Name == name {
			return e.Version
		}
	}

	return ""
}
