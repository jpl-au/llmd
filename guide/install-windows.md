# Installing llmd on Windows

How to put a downloaded `llmd.exe` on your PATH on Windows and run it
for the first time.

## Quick reference

```
mkdir %USERPROFILE%\bin
move llmd.exe %USERPROFILE%\bin\
(add %USERPROFILE%\bin to your PATH, then open a new terminal)
llmd version
llmd init && llmd config author "Your Name" && llmd guide
```

## Install

1. Create a directory for the binary:

```
mkdir %USERPROFILE%\bin
```

2. Move `llmd.exe` into it:

```
move llmd.exe %USERPROFILE%\bin\
```

3. Add the directory to your PATH:
   - Open **Settings > System > About > Advanced system settings**
   - Click **Environment Variables**
   - Under **User variables**, select **Path** and click **Edit**
   - Click **New** and add `%USERPROFILE%\bin`
   - Click **OK** to save

4. Open a new terminal window (the old one will not see the PATH change).

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
