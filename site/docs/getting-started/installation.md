---
sidebar_position: 1
title: Installation
description: Install Crux from source, go install, or the curl installer. Covers tmux prerequisites, shell completions for bash/zsh/fish/PowerShell, and man page generation.
---

# Installation

## From Source (Recommended)

Requires Go 1.25+ installed.

```bash
git clone https://github.com/roygabriel/crux.git
cd crux
make build
```

The binary is at `bin/crux`. Copy it to your PATH:

```bash
sudo install -m 755 bin/crux /usr/local/bin/crux
```

Or use the install target:

```bash
sudo make install
```

## Using `go install`

```bash
go install github.com/roygabriel/crux/cmd/crux@latest
```

## Curl Installer (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/roygabriel/crux/main/scripts/install.sh | sh
```

Override the install directory:

```bash
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/roygabriel/crux/main/scripts/install.sh | sh
```

## Prerequisites

Crux requires **tmux** to be installed and available in your PATH. Install it with your system package manager:

```bash
# Debian / Ubuntu
sudo apt install tmux

# macOS
brew install tmux

# Arch Linux
sudo pacman -S tmux
```

## Shell Completions

Generate and install shell completions for tab-completion of commands and flags.

### Bash

```bash
# Source for current session
source <(crux completion bash)

# Install permanently
crux completion bash > /etc/bash_completion.d/crux
```

### Zsh

```bash
# Ensure completions are enabled in ~/.zshrc:
# autoload -U compinit; compinit

crux completion zsh > "${fpath[1]}/_crux"
```

### Fish

```bash
crux completion fish | source

# Install permanently
crux completion fish > ~/.config/fish/completions/crux.fish
```

### PowerShell

```powershell
crux completion powershell | Out-String | Invoke-Expression
```

## Man Pages

Generate and install man pages:

```bash
make man
sudo make install-man
```

Then access with `man crux`.

## Verify Installation

```bash
crux --version
crux --help
```
