# Windows Installation Guide

## winget

Microsoft's `winget` is the recommended installation method for Windows:

In an elevated powershell session, run:

```powershell
winget install gfm-hotview
```

## PowerShell Installer

In an elevated powershell session, run:

```powershell
irm https://packages.keathmilligan.net/gfm-hotview/install.ps1 | iex
```

## Windows MSI

Download the signed `.msi` installer directly from the [GitHub
Releases](https://github.com/keathmilligan/gfm-hotview/releases) page.

## Binary

Download the Windows binary archive for your architecture (Intel `x86_64` or
ARM `aarch64`) from the [GitHub
Releases](https://github.com/keathmilligan/gfm-hotview/releases) page.

Extract the `gfm-hotview.exe` binary into a directory in your `PATH`.
