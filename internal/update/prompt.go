package update

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ellery/thicc/internal/config"
)

// Result represents the result of the update check and prompt
type Result int

const (
	// ResultNoUpdate means no update is available or check was skipped
	ResultNoUpdate Result = iota
	// ResultUpdateDeclined means user declined the update
	ResultUpdateDeclined
	// ResultUpdateSkipped means user chose to skip this version
	ResultUpdateSkipped
	// ResultUpdateSuccess means update was successful
	ResultUpdateSuccess
	// ResultUpdateFailed means update failed
	ResultUpdateFailed
)

// CheckAndPrompt checks for updates and handles the user interaction
// It returns true if the caller should proceed with quitting, false if it should wait
// The promptFn is called to ask the user Y/N questions
// The messageFn is called to show status messages (variadic to match InfoBar.Message)
func CheckAndPrompt(
	promptFn func(msg string, callback func(yes, canceled bool)),
	messageFn func(msg ...any),
	doneFn func(result Result, err error),
) {
	// Check if update checking is enabled
	if !config.GetGlobalOption("updatecheck").(bool) {
		doneFn(ResultNoUpdate, nil)
		return
	}

	// Load state and check if we should check
	state, err := LoadState()
	if err != nil {
		log.Printf("Failed to load update state: %v", err)
		doneFn(ResultNoUpdate, nil)
		return
	}

	frequency := config.GetGlobalOption("updatefrequency").(float64)
	if !state.ShouldCheck(frequency) {
		doneFn(ResultNoUpdate, nil)
		return
	}

	// Check for updates with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	channel := config.GetGlobalOption("updatechannel").(string)
	messageFn("Checking for updates...")

	updateInfo, err := CheckForUpdate(ctx, channel, false)
	if err != nil {
		log.Printf("Update check failed: %v", err)
		// Update last check time even on failure to avoid repeated failures
		state.MarkChecked()
		state.Save()
		doneFn(ResultNoUpdate, nil)
		return
	}

	if updateInfo == nil {
		// No update available
		state.MarkChecked()
		state.Save()
		doneFn(ResultNoUpdate, nil)
		return
	}

	// Check if user previously skipped this version
	if state.IsVersionSkipped(updateInfo.LatestVersion) {
		doneFn(ResultNoUpdate, nil)
		return
	}

	// Update available - ask user
	sizeStr := humanize.Bytes(uint64(updateInfo.DownloadSize))
	promptMsg := fmt.Sprintf("Update available: %s → %s (%s). Update now?",
		updateInfo.CurrentVersion,
		updateInfo.LatestVersion,
		sizeStr,
	)

	promptFn(promptMsg, func(yes, canceled bool) {
		if canceled {
			// User pressed Esc - don't update, don't mark as checked
			doneFn(ResultUpdateDeclined, nil)
			return
		}

		if !yes {
			// User said no - mark as checked but don't skip version
			state.MarkChecked()
			state.Save()
			doneFn(ResultUpdateDeclined, nil)
			return
		}

		// User said yes - download and install
		messageFn("Downloading update...")

		progressFn := func(downloaded, total int64) {
			if total > 0 {
				pct := int(float64(downloaded) / float64(total) * 100)
				messageFn(fmt.Sprintf("Downloading update... %d%%", pct))
			}
		}

		err := DownloadAndInstall(updateInfo, progressFn)
		if err != nil {
			log.Printf("Update failed: %v", err)
			messageFn(fmt.Sprintf("Update failed: %v", err))
			doneFn(ResultUpdateFailed, err)
			return
		}

		state.MarkChecked()
		state.Save()
		messageFn("Update successful! Restart to use the new version.")
		doneFn(ResultUpdateSuccess, nil)
	})
}
