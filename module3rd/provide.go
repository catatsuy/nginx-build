package module3rd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cubicdaiya/nginx-build/command"
	"github.com/cubicdaiya/nginx-build/util"
)

func Provide(m *Module3rd) error {
	if len(m.Rev) > 0 {
		originalDir, err := util.SaveCurrentDir()
		if err != nil {
			return fmt.Errorf("failed to save current directory for module %s: %w", m.Name, err)
		}

		targetDir := m.Name
		if err := os.Chdir(targetDir); err != nil {
			return fmt.Errorf("failed to change directory to %s for module %s: %w", targetDir, m.Name, err)
		}

		if err := switchRev(m.Form, m.Rev); err != nil {
			// Attempt to change back to original directory before returning error
			if errChdirBack := os.Chdir(originalDir); errChdirBack != nil {
				log.Printf("Warning: failed to change directory back to %s: %v", originalDir, errChdirBack)
			}
			return fmt.Errorf("failed to switch revision for module %s (form: %s, rev: %s): %w", m.Name, m.Form, m.Rev, err)
		}

		if err := os.Chdir(originalDir); err != nil {
			return fmt.Errorf("failed to change directory back to %s for module %s: %w", originalDir, m.Name, err)
		}
	}

	if len(m.Shprov) > 0 {
		originalDir, err := util.SaveCurrentDir()
		if err != nil {
			return fmt.Errorf("failed to save current directory for module %s shprov: %w", m.Name, err)
		}

		targetDir := m.Name
		if len(m.ShprovDir) > 0 {
			targetDir = filepath.Join(m.Name, m.ShprovDir)
		}
		if err := os.Chdir(targetDir); err != nil {
			return fmt.Errorf("failed to change directory to %s for module %s shprov: %w", targetDir, m.Name, err)
		}

		if err := provideShell(m.Shprov); err != nil {
			// Attempt to change back to original directory before returning error
			if errChdirBack := os.Chdir(originalDir); errChdirBack != nil {
				log.Printf("Warning: failed to change directory back to %s: %v", originalDir, errChdirBack)
			}
			return fmt.Errorf("failed to execute shprov for module %s (shprov: %s): %w", m.Name, m.Shprov, err)
		}

		if err := os.Chdir(originalDir); err != nil {
			return fmt.Errorf("failed to change directory back to %s for module %s shprov: %w", originalDir, m.Name, err)
		}
	}
	return nil
}

func provideShell(sh string) error {
	if strings.TrimSpace(sh) == "" {
		return nil
	}
	if command.VerboseEnabled {
		return command.Run([]string{"sh", "-c", sh})
	}

	cmd := exec.Command("sh", "-c", sh)
	return cmd.Run()
}

func switchRev(form, rev string) error {
	var err error

	switch form {
	case "git":
		err = command.Run([]string{"git", "checkout", rev})
	case "hg":
		err = command.Run([]string{"hg", "checkout", rev})
	default:
		err = fmt.Errorf("form=%s is not supported", form)
	}

	return err
}
