# Installation

## Installer scripts

Release snapshots render the repository into both installers, so the normal installation command only needs the version.

Linux or macOS:

```bash
./install.sh 1.0.0
```

Windows PowerShell:

```powershell
.\install.ps1 1.0.0
```

Mirrors and offline smoke tests can override the release base URL:

```bash
GOSVC_RELEASE_BASE_URL=http://127.0.0.1:18080 \
GOSVC_INSTALL_DIR="$PWD/tmp-bin" \
./install.sh 1.0.0
```

The repository can still be overridden explicitly with `GOSVC_REPOSITORY` or `-Repository`.

## Homebrew

Every release contains a formula named `gosvc.rb`. A tap can copy that formula directly:

```bash
brew install OWNER/tap/gosvc
```

For local validation:

```bash
brew install --formula ./gosvc.rb
```

## Scoop

Every release contains `gosvc.json`, ready to place in a Scoop bucket:

```powershell
scoop bucket add OWNER https://github.com/OWNER/scoop-bucket
scoop install ailtonmacedo/gosvc
```

For local validation:

```powershell
scoop install .\gosvc.json
```

## Release archive

Linux or macOS:

```bash
curl -fsSLO https://github.com/ailtonmacedo/gosvc/releases/download/v1.0.0/gosvc_1.0.0_linux_amd64.tar.gz
curl -fsSLO https://github.com/ailtonmacedo/gosvc/releases/download/v1.0.0/checksums.txt
sha256sum --check --ignore-missing checksums.txt
tar -xzf gosvc_1.0.0_linux_amd64.tar.gz
install -m 0755 gosvc_1.0.0_linux_amd64/gosvc ~/.local/bin/gosvc
```

Windows users can download the matching ZIP and verify it with `Get-FileHash`.

## Go install

After preparing the final repository identity:

```bash
go install github.com/ailtonmacedo/gosvc/cmd/gosvc@v1.0.0
```

## Shell completion

```bash
gosvc completion bash > ~/.local/share/bash-completion/completions/gosvc
gosvc completion zsh > "${fpath[1]}/_gosvc"
gosvc completion fish > ~/.config/fish/completions/gosvc.fish
gosvc completion powershell > gosvc.ps1
```
