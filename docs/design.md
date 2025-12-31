Here is the **Complete Design Document** for the **Hashnode Git-Like CLI (`hnsync`)**.

This architecture incorporates the **First Principles** (Identity, Intent, Delta) and the **Senior Improvements** (Smart Renames, Series Ordering, Frontmatter Supremacy).

---

# Design Document: Hashnode Sync (`hnsync`)

## 1. System Philosophy

`hnsync` treats a Hashnode publication as a compilation target. The local filesystem is the "Source Code," and the Hashnode API is the "Production Environment."

* **Ledger (`sum.yml`) is the Database:** It persists the identity and relationships of content. It is committed to Git.
* **Frontmatter is Truth:** Titles, tags, and canonical URLs are defined in the Markdown file, not the CLI config.
* **Series are Ordered Lists:** The Ledger defines the reading order of series, not the file timestamps.
* **Atomic Transitions:** State is only updated after a confirmed API success.

---

## 2. Directory Structure

```text
my-blog-repo/
├── .hashnode/
│   ├── sum.yml         # THE LEDGER (Committed to Git)
│   ├── stage.yml       # THE STAGE  (Ignored by Git)
│   └── logs/           # Debug logs (Ignored)
├── posts/
│   ├── go-concurrency/
│   │   ├── intro.md
│   │   └── channels.md
│   └── my-life-story.md
├── .gitignore          # Must include .hashnode/stage.yml
└── README.md

```

---

## 3. Data Schema (The "Brain")

### 3.1 The Ledger (`.hashnode/sum.yml`)

This file maps Local Paths to Remote Identities. It is the single source of truth for "what is synced."

```yaml
version: 1

# SERIES: The "Container" and "Order" manager
series:
  # Key: Local Slug (Derived from name or explicit config). Stable ID.
  "mastering-go":
    remote_id: "s_5678"          # Empty "" if Local-Only
    name: "Mastering Go"         # Cached display name
    # The List defines the precise order on Hashnode
    articles:
      - "posts/go-concurrency/intro.md"
      - "posts/go-concurrency/channels.md"

# ARTICLES: The "Content" manager
articles:
  # Key: Repo-Relative Path.
  "posts/go-concurrency/intro.md":
    remote_id: "p_1234"          # Empty "" if Local-Only
    checksum: "sha256:abc12345"  # Hash of ENTIRE file (Frontmatter + Body) at last sync

  "posts/go-concurrency/channels.md":
    remote_id: "p_9999"
    checksum: "sha256:xyz98765"

```

### 3.2 The Stage (`.hashnode/stage.yml`)

This file captures "Intent." It is transient and deleted after a push.

```yaml
version: 1

items:
  "posts/go-concurrency/intro.md":
    operation: "UPDATE"          # ADD/UPDATE, DELETE, or REORDER
    checksum: "sha256:def56789"  # The hash at the moment of staging

  "mastering-go":
    operation: "SERIES_META"     # Update name/slug/description

```

---

## 4. Core Workflows (The Logic)

### 4.1 `status` (The Observer)

This command runs automatically before `stage` or `plan`. It detects the state of the world.

1. **Scan Disk:** Find all `.md` files. Hash them.
2. **Load Ledger:** Load `sum.yml`.
3. **Compute Delta:**
* **Modified:** File on Disk has different hash than Ledger.
* **Deleted:** File in Ledger missing from Disk.
* **Untracked:** File on Disk not in Ledger.


4. **Smart Rename Detection (Heuristic):**
* If `File A` is **Deleted** AND `File B` is **Untracked**...
* AND `Hash(File A) == Hash(File B)`...
* **Inference:** It's a **RENAME**.
* **Action:** Update Ledger immediately (Key change from A to B). Preserve `RemoteID`.



### 4.2 `stage add <path>` (The Intent)

1. **Validation:** Ensure path exists.
2. **Dependency Check:**
* Parse Frontmatter. Does it define `series: mastering-go`?
* If yes, is "mastering-go" in the Ledger?
* If no, **Auto-Stage** the creation of the Series entry in the Ledger (Local-Only).


3. **Write Stage:** Save path and current checksum to `stage.yml`.

### 4.3 `plan` (The Diff Engine)

This generates the "To-Do List" for the execution phase.

**Logic:**
Compare **Stage** vs. **Ledger**.

| Scenario | Stage | Ledger | Plan Action |
| --- | --- | --- | --- |
| **New Post** | Present | Missing | `CREATE Article` |
| **Draft Publish** | Present | Present (`RemoteID: ""`) | `CREATE Article` (Promote) |
| **Update** | Present | Present (`RemoteID: "uuid"`) | `UPDATE Article` |
| **Reorder** | Series Staged | Present | `REORDER Series` (Sync article list) |
| **Delete** | Marked Remove | Present | `DELETE Article` |

### 4.4 `push` (The Executor)

This performs the Atomic State Transition.

1. **Locking:** Create `.hashnode/lock` (PID) to prevent concurrent pushes.
2. **Pre-Flight:** Verify all "Parent" dependencies (Series) are either Synced or in the Plan to be created *before* their children.
3. **Execution Loop:**
* **Step 1: Create Series** (if any). Store new `RemoteID` in memory.
* **Step 2: Create/Update Articles.**
* If Article is new, use `RemoteID` from API response.
* If Article belongs to Series, pass the `SeriesID` in the API payload.


* **Step 3: Update Series Order.**
* If the Series list changed, call Hashnode API to set the sort order.




4. **Commit Phase (Critical):**
* Update `sum.yml` with new `RemoteIDs` and `Checksums`.
* **Write to Disk.**
* Delete `stage.yml`.
* Release Lock.



---

## 5. Edge Case Matrix

| Edge Case | Solution |
| --- | --- |
| **User Renames File** | `status` detects Hash match. Updates Ledger key automatically. |
| **Fresh Clone** | Ledger exists (Git). Files exist. `status` hashes files. If match -> "Synced". If diff -> "Modified". |
| **Series Dependency** | If user stages Article but Series is unpublished, `plan` auto-injects Series creation step. |
| **Network Failure** | `sum.yml` is **NOT** updated for failed items. `stage.yml` is **NOT** cleared. User runs `push` again to retry. |
| **Remote-Only Post** | `import` command queries API. Finds post. Writes entry to `sum.yml`. If file missing, downloads it. |
| **Concurrent Push** | Lock file prevents local concurrency. Hashnode API prevents slug collision (returns 400). CLI catches 400 and suggests `import` or `rename slug`. |

---

## 6. Implementation Plan

We will build this in 4 distinct phases:

1. **Phase 1: The Core (State)**
* Implement `Ledger` struct and `sum.yml` persistence.
* Implement `Smart Rename` logic.


2. **Phase 2: The Observer (Status/Stage)**
* Implement `Scan` and `Hash`.
* Implement `stage` command.


3. **Phase 3: The Brain (Plan)**
* Implement the diff logic between Stage and Ledger.


4. **Phase 4: The Executor (Push)**
* Mock API interface.
* Implement atomic Ledger updates.



**Would you like me to start by generating the `state` package code for the `Ledger` and `Smart Rename` logic?**