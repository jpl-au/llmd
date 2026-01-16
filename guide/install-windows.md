# Installing llmd on Windows

You have llmd downloaded as a binary. Now make it accessible from anywhere.

## Move to a folder and add to PATH

1. Create a folder for your binaries (if you don't have one):
   ```
   C:\Users\YourName\bin
   ```

2. Move `llmd.exe` to that folder

3. Add the folder to your PATH:
   - Press `Win + R`, type `sysdm.cpl`, press Enter
   - Click "Advanced" tab → "Environment Variables"
   - Under "User variables", select "Path" and click "Edit"
   - Click "New" and add `C:\Users\YourName\bin`
   - Click OK to save

4. Open a **new** terminal window

## Verify

```powershell
llmd version
```
