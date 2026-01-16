# Installing llmd on Linux

You have llmd downloaded as a binary. Now make it accessible from anywhere.

## Move to a directory in your PATH

```bash
# Option 1: System-wide (requires sudo)
sudo mv llmd /usr/local/bin/

# Option 2: User-only (no sudo)
mkdir -p ~/.local/bin
mv llmd ~/.local/bin/
```

If using `~/.local/bin`, ensure it's in your PATH:

```bash
echo 'export PATH="$PATH:$HOME/.local/bin"' >> ~/.bashrc
source ~/.bashrc
```

## Verify

```bash
llmd version
```
