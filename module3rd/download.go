package module3rd

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/cubicdaiya/nginx-build/command"
	"github.com/cubicdaiya/nginx-build/util"
)

func DownloadAndExtractParallel(m Module3rd) error {
	if util.FileExists(m.Name) {
		log.Printf("Module %s already exists in %s. Skipping download.", m.Name, m.Name)
		return nil
	}

	if m.Form != "local" {
		if len(m.Rev) > 0 {
			log.Printf("Downloading module %s-%s from %s.....", m.Name, m.Rev, m.Url)
		} else {
			log.Printf("Downloading module %s from %s.....", m.Name, m.Url)
		}

		logName := fmt.Sprintf("%s.log", m.Name) // Log for the download process itself

		err := download(m, logName) // download is from the same package
		if err != nil {
			return fmt.Errorf("failed to download module %s from %s: %w", m.Name, m.Url, err)
		}
		log.Printf("Successfully downloaded module %s.", m.Name)
	} else {
		// This is for m.Form == "local"
		if !util.FileExists(m.Url) {
			// m.Url is the local path here. m.Name is the directory name it will have in the build context.
			return fmt.Errorf("local module path %s (for module %s) not found", m.Url, m.Name)
		}
		log.Printf("Using local module %s from %s.", m.Name, m.Url)
	}
	return nil
}

func download(m Module3rd, logName string) error {
	form := m.Form
	url := m.Url

	switch form {
	case "git":
		args := []string{form, "clone", "--recursive", url}
		if command.VerboseEnabled {
			return command.Run(args)
		}

		f, err := os.Create(logName)
		if err != nil {
			return command.Run(args)
		}
		defer f.Close()

		cmd, err := command.Make(args)
		if err != nil {
			return err
		}

		writer := bufio.NewWriter(f)
		defer writer.Flush()

		cmd.Stderr = writer

		return cmd.Run()
	case "hg":
		args := []string{form, "clone", url}
		if command.VerboseEnabled {
			return command.Run(args)
		}

		f, err := os.Create(logName)
		if err != nil {
			return command.Run(args)
		}
		defer f.Close()

		cmd, err := command.Make(args)
		if err != nil {
			return err
		}

		writer := bufio.NewWriter(f)
		defer writer.Flush()

		cmd.Stderr = writer

		return cmd.Run()
	case "local": // not implemented yet
		return nil
	}

	return fmt.Errorf("form=%s is not supported", form)
}
