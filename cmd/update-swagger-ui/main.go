package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	targetDir   = "web/swagger"
	specFile    = "openapi.yaml"
	sourceSpec  = "api/openapi.yaml"
	githubRepo  = "swagger-api/swagger-ui"
	testSpecURL = "https://petstore.swagger.io/v2/swagger.json"
	localSpec   = "openapi.yaml"
)

type Release struct {
	TagName string `json:"tag_name"`
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("[update-swagger-ui] ")

	// 1. Get latest release tag
	tag, err := getLatestTag()
	if err != nil {
		log.Fatalf("Error getting latest tag: %v", err)
	}
	log.Printf("Latest release: %s", tag)

	// 2. Clean target directory (except specFile)
	if err := cleanTargetDir(); err != nil {
		log.Fatalf("Error cleaning target directory: %v", err)
	}

	// 3. Download and extract dist folder
	if err := downloadAndInstall(tag); err != nil {
		log.Fatalf("Error downloading/installing: %v", err)
	}

	// 4. Copy OpenAPI spec
	if err := copySpec(); err != nil {
		log.Fatalf("Error copying spec: %v", err)
	}

	// 5. Configure swagger-initializer.js
	if err := updateInitializer(); err != nil {
		log.Fatalf("Error updating initializer: %v", err)
	}

	log.Println("Success!")
}

func copySpec() error {
	src, err := os.Open(sourceSpec)
	if err != nil {
		return fmt.Errorf("failed to open source spec %s: %w", sourceSpec, err)
	}
	defer src.Close()

	dstPath := filepath.Join(targetDir, specFile)
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create target spec %s: %w", dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy spec content: %w", err)
	}
	return nil
}

func getLatestTag() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

func cleanTargetDir() error {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(targetDir, 0755)
		}
		return err
	}

	for _, entry := range entries {
		if entry.Name() == specFile {
			continue
		}
		path := filepath.Join(targetDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func downloadAndInstall(tag string) error {
	// Download tarball
	url := fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.tar.gz", githubRepo, tag)
	log.Printf("Downloading %s...", url)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	// Extract
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}

	foundDist := false
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Look for /dist/ folder in the archive
		// Structure is usually swagger-ui-x.y.z/dist/...
		parts := strings.Split(header.Name, "/")
		if len(parts) > 1 && parts[1] == "dist" {
			foundDist = true
			if header.Typeflag == tar.TypeDir {
				continue // Skip directories, we create files
			}

			// Determine relative path inside dist
			relPath := strings.Join(parts[2:], "/")
			if relPath == "" {
				continue
			}

			destPath := filepath.Join(targetDir, relPath)
			absDestPath, err := filepath.Abs(destPath)
			if err != nil {
				return err
			}

			// Ensure that the destination path is within the target directory
			prefix := absTargetDir + string(os.PathSeparator)
			if !strings.HasPrefix(absDestPath+string(os.PathSeparator), prefix) {
				return fmt.Errorf("invalid path in archive: %s", header.Name)
			}

			// Create directory if needed (e.g. dist/foo/bar.js)
			if err := os.MkdirAll(filepath.Dir(absDestPath), 0755); err != nil {
				return err
			}

			f, err := os.Create(absDestPath)
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	if !foundDist {
		return fmt.Errorf("dist folder not found in archive")
	}
	return nil
}

func updateInitializer() error {
	path := filepath.Join(targetDir, "swagger-initializer.js")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Replace the default URL with our local spec
	newContent := strings.Replace(string(content), testSpecURL, localSpec, 1)

	return os.WriteFile(path, []byte(newContent), 0644)
}
