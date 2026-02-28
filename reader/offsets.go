package reader

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/NhProGamer/AmongUsReader/types"
)

const (
	baseURLPrimary  = "https://raw.githubusercontent.com/OhMyGuus/BetterCrewlink-Offsets/main"
	baseURLFallback = "https://cdn.jsdelivr.net/gh/OhMyGuus/BetterCrewlink-Offsets@main"
)

// httpGet fetches a URL with a reasonable timeout, falling back to the CDN on error.
func httpGet(primaryURL, fallbackURL string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(primaryURL)
	if err != nil || resp.StatusCode != 200 {
		if fallbackURL != "" {
			resp, err = client.Get(fallbackURL)
			if err != nil {
				return nil, fmt.Errorf("both primary and fallback fetch failed: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", primaryURL, err)
		}
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// cacheDir returns (and creates if needed) the directory used to store
// the cached lookup and offsets JSON files.
func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".amogus-reader")
	return dir, os.MkdirAll(dir, 0o755)
}

// FetchOffsetLookup downloads lookup.json and caches it locally.
// On network failure, the cached copy is used as a fallback.
func FetchOffsetLookup() (*types.IOffsetsLookup, error) {
	data, netErr := httpGet(
		baseURLPrimary+"/lookup.json",
		baseURLFallback+"/lookup.json",
	)

	dir, err := cacheDir()
	if err != nil && netErr != nil {
		return nil, fmt.Errorf("network error and no cache dir: %w", netErr)
	}
	cachePath := filepath.Join(dir, "lookup.json")

	if netErr == nil {
		// Persist fresh copy
		_ = os.WriteFile(cachePath, data, 0o644)
	} else {
		// Try reading from cache
		data, err = os.ReadFile(cachePath)
		if err != nil {
			return nil, fmt.Errorf("lookup fetch failed and no cache available: %w", netErr)
		}
	}

	var lookup types.IOffsetsLookup
	if err := json.Unmarshal(data, &lookup); err != nil {
		return nil, fmt.Errorf("parse lookup.json: %w", err)
	}
	return &lookup, nil
}

// FetchOffsets downloads the versioned offsets JSON for the given architecture
// and filename.  It caches the result and only re-downloads when offsetsVersion
// is newer than what we have on disk.
func FetchOffsets(is64bit bool, filename string, offsetsVersion int) (*types.IOffsets, error) {
	arch := "x86"
	if is64bit {
		arch = "x64"
	}
	dir, _ := cacheDir()
	cacheKey := fmt.Sprintf("offsets_%s_%s.json", arch, filename)
	cachePath := filepath.Join(dir, cacheKey)
	metaPath := cachePath + ".meta"

	// Check if cached version is recent enough.
	if cachedVersion, err := readCachedVersion(metaPath); err == nil && cachedVersion >= offsetsVersion {
		if data, err := os.ReadFile(cachePath); err == nil {
			var offsets types.IOffsets
			if json.Unmarshal(data, &offsets) == nil {
				return &offsets, nil
			}
		}
	}

	// Download fresh copy.
	url := fmt.Sprintf("%s/offsets/%s/%s", baseURLPrimary, arch, filename)
	fallback := fmt.Sprintf("%s/offsets/%s/%s", baseURLFallback, arch, filename)
	data, err := httpGet(url, fallback)
	if err != nil {
		// Try stale cache before giving up.
		if stale, e2 := os.ReadFile(cachePath); e2 == nil {
			var offsets types.IOffsets
			if json.Unmarshal(stale, &offsets) == nil {
				return &offsets, nil
			}
		}
		return nil, fmt.Errorf("fetch offsets %s/%s: %w", arch, filename, err)
	}

	var offsets types.IOffsets
	if err := json.Unmarshal(data, &offsets); err != nil {
		return nil, fmt.Errorf("parse offsets %s/%s: %w", arch, filename, err)
	}

	_ = os.WriteFile(cachePath, data, 0o644)
	_ = writeCachedVersion(metaPath, offsetsVersion)
	return &offsets, nil
}

func readCachedVersion(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1, err
	}
	var v int
	if _, err := fmt.Sscanf(string(data), "%d", &v); err != nil {
		return -1, errors.New("invalid meta")
	}
	return v, nil
}

func writeCachedVersion(path string, version int) error {
	return os.WriteFile(path, []byte(fmt.Sprintf("%d", version)), 0o644)
}
