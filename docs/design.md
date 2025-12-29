
# Engineering Design Document: `hnsync`

**Project:** Hashnode Git Sync (`hnsync`)
**Status:** Approved for Implementation
**Date:** December 29, 2025
**Language:** Go

## 1. Executive Summary

`hnsync` is an **Infrastructure-as-Code (IaC)** CLI tool for Hashnode. It allows developers to manage their blog content using a local Git repository as the **Single Source of Truth**.

It solves "Configuration Drift" by implementing a rigorous `Plan` → `Apply` workflow (inspired by Terraform), ensuring that local Markdown files are deterministically synced to Hashnode without accidental data loss.

---

## 2. System Architecture

The system relies on a **Reconciliation Loop** to synchronize three distinct states:

1. **Desired State:** Your local Markdown files.
2. **Known State:** The `.hnsync.lock` file (Snapshot of the last sync).
3. **Actual State:** The live data on Hashnode's API.

### 2.1 The Sync Lifecycle

Every operation follows this strict sequence:

1. **Refresh (Read-Only):** Fetch metadata (ID, UpdatedAt) from Hashnode to detect remote changes (e.g., browser edits).
2. **Plan (Logic):** Compare **Local** vs. **Refreshed Remote** to generate a diff (Create, Update, Delete).
3. **Apply (Write):** Execute GraphQL mutations and update the Lock File.

---

## 3. Data Models

### 3.1 The Source of Truth (Local Markdown)

Files are pure Markdown with YAML Frontmatter.

* **Constraint:** Files **must not** contain Hashnode IDs. The file name or Slug is the unique key.
* **Location:** `./posts/*.md`

```yaml
---
title: "The Architecture of hnsync"
slug: "hnsync-architecture"
series: "Building Tools"     # Referenced by Name, not ID
published: false             # The master switch for "Draft" vs "Public"
tags: ["go", "cli", "engineering"]
---

```

### 3.2 The State File (The Lock)

A custom, line-based text file (similar to `go.sum`). It maps local filenames to remote IDs and tracks content hashes to detect changes.

* **Format:** Space-separated columns.
* **Location:** `./.hnsync.lock`

```text
# TYPE   HASH (SHA256)      IDENTIFIER (Key)       REMOTE_ID    STATUS
SER      sha256:8f4b...     "Building Tools"       s_12345      active
ART      sha256:a1b2...     posts/intro.md         p_98765      published
ART      sha256:c3d4...     posts/draft-v1.md      p_54321      draft

```

### 3.3 The Remote Model

* **API:** Hashnode GraphQL v2 (`gql.hashnode.com`).
* **Auth:** Bearer Token via `HASHNODE_TOKEN` environment variable.

---

## 4. CLI Command Specification

### 4.1 `hnsync init`

Initializes a new project.

* Creates `.hnsync.lock` (empty).
* Creates `hnsync.yaml` (config for Publication ID).

### 4.2 `hnsync import` (Onboarding)

**Goal:** Bootstrap a repository from an existing blog.

1. Fetch all posts from the API.
2. Convert HTML body to Markdown (using a converter library).
3. Generate `.md` files + Frontmatter.
4. Populate `.hnsync.lock`.

### 4.3 `hnsync plan` (The Safety Net)

**Goal:** Visualization.

* Runs implicit **Refresh**.
* Calculates diffs.
* **Output:** colored TUI (Green `+`, Yellow `~`, Red `-`).

### 4.4 `hnsync apply`

**Goal:** Execution.

1. **Pre-flight:** Re-runs logic to ensure no drift occurred since the last plan.
2. **Execution:**
* **Phase 1 (Assets):** Upload local images (Future Scope).
* **Phase 2 (Series):** Create missing series; resolve Series IDs.
* **Phase 3 (Posts):** Create/Update posts via GraphQL.


3. **Post-flight:** Write to `.hnsync.lock`.

---

## 5. Critical Technical Assumptions (SPIKE List)

*These assumptions must be verified before writing core logic.*

| ID | Assumption | Risk | Mitigation |
| --- | --- | --- | --- |
| **A1** | **Series Safety:** Deleting a Series does *not* delete its Posts. | **Critical** | Verify in Playground. If false, implement "Rescue" logic. |
| **A2** | **Draft Access:** The API allows fetching `published: false` posts via the standard query. | **High** | Verify token permissions return drafts. |
| **A3** | **Content Fidelity:** HTML  Markdown conversion is accurate enough to prevent "Diff Loops" (endless updates). | **Medium** | Accept that `import` is "lossy" but `apply` is exact. |

---

## 6. Implementation Stack (Go)

| Component | Library | Purpose |
| --- | --- | --- |
| **CLI Framework** | `spf13/cobra` | Standard command structure. |
| **TUI / Styling** | `charmbracelet/lipgloss` | Beautiful "Terraform-style" output. |
| **Frontmatter** | `adrg/frontmatter` | robust YAML parsing. |
| **Diff Engine** | `google/go-cmp` | Semantic text comparison. |
| **GraphQL** | `hasura/go-graphql-client` | Type-safe API interactions. |

---

## 7. Security & Privacy

1. **Tokens:** Never stored in files. Only read from process Environment (`HASHNODE_TOKEN`).
2. **Lock File:** Safe to commit to public Git (contains IDs/Hashes, but no secrets).
3. **Drafts:** Synced to Hashnode servers but kept hidden via the `published: false` flag.

## 8. Development Roadmap

1. **Phase 1 (Skeleton):** Project structure, `state` parser, and Hashnode API client.
2. **Phase 2 (Read):** Implement `import` and `plan` (Read-only logic).
3. **Phase 3 (Write):** Implement `apply` (Mutations) for basic text.
4. **Phase 4 (Polish):** Series management and TUI refinements.
