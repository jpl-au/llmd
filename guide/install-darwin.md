# Installing llmd on macOS

How to put a downloaded `llmd` binary on your PATH on macOS and run
it for the first time.

## Quick reference

```
xattr -d com.apple.quarantine llmd      # clear the download quarantine
sudo mv llmd /usr/local/bin/            # system-wide, or:
mkdir -p ~/.local/bin && mv llmd ~/.local/bin/
llmd version                            # verify
llmd init && llmd config author "Your Name" && llmd guide
```

## Remove quarantine

macOS quarantines unsigned binaries downloaded from the internet. Clear
the quarantine attribute before running:

```
xattr -d com.apple.quarantine llmd
```

## Install

Move the binary to a directory on your PATH:

```
# System-wide (requires sudo)
sudo mv llmd /usr/local/bin/

# User-local
mkdir -p ~/.local/bin
mv llmd ~/.local/bin/
```

If using `~/.local/bin`, ensure it is on your PATH. Add this to your
`~/.zshrc`:

```
export PATH="$HOME/.local/bin:$PATH"
```

Then reload your shell:

```
source ~/.zshrc
```

## Verify

```
llmd version
```

## Get started

```
llmd init
llmd config author "Your Name"
llmd guide
```
