# Installing llmd on Linux

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
`~/.bashrc` or `~/.profile` if it is not already there:

```
export PATH="$HOME/.local/bin:$PATH"
```

Then reload your shell:

```
source ~/.bashrc
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
