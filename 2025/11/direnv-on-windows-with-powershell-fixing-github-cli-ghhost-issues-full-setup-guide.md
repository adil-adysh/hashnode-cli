# **Direnv on Windows with PowerShell: Fixing GitHub CLI GH\_HOST Issues (Full Setup Guide)**

I’ve been working across multiple GitHub environments lately — some repos use GitHub Enterprise, others use the public [`github.com`](http://github.com), and a few even have *both* remotes inside the same repo. This caused a very specific and frustrating issue with the GitHub CLI (`gh`):

* The repo had **two remotes**:
    
    * `origin` → internal enterprise
        
    * `upstream` → public GitHub
        
* Running `gh repo set-default` only showed [**github.com**](http://github.com)
    
* API calls kept going to the wrong host
    
* Authentication was inconsistent
    
* PR and issue commands returned incorrect results
    

After debugging, I discovered the real reason:

> `gh` gets confused unless `GH_HOST` is explicitly set.

When I set `GH_HOST` manually, everything worked. But I didn’t want to do that for every repository, every time.

That’s when I set up **direnv**, so each project loads its own `GH_HOST` automatically.

Getting direnv working on Windows with PowerShell wasn’t straightforward, so this post documents everything that finally worked for me.

---

# What is Direnv?

Direnv is a small utility that automatically loads and unloads environment variables based on the directory you’re in. When a folder contains an `.envrc` file:

* entering the folder → applies its variables
    
* leaving the folder → unloads them
    

It keeps environments clean and per-project config isolated. Perfect for tools like GitHub CLI that depend on environment variables like `GH_HOST`.

---

# Why Direnv Solved My GitHub CLI Problem

### The exact issue

Inside one repo:

```plaintext
origin   -> github.mycompany.com  (internal)
upstream -> github.com            (public)
```

Because `GH_HOST` wasn’t set:

* `gh repo set-default` **ignored the internal remote**
    
* PR and issue queries pointed to the wrong API host
    
* Workflow commands broke
    
* Enterprise authentication failed
    
* Public GitHub was incorrectly chosen
    

Setting `GH_HOST` explicitly fixes this:

```bash
export GH_HOST="github.mycompany.com"
```

But doing this manually is error-prone.

Direnv automates it — safely, and per project.

---

# Other Useful Direnv Use Cases

Besides GitHub CLI, direnv can help with:

* per-project API keys
    
* AWS\_PROFILE switching
    
* Docker/Kubernetes environment variables
    
* custom PATH values
    
* preventing variable leakage across projects
    

But GitHub CLI was my main motivation.

---

# Requirements

* PowerShell 7+
    
* Git for Windows (provides the Bash executable direnv uses)
    

---

# Step 1: Install Direnv

```pwsh
winget install direnv.direnv
```

Restart the terminal afterwards.

---

# Step 2: Configure XDG Directories in PowerShell

Direnv expects Linux-style XDG paths. Add them in your PowerShell profile:

```pwsh
code $PROFILE
```

Paste at the top:

```pwsh
# --- Direnv Configuration ---
$env:XDG_CONFIG_HOME = "$env:USERPROFILE\.config"
$env:XDG_DATA_HOME   = "$env:USERPROFILE\.local\share"
$env:XDG_CACHE_HOME  = "$env:USERPROFILE\.cache"

# Optional: Reduce direnv logging
$env:DIRENV_LOG_FORMAT = ""

# Load direnv into PowerShell
direnv hook pwsh | Out-String | Invoke-Expression
```

---

# Step 3: Create the Required Directories

```pwsh
New-Item -ItemType Directory -Force -Path `
  "$env:USERPROFILE\.config\direnv", `
  "$env:USERPROFILE\.local\share\direnv", `
  "$env:USERPROFILE\.cache\direnv"
```

---

# Step 4: Fixing `exit status 127` (Choosing the Correct Bash)

Direnv evaluates `.envrc` using Bash. On Windows, multiple Bash binaries exist:

* WSL Bash (uses `/mnt/c/...`)
    
* System32 Bash (legacy)
    
* **Git Bash (best for Windows paths)**
    

Create this file:

```pwsh
code "$env:USERPROFILE\.config\direnv\direnv.toml"
```

Add:

```toml
[global]
bash_path = "C:\\Program Files\\Git\\bin\\bash.exe"
```

If Git is installed elsewhere, adjust accordingly.

---

# Step 5: Verify the Setup

```pwsh
mkdir direnv_test; cd direnv_test

"export MY_API_KEY='ItWorks'" | Out-File .envrc -Encoding utf8
direnv allow

$env:MY_API_KEY
```

Cleanup:

```pwsh
cd ..
Remove-Item direnv_test -Recurse -Force
```

---

# How Direnv Fixed My GitHub CLI Problem (Actual `.envrc` Files)

### For internal GitHub Enterprise repos

`.envrc`:

```bash
export GH_HOST="github.mycompany.com"
```

### For public GitHub

```bash
export GH_HOST="github.com"
```

### Why this works

Without `GH_HOST`, GitHub CLI guesses the host — and with two remotes, that fails.

With direnv:

```plaintext
cd repo   → direnv loads GH_HOST → gh uses correct API host  
cd out    → GH_HOST removed → clean environment
```

This solved:

* missing remotes
    
* incorrect API host selection
    
* PR/issue failures
    
* authentication confusion
    

Everything became predictable.

---

# Optional: Direnv Audit Script

```pwsh
Write-Host "--- DIRENV SETUP AUDIT ---" -ForegroundColor Cyan

# Check Installation
Write-Host "`n[Step 1] Checking Version:" -ForegroundColor Yellow
try { direnv version } catch {
    Write-Host "ERROR: direnv not found in PATH." -ForegroundColor Red
}

# Check Profile
Write-Host "`n[Step 2] Checking PowerShell Profile:" -ForegroundColor Yellow
if (Test-Path $PROFILE) {
    $profileContent = Get-Content $PROFILE -Raw
    if ($profileContent -match "XDG_CONFIG_HOME" -and $profileContent -match "direnv hook pwsh") {
        Write-Host "Profile looks correctly configured." -ForegroundColor Green
    } else {
        Write-Host "Possible missing config." -ForegroundColor Yellow
    }
}

# Check Directories
Write-Host "`n[Step 3] Checking XDG Directories:" -ForegroundColor Yellow
$dirs = @(
    "$env:USERPROFILE\.config\direnv",
    "$env:USERPROFILE\.local\share\direnv",
    "$env:USERPROFILE\.cache\direnv"
)
foreach ($dir in $dirs) {
    if (Test-Path $dir) { Write-Host "Found: $dir" -ForegroundColor Green }
    else { Write-Host "Missing: $dir" -ForegroundColor Red }
}

# Check TOML
Write-Host "`n[Step 4] Checking direnv.toml:" -ForegroundColor Yellow
$tomlPath = "$env:USERPROFILE\.config\direnv\direnv.toml"
if (Test-Path $tomlPath) {
    $content = Get-Content $tomlPath -Raw
    Write-Host $content -ForegroundColor Gray
    if ($content -match "bash_path") {
        Write-Host "bash_path configured." -ForegroundColor Green
    } else {
        Write-Host "bash_path missing." -ForegroundColor Red
    }
} else {
    Write-Host "direnv.toml not found." -ForegroundColor Red
}

Write-Host "`n--- AUDIT COMPLETE ---" -ForegroundColor Cyan
```

---

# Frequently Asked Questions (FAQ)

### **Why does GitHub CLI ignore my enterprise remote?**

Because the CLI picks [`github.com`](http://github.com) as the default API host unless `GH_HOST` is set.

### **What does GH\_HOST do?**

It tells GitHub CLI which API endpoint to use (enterprise or public GitHub).

### **Can direnv load GH\_HOST automatically?**

Yes — put it in `.envrc`, and direnv applies it when you enter the repo.

### **Why does direnv require Git Bash on Windows?**

Because Git Bash understands Windows paths (`C:\Users\...`), unlike WSL or System32 bash.

### **What causes exit status 127 in direnv?**

Usually direnv is calling the wrong Bash. Setting `bash_path` fixes this.

---

# Reflection

This setup has been reliable for me. Direnv finally solved the GitHub CLI confusion that happened whenever a repo had both internal and public remotes. The correct `GH_HOST` loads automatically, and everything resets cleanly when I leave the folder.

It keeps my workflow predictable — and prevents API calls from accidentally hitting the wrong host.