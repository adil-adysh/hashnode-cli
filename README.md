# Hashnode CLI (`hn`)

> **Git-inspired workflow for Hashnode content management**

A production-ready CLI tool that brings Git's elegant staging workflow to Hashnode blog management. Write locally in Markdown, stage your changes, review diffs, and sync to production—just like deploying code.

[![Go Version](https://img.shields.io/badge/go-1.24.6-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

---

## 🎯 Overview

**Hashnode CLI** (`hn`) treats your blog like a codebase. Your local Markdown files are the source of truth, and the Hashnode API is your production environment. This design enables:

- **Version Control**: Track every content change in Git
- **Editorial Workflows**: PR reviews for blog posts
- **Safe Deployments**: Plan-before-apply prevents accidents
- **Offline Authoring**: Write without internet, sync when ready
- **Content Deduplication**: Automatic snapshot management with garbage collection

### Philosophy

- **Ledger is Truth**: `hashnode.sum` tracks the canonical state of your published content
- **Staging is Intent**: `.hashnode/stage.yml` declares what you want to change
- **Snapshots are Immutable**: Content-addressable storage prevents duplication
- **Frontmatter is Source**: Titles, slugs, and metadata live in your Markdown files

---

## ✨ Key Features

### Git-Like Workflow
```bash
hn stage posts/my-article.md    # Stage your changes
hn plan                          # Review what will happen
hn apply                         # Sync to Hashnode
```

### Content-Addressable Storage
- SHA256-based snapshot system (`.hashnode/snapshots/`)
- Automatic deduplication of identical content
- Built-in garbage collection to clean orphaned snapshots

### Transaction Safety
- Atomic ledger updates prevent partial state
- Lock-based concurrency prevents conflicts
- Automatic rollback on API failures

### Developer Experience
- Colored diff output for change review
- Fast operations with intelligent caching
- Comprehensive error messages
- Dry-run mode for all destructive operations

---

## 🚀 Installation

### From Source
```bash
go install adil-adysh/hashnode-cli/cmd/hashnode-cli@latest
```

### Build Locally
```bash
git clone https://github.com/adil-adysh/hashnode-cli.git
cd hashnode-cli
go build -o hn ./cmd/hashnode-cli
```

---

## 🎬 Quick Start

### 1. Initialize Repository
```bash
# Set your Hashnode API token
export HASHNODE_TOKEN="your-token-here"

# Initialize the repository
hn init
```

This creates:
- `hashnode.sum` - The ledger (tracks synced content)
- `.hashnode/` - State directory (add to `.gitignore`)

### 2. Import Existing Content (Optional)
```bash
hn import
```

Downloads all posts from your Hashnode publication and creates:
- Local Markdown files with frontmatter
- Ledger entries for each post
- Snapshots for content deduplication

### 3. Stage Changes
```bash
# Stage specific file
hn stage posts/my-article.md

# Stage entire directory
hn stage posts/

# Stage for deletion
hn stage delete posts/old-article.md

# View staged changes
hn stage list
```

### 4. Review Changes
```bash
hn plan
```

Shows a colored diff of:
- 🟢 **CREATE**: New posts to publish
- 🟡 **UPDATE**: Modified posts to sync
- 🔴 **DELETE**: Posts marked for removal
- ⚪ **SKIP**: Unchanged content

### 5. Apply Changes
```bash
# Dry run first (recommended)
hn apply --dry-run

# Apply changes
hn apply

# Apply without confirmation for deletions
hn apply --yes
```

---

## 📖 Commands Reference

### Core Workflow

| Command | Description |
|---------|-------------|
| `hn init` | Initialize repository with Hashnode configuration |
| `hn import` | Import all posts from your Hashnode publication |
| `hn stage <path>` | Stage files for sync (creates snapshots) |
| `hn stage delete <path>` | Mark post for deletion |
| `hn unstage <path>` | Remove file from staging area |
| `hn stage list` | View all staged, tracked, and untracked files |
| `hn plan` | Preview changes without applying |
| `hn apply` | Execute staged changes (sync to Hashnode) |

### Maintenance

| Command | Description |
|---------|-------------|
| `hn gc` | Garbage collect unreferenced snapshots |
| `hn gc --dry-run` | Preview what would be removed |
| `hn gc --verify` | Verify integrity of referenced snapshots |

### Series Management

| Command | Description |
|---------|-------------|
| `hn series create <name>` | Create a new series (idempotent by name) |

---

## 📂 Project Structure

```
my-blog/
├── .hashnode/                   # State directory (gitignore this)
│   ├── snapshots/               # Content-addressable storage
│   │   └── abc123def456.md      # SHA256-named immutable snapshots
│   ├── stage.yml                # Staging area (what to sync)
│   └── hashnode.lock            # Concurrency lock
├── hashnode.sum                 # Ledger (commit to git)
├── posts/
│   ├── intro-to-go.md
│   └── advanced-patterns.md
└── .gitignore
```

### File Descriptions

- **`hashnode.sum`**: Ledger tracking synced content (path → remote ID + checksum)
- **`.hashnode/stage.yml`**: Staging area declaring sync intent
- **`.hashnode/snapshots/`**: Immutable content snapshots (SHA256-named)
- **`.hashnode/hashnode.lock`**: Prevents concurrent `apply` operations

---

## 📝 Markdown Format

Posts use standard Markdown with YAML frontmatter:

```markdown
---
title: "Understanding Go Interfaces"
slug: "go-interfaces-deep-dive"
series: "Learning Go"
published: true
tags: ["go", "programming", "backend"]
canonical: "https://myblog.dev/go-interfaces"
---

# Understanding Go Interfaces

Your content here...
```

### Frontmatter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `title` | ✅ | Post title (used for display and routing) |
| `slug` | ❌ | URL slug (auto-generated from title if omitted) |
| `series` | ❌ | Series name (creates series if doesn't exist) |
| `published` | ❌ | `true` for published, `false` for draft (default: `false`) |
| `tags` | ❌ | Array of tag strings |
| `canonical` | ❌ | Canonical URL for syndicated content |

---

## 🏗️ Architecture

### The Git-Inspired Model

```
┌─────────────┐      ┌─────────────┐      ┌──────────────┐      ┌─────────────┐
│   Working   │      │   Staging   │      │    Ledger    │      │   Remote    │
│    Tree     │─────▶│    Area     │─────▶│ (hashnode.  │─────▶│  (Hashnode  │
│  (Disk)     │stage │  (Intent)   │apply │    sum)      │ API  │     API)    │
└─────────────┘      └─────────────┘      └──────────────┘      └─────────────┘
     📝 Edit            🎯 Declare           📋 Track            ☁️ Production
```

### Data Flow

1. **Stage**: Create immutable snapshots of local files
2. **Plan**: Diff snapshots against ledger (last known state)
3. **Apply**: Execute API calls and atomically update ledger
4. **GC**: Remove unreferenced snapshots automatically

### Key Design Principles

- **Single Source of Truth**: Ledger (`hashnode.sum`) is canonical
- **Immutable Snapshots**: Content never changes once captured
- **Atomic Transactions**: Ledger updates are all-or-nothing
- **Intent-Based Staging**: Stage declares "what" not "how"

---

## 🔒 Concurrency & Safety

### Lock File
`hashnode.lock` prevents multiple `apply` operations from running simultaneously:
```text
<pid>:<timestamp>
```

Automatically acquired on `apply`, released on completion or error.

### Transaction Safety
All ledger updates are batched and applied atomically. If any API call fails:
- Ledger remains unchanged
- Stage is preserved
- Lock is released
- User can retry after fixing issues

### Snapshot Protection
The `.hashnode/` directory is automatically protected:
- Cannot be staged accidentally
- Skipped during `hn stage posts/`
- Validation errors prevent manual staging

---

## 🧹 Garbage Collection

Snapshots accumulate over time as you edit content. The GC system cleans unreferenced snapshots:

```bash
# Preview what would be removed
hn gc --dry-run

# Remove unreferenced snapshots
hn gc

# Verify referenced snapshots are intact
hn gc --verify
```

### Automatic GC
Garbage collection runs automatically after:
- Staging operations (`hn stage`)
- Unstaging operations (`hn unstage`)
- Apply operations (`hn apply`)

Output shows cleanup statistics when snapshots are removed:
```
🧹 Removed 3 old snapshot(s)
```

---

## 🛠️ Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `HASHNODE_TOKEN` | ✅ | Your Hashnode API token ([Get it here](https://hashnode.com/settings/developer)) |

You can also pass the token via CLI flag:
```bash
hn apply --token "your-token"
```

### .gitignore Recommendations

```gitignore
# Hashnode CLI state (don't commit)
.hashnode/

# Keep the ledger (commit this)
!hashnode.sum
```

---

## 🧪 Development

### Prerequisites
- Go 1.24.6 or later
- Hashnode API token

### Building
```bash
# Build binary
go build -o hn ./cmd/hashnode-cli

# Run tests
go test ./...

# Run specific test
go test ./internal/state -v -run TestStageWorkflow

# Run with coverage
go test ./... -coverprofile=coverage.out
```

### Project Layout
```
hashnode-cli/
├── cmd/hashnode-cli/           # CLI commands (cobra)
│   ├── main.go                 # Entry point
│   ├── root.go                 # Root command
│   ├── init.go                 # hn init
│   ├── import.go               # hn import
│   ├── stage.go                # hn stage/*
│   ├── plan.go                 # hn plan
│   ├── apply.go                # hn apply
│   ├── gc.go                   # hn gc
│   └── series.go               # hn series
├── internal/
│   ├── api/                    # GraphQL client (genqlient)
│   │   ├── queries.graphql     # Query definitions
│   │   └── generated.go        # Generated client
│   ├── state/                  # Core state management
│   │   ├── stage.go            # Staging operations
│   │   ├── sum.go              # Ledger management
│   │   ├── snapshot.go         # Content-addressable storage
│   │   ├── lock.go             # Concurrency control
│   │   ├── frontmatter.go      # YAML parsing & title resolution
│   │   ├── io.go               # File I/O utilities
│   │   └── *_test.go           # Comprehensive tests
│   ├── diff/                   # Plan generation
│   │   └── diff.go             # Diff algorithm
│   └── log/                    # Logging utilities
├── scripts/                    # Development scripts
│   └── schema/                 # GraphQL schema reference
├── tools/                      # Internal tools
└── docs/                       # Design documents
```

### Running from Source
```bash
# Use go run
go run ./cmd/hashnode-cli stage posts/

# Or use taskfile
task run -- stage posts/
```

### Regenerating GraphQL Client
When `internal/api/queries.graphql` changes:
```bash
go run github.com/Khan/genqlient
```

---

## 🧩 Advanced Usage

### Working with Series

Create a series and add posts:
```bash
# Create series
hn series create "Learning Go"

# Add to posts via frontmatter
cat > posts/part1.md <<EOF
---
title: "Go Basics"
series: "Learning Go"
published: true
---
# Content here
EOF

hn stage posts/part1.md
hn apply
```

### Bulk Operations

Stage entire directory:
```bash
hn stage posts/
```

Unstage everything:
```bash
hn unstage .
```

### Handling Conflicts

If remote content was edited outside the CLI:
1. Import to refresh local state: `hn import`
2. Review changes: `git diff`
3. Resolve conflicts manually
4. Re-stage and apply

### Dry Run Everything

Always test destructive operations first:
```bash
hn apply --dry-run
hn gc --dry-run
```

---

## 🔧 Troubleshooting

### "Lock file exists"
Another `apply` operation is running or crashed. Check if process is still active:
```bash
# Windows
tasklist | findstr <pid>

# Linux/Mac
ps aux | grep <pid>
```

If not running, manually remove:
```bash
rm .hashnode/hashnode.lock
```

### "Cannot find project root"
Ensure you're in a directory with:
- `hashnode.sum` file, OR
- `.hashnode/` directory

Or run `hn init` to initialize.

### "Snapshot validation failed"
A snapshot file is corrupted. Run GC with verification:
```bash
hn gc --verify
```

This will detect and report integrity issues.

### "Title not found"
The CLI couldn't resolve a post title. This happens when:
- Frontmatter is missing `title` field
- Snapshot is corrupted
- Ledger is out of sync

**Fix**: Ensure frontmatter has `title` field, or re-import:
```bash
hn import
```

---

## 📊 Performance Characteristics

- **Staging**: O(n) where n = number of files
- **Planning**: O(m) where m = number of staged items
- **Apply**: O(k) where k = number of API calls (sequential)
- **GC**: O(s + r) where s = snapshots, r = references

### Benchmarks (Typical)
- Stage 100 files: ~500ms
- Plan with 50 changes: ~50ms
- Apply 10 updates: ~3s (network-bound)
- GC 1000 snapshots: ~200ms

---

## 🤝 Contributing

Contributions are welcome! This project follows standard Go conventions.

### Guidelines
1. Run tests: `go test ./...`
2. Format code: `go fmt ./...`
3. Lint: `go vet ./...`
4. Write tests for new features
5. Update documentation

### Submitting PRs
1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Submit a PR with clear description

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- Inspired by Git's staging model and Terraform's plan-apply workflow
- Built with [Cobra](https://github.com/spf13/cobra) for CLI
- Uses [genqlient](https://github.com/Khan/genqlient) for type-safe GraphQL
- Powered by [Hashnode's GraphQL API](https://api.hashnode.com)

---

## 📚 Additional Resources

- [Design Document](docs/design.md) - Deep dive into architecture
- [Hashnode API Documentation](https://api.hashnode.com)
- [Go Documentation](https://golang.org/doc/)

---

**Built with ❤️ for the Hashnode community**

## 🔒 Concurrency & Safety

### Lock File
`hashnode.lock` prevents multiple `apply` operations from running simultaneously:
```text
<pid>:<timestamp>
```

Automatically acquired on `apply`, released on completion or error.

### Transaction Safety
All ledger updates are batched and applied atomically. If any API call fails:
- Ledger remains unchanged
- Stage is preserved
- Lock is released
- User can retry after fixing issues

### Snapshot Protection
The `.hashnode/` directory is automatically protected:
- Cannot be staged accidentally
- Skipped during `hn stage posts/`
- Validation errors prevent manual staging

---

## 🧹 Garbage Collection

Snapshots accumulate over time as you edit content. The GC system cleans unreferenced snapshots:

```bash
# Preview what would be removed
hn gc --dry-run

# Remove unreferenced snapshots
hn gc

# Verify referenced snapshots are intact
hn gc --verify
```

### Automatic GC
Garbage collection runs automatically after:
- Staging operations (`hn stage`)
- Unstaging operations (`hn unstage`)
- Apply operations (`hn apply`)
* **Logic:**
1. Fetch all posts from Hashnode.
2. Convert HTML/JSON body to Markdown.
3. Generate `.md` files with correct Frontmatter.
4. Populate `.hnsync.lock`.


### 4.3 `hn plan` (The Safety Net)

* **Goal:** Show the user what *will* happen.
* **Output (TUI):**
* <span style="color:green">+ CREATE:</span> `new-post.md` (Draft)
* <span style="color:yellow">~ UPDATE:</span> `existing.md` (Content changed)
* <span style="color:red">- DELETE:</span> `old.md` (File removed locally)



### 4.4 `hn apply`

* **Goal:** Execute the changes.
* **Logic:**
1. Run the Plan logic again (implicit safety check).
2. Iterate through changes:
* **Images:** Upload local images found in content -> Replace URL in memory.
* **Series:** Resolve/Create Series ID from Name.
* **Post:** Execute `createPost` or `updatePost`.


3. Write new `HASH` to `.hnsync.lock`.



---

## 5. Technical Implementation (Go)

### 5.1 Stack

* **Language:** Go 1.22+
* **CLI Framework:** `spf13/cobra`
* **UI/Styling:** `charmbracelet/lipgloss` (Critical for the "Terraform-like" output).
* **Diffing:** `google/go-cmp`
* **Frontmatter:** `adrg/frontmatter`

### 5.2 Folder Structure

```text
hnsync/
├── cmd/
│   ├── root.go          # Entry point
│   ├── plan.go
│   ├── apply.go
│   └── import.go
├── internal/
│   ├── state/           # Parser for .hnsync.lock
│   ├── provider/        # Hashnode GraphQL Client
│   ├── diff/            # Logic to compare Local vs Remote
│   └── markdown/        # Frontmatter & Image processing
├── go.mod
└── main.go

```

---

## 6. Critical Workflows

### 6.1 Draft to Publish

1. User changes `published: false` → `published: true` in Frontmatter.
2. `hn plan` detects:
* Content Hash: Match (or mismatch).
* Metadata Change: `published` changed.


3. `hn apply`:
* Sends `updatePost` mutation.
* Updates Lock File status column.


### 6.2 Series "Garbage Collection"

* **Policy:** **Reference Counting.**
* The tool calculates if a Series is used by *any* local file.
* If a Series exists in Lock File but has 0 local references:
* Mark as "Orphaned" in Plan.
* **Action:** Explicitly delete or ignore (Configurable via `--prune-series`).


---

## 7. Security & Privacy

* **Tokens:** Never stored in any file. Only read from process Environment (`HASHNODE_TOKEN`).
* **Drafts:** Drafts are synced to Hashnode but are not visible to the public. The "Source of Truth" for privacy is the `published` boolean.

## 8. Future Scope (V2)

* **Inline Image Hosting:** Auto-upload local images (`./img.png`) to Hashnode CDN during apply.
* **Canonical URL Sync:** Auto-sync canonical URLs if cross-posting to Dev.to.
* **Multi-Provider:** Support Dev.to or Medium using the same Markdown source.

<current_datetime>2025-12-29T16:17:13.316Z</current_datetime>
