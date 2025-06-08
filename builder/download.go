package builder

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cubicdaiya/nginx-build/command"
	"github.com/cubicdaiya/nginx-build/util"
)

const DefaultDownloadTimeout = time.Duration(900) * time.Second

func extractArchive(path string) error {
	// Use command.Run directly as it handles verbose logging.
	return command.Run([]string{"tar", "zxvf", path})
}

// download Fetches the component archive.
// It takes a non-pointer Builder because it only reads from it.
func downloadFile(b Builder) error { // Changed to non-pointer b as it only reads.
	c := &http.Client{
		Timeout: DefaultDownloadTimeout,
	}
	downloadURL := b.DownloadURL()
	log.Printf("Downloading %s from %s", b.ArchivePath(), downloadURL)
	res, err := c.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to start download for %s: %w", downloadURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: status %s", downloadURL, res.Status)
	}

	tmpFileName := b.ArchivePath() + ".download"
	f, err := os.Create(tmpFileName)
	if err != nil {
		return fmt.Errorf("failed to create temporary file %s: %w", tmpFileName, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, res.Body); err != nil { // Removed io.EOF check, Copy returns nil on EOF.
		return fmt.Errorf("failed to write to temporary file %s: %w", tmpFileName, err)
	}

	if err := os.Rename(tmpFileName, b.ArchivePath()); err != nil {
		return fmt.Errorf("failed to rename temporary file %s to %s: %w", tmpFileName, b.ArchivePath(), err)
	}
	log.Printf("Successfully downloaded %s", b.ArchivePath())
	return nil
}

// DownloadAndExtractComponent handles downloading and extracting a component.
// It's designed to be called concurrently.
func DownloadAndExtractComponent(b *Builder) {
	if util.FileExists(b.SourcePath()) {
		log.Printf("%s already exists. Skipping download and extraction.", b.SourcePath())
		return
	}

	// Ensure archive path directory exists (though current logic implies CWD is workDir)
	// No, ArchivePath is usually in the CWD which is workDir/nginx-version etc.

	if !util.FileExists(b.ArchivePath()) {
		log.Printf("Attempting to download %s.....", b.SourcePath()) // b.SourcePath() is like "nginx-1.25.3"
		if err := downloadFile(*b); err != nil { // Pass by value to downloadFile
			util.PrintFatalMsg(fmt.Errorf("failed to download %s: %w", b.SourcePath(), err), b.LogPath())
			return // Critical error, stop processing this component
		}
	} else {
		log.Printf("Archive %s already exists. Skipping download.", b.ArchivePath())
	}

	log.Printf("Extracting %s.....", b.ArchivePath())
	if err := extractArchive(b.ArchivePath()); err != nil {
		util.PrintFatalMsg(fmt.Errorf("failed to extract %s: %w", b.ArchivePath(), err), b.LogPath())
		return
	}
	log.Printf("Successfully extracted %s to %s", b.ArchivePath(), b.SourcePath())
}
