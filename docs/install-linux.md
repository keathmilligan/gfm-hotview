# Linux Installation Guide

Choose the best installation option for your distro.

## Shell Installer (all distros)

```bash
curl -fsSL https://packages.keathmilligan.net/gfm-hotview/install.sh | sh
```

This will install `gfm-hotview` into `~/.local/bin`.

## Homebrew (all distros)

Homebrew is also supported on Linux. If you have it installed:

```bash
brew tap keathmilligan/tap
brew install keathmilligan/tap/gfm-hotview
```

## apt (Debian / Ubuntu)

```bash
curl -fsSL https://packages.keathmilligan.net/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/keathmilligan.gpg
echo "deb [signed-by=/etc/apt/keyrings/keathmilligan.gpg] https://packages.keathmilligan.net/apt stable main" | sudo tee /etc/apt/sources.list.d/keathmilligan.list
sudo apt update
sudo apt install gfm-hotview
```

Stay up to date with:

```
sudo apt upgrade gfm-hotview
```

## dnf / rpm (Fedora / RHEL / CentOS)

```bash
sudo curl -o /etc/yum.repos.d/keathmilligan.repo https://packages.keathmilligan.net/rpm/keathmilligan.repo
sudo dnf install gfm-hotview
```

Stay up to date with:

```
sudo dnf upgrade gfm-hotview
```

## Binary

Download the linux binary archive for your architecture (Intel `x86_64` or
ARM `aarch64`) from the [GitHub
Releases](https://github.com/keathmilligan/gfm-hotview/releases) page.

Extract the `gfm-hotview` binary into a directory in your `PATH`.
