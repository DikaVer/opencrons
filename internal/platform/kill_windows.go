//go:build windows

// kill_windows.go implements process enumeration via tasklist and force
// termination via taskkill for Windows.
package platform

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// killAllByName finds all processes whose image name matches exeName and kills
// them, skipping selfPID to avoid killing the current CLI process.
func killAllByName(exeName string, selfPID int) ([]int, error) {
	out, err := exec.Command(
		"tasklist",
		"/FI", "IMAGENAME eq "+exeName,
		"/FO", "CSV",
		"/NH",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}

	reader := csv.NewReader(bytes.NewReader(out))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing process list: %w", err)
	}

	var killed []int
	for _, record := range records {
		// CSV columns: "Image Name","PID","Session Name","Session#","Mem Usage"
		// When no processes match, tasklist exits 0 and writes a localized
		// info message — a single non-CSV column. Verify the image name so
		// those rows (and any other noise) are skipped.
		if len(record) < 2 || !strings.EqualFold(record[0], exeName) {
			continue
		}
		pidStr := strings.TrimSpace(record[1])
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid == selfPID {
			continue
		}
		if err := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run(); err == nil {
			killed = append(killed, pid)
		}
	}

	return killed, nil
}
