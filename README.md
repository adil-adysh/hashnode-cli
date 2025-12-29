# Engineering Design Document: Hashnode Git Sync (`hnsync`)

**Status:** Draft
**Date:** December 29, 2025
**Author:** [Your Name]

## 1. Overview

`hnsync` is a "Content-as-Code" CLI tool designed to manage Hashnode blogs using a local Git repository as the Single Source of Truth. It adopts the **Infrastructure-as-Code (IaC)** philosophy (inspired by Terraform) to prevent configuration drift and enable rigorous editorial workflows (PR reviews, versioning) for blog content.

### 1.1 Goals

* **Git-Centric:** All content lives in local Markdown files.
* **Deterministic:** A lock file ensures strict mapping between local files and remote posts.
* **Safety:** A "Plan" (Dry Run) phase prevents accidental overwrites or deletions.
* **Developer Experience:** First-class CLI experience with colored diffs and fast execution.

---

## 2. Architecture

The system follows a strict **Reconciliation Loop** pattern: `Refresh -> Plan -> Apply`.

### 2.1 The Reconciliation Loop

1. **Refresh (Read-Only):**
* Fetches lightweight metadata (ID, UpdatedAt, Slug) from Hashnode API.
* Updates the internal memory of "Remote State".
* *Constraint:* Does NOT modify local Markdown files.


2. **Plan (Logic):**
* Compares **Local State** (Markdown + Frontmatter) against **Remote State**.
* Calculates a "Diff" (Create, Update, Delete, Skip).
* Detects "Drift" (e.g., if the remote post was edited in the browser).


3. **Apply (Write):**
* Executes the calculated plan via GraphQL mutations.
* Updates the local **Lock File** with new Hashes and IDs.


---

## 3. Data Models

### 3.1 Local Source (Markdown)

The "User Interface" for the content. Standard Markdown with YAML Frontmatter.

**File:** `posts/my-article.md`

```yaml
---
title: "Understanding Go Interfaces"
slug: "go-interfaces-deep-dive"
series: "Learning Go"       # Reference by Name (not ID)
published: false            # true = Live, false = Draft
tags: ["go", "backend"]
---

# Content starts here...

```

### 3.2 State Management (The Lock File)

A custom, line-based text file to track the mapping between local files and remote IDs.

* **Format:** Custom "Go.sum" style (Space-separated).
* **Location:** `.hnsync.lock` (Committed to Git).

**Schema:**

```text
# TYPE    HASH (SHA256)      IDENTIFIER (Filename/Name)    REMOTE_ID    STATUS
SER       sha256:a1b2...     "Learning Go"                 s_12345      active
ART       sha256:xyz...      posts/intro.md                p_98765      published
ART       sha256:f9e8...     posts/draft.md                p_54321      draft

```

### 3.3 Remote Model (Hashnode API)

* **Endpoint:** `https://gql.hashnode.com`
* **Auth:** Bearer Token via `HASHNODE_TOKEN` env var.
* **Entities:** `Post`, `Series`, `Publication`.

---

## 4. CLI Commands

### 4.1 `hnsync init`

* Creates the initial `.hnsync.lock` and config file.
* Verifies API token.

### 4.2 `hnsync import` (The Onboarding)

* **Goal:** Bootstrap a repo from an existing blog.
* **Logic:**
1. Fetch all posts from Hashnode.
2. Convert HTML/JSON body to Markdown.
3. Generate `.md` files with correct Frontmatter.
4. Populate `.hnsync.lock`.


### 4.3 `hnsync plan` (The Safety Net)

* **Goal:** Show the user what *will* happen.
* **Output (TUI):**
* <span style="color:green">+ CREATE:</span> `new-post.md` (Draft)
* <span style="color:yellow">~ UPDATE:</span> `existing.md` (Content changed)
* <span style="color:red">- DELETE:</span> `old.md` (File removed locally)



### 4.4 `hnsync apply`

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
2. `hnsync plan` detects:
* Content Hash: Match (or mismatch).
* Metadata Change: `published` changed.


3. `hnsync apply`:
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
