// service.go provides a system service wrapper using kardianos/service for
// running the OpenCron daemon as a Windows Service or systemd unit. It
// implements the service.Interface (Start/Stop) and exposes InstallService
// and UninstallService functions for managing the service lifecycle.
//
// On Linux, InstallService installs a systemd user service (~/.config/systemd/user/)
// which does not require root. Use --system flag to install as a system-wide service
// (requires sudo).
package daemon

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/kardianos/service"
)

// opencronsService implements the kardianos/service Interface.
type opencronsService struct{}

func (s *opencronsService) Start(_ service.Service) error {
	go func() {
		if err := Run(); err != nil {
			log.Printf("Daemon error: %v", err)
			os.Exit(1)
		}
	}()
	return nil
}

func (s *opencronsService) Stop(_ service.Service) error {
	// Daemon handles shutdown via signal
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(os.Interrupt)
	return nil
}

// systemd user unit template
var userUnitTmpl = template.Must(template.New("unit").Parse(`[Unit]
Description=OpenCron daemon for Claude Code automation jobs
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.ExecPath}} start --foreground
Restart=on-failure
RestartSec=5
Environment=HOME={{.Home}}
Environment=PATH={{.Path}}

[Install]
WantedBy=default.target
`))

type unitData struct {
	ExecPath string
	Home     string
	Path     string
}

// InstallService installs OpenCron as a system service.
// On Linux, this installs a systemd user service by default (no root required).
// Set system=true to install as a system-wide service (requires sudo).
func InstallService() error {
	return installService(false)
}

// InstallSystemService installs OpenCron as a system-wide service (requires root).
func InstallSystemService() error {
	return installService(true)
}

func installService(system bool) error {
	// On Linux, prefer user service unless --system was requested.
	if runtime.GOOS == "linux" && !system {
		return installUserService()
	}
	return installSystemServiceViaKardianos()
}

// installUserService writes a systemd user unit and enables it.
func installUserService() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}
	// Resolve symlinks so the unit file points to the real binary.
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return fmt.Errorf("creating systemd user directory: %w", err)
	}

	unitPath := filepath.Join(unitDir, "opencrons.service")
	f, err := os.Create(unitPath)
	if err != nil {
		return fmt.Errorf("creating unit file: %w", err)
	}

	data := unitData{
		ExecPath: execPath,
		Home:     home,
		Path:     os.Getenv("PATH"),
	}
	if err := userUnitTmpl.Execute(f, data); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing unit file: %w", err)
	}
	_ = f.Close()

	// Reload and enable
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("daemon-reload failed: %w", err)
	}
	if err := exec.Command("systemctl", "--user", "enable", "opencrons.service").Run(); err != nil {
		return fmt.Errorf("enabling service: %w", err)
	}

	fmt.Printf("User service installed: %s\n", unitPath)
	fmt.Println("Start with:  systemctl --user start opencrons")
	fmt.Println("View logs:   journalctl --user -u opencrons -f")
	return nil
}

// installSystemServiceViaKardianos uses kardianos/service for system-wide
// installation (Linux system service, Windows Service, macOS launchd).
func installSystemServiceViaKardianos() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	svcConfig := &service.Config{
		Name:        "opencrons",
		DisplayName: "OpenCron",
		Description: "OpenCron daemon for Claude Code automation jobs",
		Arguments:   []string{"start", "--foreground"},
		Executable:  execPath,
	}

	svc, err := service.New(&opencronsService{}, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	if err := svc.Install(); err != nil {
		return fmt.Errorf("installing service: %w", err)
	}

	fmt.Println("System service installed successfully.")
	fmt.Println("Start it with: sudo systemctl start opencrons")
	return nil
}

// UninstallService removes the service (tries user service first on Linux).
func UninstallService() error {
	return uninstallService(false)
}

// UninstallSystemService removes the system-wide service.
func UninstallSystemService() error {
	return uninstallService(true)
}

func uninstallService(system bool) error {
	if runtime.GOOS == "linux" && !system {
		return uninstallUserService()
	}
	return uninstallSystemServiceViaKardianos()
}

func uninstallUserService() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	// Stop first (ignore error if not running)
	_ = exec.Command("systemctl", "--user", "stop", "opencrons.service").Run()
	_ = exec.Command("systemctl", "--user", "disable", "opencrons.service").Run()

	unitPath := filepath.Join(home, ".config", "systemd", "user", "opencrons.service")
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file: %w", err)
	}

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()

	fmt.Println("User service uninstalled.")
	return nil
}

func uninstallSystemServiceViaKardianos() error {
	svcConfig := &service.Config{
		Name: "opencrons",
	}

	svc, err := service.New(&opencronsService{}, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	if err := svc.Uninstall(); err != nil {
		return fmt.Errorf("uninstalling service: %w", err)
	}

	fmt.Println("System service uninstalled.")
	return nil
}
