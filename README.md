# Hashnode CLI (`hn`)

> **Command-line tool for managing Hashnode blogs**

`hn` is a CLI that allows you to manage your Hashnode blog from local Markdown files. You can stage changes, preview planned updates, and apply them to your Hashnode publication.

[![Go Version](https://img.shields.io/badge/go-1.24.6-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache_2.0-blue.svg)](LICENSE)
[![GitHub Repo](https://img.shields.io/badge/github-adil--adysh%2Fhashnode--cli-181717.svg?logo=github&logoColor=white)](https://github.com/adil-adysh/hashnode-cli)

---

## 🚀 Installation

### Windows (Scoop)
```powershell
scoop bucket add hashnode-cli https://github.com/adil-adysh/scoop-bucket.git
scoop install hn
scoop update hn  # update if already installed
hn --help        # verify installation
````

### Build Locally

```bash
git clone https://github.com/adil-adysh/hashnode-cli.git
cd hashnode-cli
go build -o hn ./cmd/hashnode-cli   # Build locally
./hn --help                          # Verify installation
```

### Run via Go

```bash
cd hashnode-cli
go run ./cmd/hashnode-cli stage posts/
```

---

## Quick Start

### 1. Initialize Repository

```bash
export HASHNODE_TOKEN="your-token"
hn init
```

Creates:

* `.hashnode/` — CLI state directory
* `hashnode.sum` — ledger

### 2. Import Existing Posts (Optional)

```bash
hn import
```

* Converts posts to Markdown
* Generates snapshots and ledger entries

### 3. Stage Changes

```bash
hn stage posts/my-article.md   # Stage a file
hn stage posts/                # Stage directory
hn stage delete posts/old.md   # Mark for deletion
hn stage list                  # View staged files
```

### 4. Review Plan

```bash
hn plan
```

Shows:

* 🟢 CREATE — new posts
* 🟡 UPDATE — modified posts
* 🔴 DELETE — marked for deletion
* ⚪ SKIP — unchanged

### 5. Apply Changes

```bash
hn apply --dry-run   # Preview first
hn apply             # Apply changes
hn apply --yes       # Apply without confirmation for deletions
```

---

## Commands Reference

| Command                  | Description                                |
| ------------------------ | ------------------------------------------ |
| `hn init`                | Initialize repository with Hashnode config |
| `hn import`              | Import posts from Hashnode                 |
| `hn stage <path>`        | Stage files for sync                       |
| `hn stage delete <path>` | Mark post for deletion                     |
| `hn unstage <path>`      | Remove from staging                        |
| `hn stage list`          | List staged files                          |
| `hn plan`                | Preview planned changes                    |
| `hn apply`               | Apply staged changes                       |
| `hn gc`                  | Clean unreferenced snapshots               |

> Series commands are not yet supported.

---

## Project Structure

```
my-blog/
├── .hashnode/             # CLI state
│   ├── snapshots/         # SHA256 snapshots
│   ├── stage.yml          # Staging intent
│   └── hashnode.lock      # Concurrency lock
├── hashnode.sum           # Ledger
├── posts/                 # Markdown content
└── .gitignore
```

---

## Markdown Format

```yaml
---
title: "Understanding Go Interfaces"
slug: "go-interfaces-deep-dive"
published: true
tags: ["go","programming"]
canonical: "https://myblog.dev/go-interfaces"
---
```

Required: `title`
Optional: `slug`, `published`, `tags`, `canonical`

---

## Architecture

```
Working Tree → Staging Area → Ledger (hashnode.sum) → Remote (Hashnode API)
```

Flow: Stage → Plan → Apply → Garbage Collect

---

## Safety & Concurrency

* Lock file prevents multiple simultaneous `apply`
* Ledger updates are applied atomically
* Snapshots are protected from accidental staging

---

## Garbage Collection

```bash
hn gc --dry-run   # Preview cleanup
hn gc             # Remove unreferenced snapshots
hn gc --verify    # Verify integrity
```

Automatically runs after staging, unstaging, or applying changes.

---

## Development

* Go 1.24+
* Cobra CLI
* genqlient for GraphQL

Build & run:

```bash
go build -o hn ./cmd/hashnode-cli
go run ./cmd/hashnode-cli stage posts/
```

---

## Contributing

1. Fork & clone
2. Make changes with tests
3. Submit PR with description

---

## License

Apache License 2.0 — see [LICENSE](LICENSE)
