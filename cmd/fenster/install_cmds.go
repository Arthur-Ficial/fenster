package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Arthur-Ficial/fenster/internal/extension"
	"github.com/Arthur-Ficial/fenster/internal/manifest"
	"github.com/spf13/cobra"
)

func newInstallExtensionCmd() *cobra.Command {
	var dest string
	cmd := &cobra.Command{
		Use:   "install-extension",
		Short: "extract the bundled Chrome extension to ~/.fenster/extension/",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := dest
			if out == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				out = filepath.Join(home, ".fenster", "extension")
			}
			if err := writeExtension(out); err != nil {
				return err
			}
			fmt.Println("✓ extension written to", out)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  1. Open Chrome → chrome://extensions/")
			fmt.Println("  2. Toggle Developer mode (top-right)")
			fmt.Println("  3. Click `Load unpacked`")
			fmt.Println("  4. Select the directory:", out)
			fmt.Println("  5. Note the extension ID shown on chrome://extensions/")
			fmt.Println("  6. Run: fenster install-manifest --extension-id <ID>")
			fmt.Println("  7. Restart Chrome (so the manifest is reloaded)")
			fmt.Println("  8. Run: fenster --serve")
			return nil
		},
	}
	cmd.Flags().StringVar(&dest, "dest", "", "override destination directory")
	return cmd
}

func newInstallManifestCmd() *cobra.Command {
	var extID string
	cmd := &cobra.Command{
		Use:   "install-manifest",
		Short: "register fenster as a Chrome Native Messaging host",
		RunE: func(cmd *cobra.Command, args []string) error {
			if extID == "" {
				return fmt.Errorf("--extension-id is required (find it on chrome://extensions/)")
			}
			binary, err := os.Executable()
			if err != nil {
				return err
			}
			results, err := manifest.InstallAll(binary, extID)
			if err != nil {
				return err
			}
			for _, r := range results {
				if r.Err != nil {
					fmt.Printf("· %-9s skipped: %v\n", r.Browser, r.Err)
					continue
				}
				fmt.Printf("✓ %-9s manifest at %s\n", r.Browser, r.Path)
			}
			fmt.Println("\nDone. Restart Chrome (or relevant browser) so it reloads the NM manifest.")
			return nil
		},
	}
	cmd.Flags().StringVar(&extID, "extension-id", "", "fenster extension ID from chrome://extensions/")
	return cmd
}

// writeExtension extracts internal/extension/assets/* into the given dir.
func writeExtension(out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(extension.Assets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(extension.Assets, path)
		if err != nil {
			return err
		}
		// strip the "assets/" prefix
		dst := filepath.Join(out, filepath.Base(path))
		return os.WriteFile(dst, body, 0o644)
	})
}
