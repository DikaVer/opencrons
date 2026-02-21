// service.go provides a system service wrapper using kardianos/service for
// running the OpenCron daemon as a Windows Service or systemd unit. It
// implements the service.Interface (Start/Stop) and exposes InstallService
// and UninstallService functions for managing the service lifecycle.
package daemon

import (
	"fmt"
	"log"
	"os"

	"github.com/kardianos/service"
)

// opencronService implements the kardianos/service Interface.
type opencronService struct{}

func (s *opencronService) Start(_ service.Service) error {
	go func() {
		if err := Run(); err != nil {
			log.Printf("Daemon error: %v", err)
			os.Exit(1)
		}
	}()
	return nil
}

func (s *opencronService) Stop(_ service.Service) error {
	// Daemon handles shutdown via signal
	p, _ := os.FindProcess(os.Getpid())
	p.Signal(os.Interrupt)
	return nil
}

// InstallService installs OpenCron as a system service.
func InstallService() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	svcConfig := &service.Config{
		Name:        "opencron",
		DisplayName: "OpenCron",
		Description: "OpenCron daemon for Claude Code automation jobs",
		Arguments:   []string{"start"},
		Executable:  execPath,
	}

	svc, err := service.New(&opencronService{}, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	if err := svc.Install(); err != nil {
		return fmt.Errorf("installing service: %w", err)
	}

	fmt.Println("Service installed successfully.")
	fmt.Println("Start it with: opencron start (or via system service manager)")
	return nil
}

// UninstallService removes the system service.
func UninstallService() error {
	svcConfig := &service.Config{
		Name: "opencron",
	}

	svc, err := service.New(&opencronService{}, svcConfig)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	if err := svc.Uninstall(); err != nil {
		return fmt.Errorf("uninstalling service: %w", err)
	}

	fmt.Println("Service uninstalled.")
	return nil
}
