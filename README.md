# OpenCron

An open-source cron scheduler for Claude Code automation. Run `claude -p` jobs on cron schedules and chat with Claude from Telegram — all managed through an interactive TUI or command-line interface.

**Core idea:**


## Quick Start

### Install

One command to build and install `opencron` globally:

```bash
# All platforms (requires Go)
go install github.com/DikaVer/opencron/cmd/opencron@latest
```

This builds the binary and places it in `$GOPATH/bin` (`$HOME/go/bin` by default), which is already in your PATH if Go is set up correctly.

**Or build from source:**

```bash
# Linux / macOS
sudo make install              # builds and installs to /usr/local/bin/

# Windows (PowerShell)
go install ./cmd/opencron/     # builds and installs to %GOPATH%\bin\
```

Verify it works:

```bash
opencron --help
```

**Uninstall:**

```bash
# Linux / macOS
sudo make uninstall

# Windows (PowerShell) — remove from GOPATH\bin
Remove-Item "$(go env GOPATH)\bin\opencron.exe"
```

