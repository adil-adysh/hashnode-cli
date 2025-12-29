# Implementation Checklist: hnsync

**Goal:** Build a "Content-as-Code" CLI for Hashnode that treats a local Git repo as the Source of Truth.
**Stack:** Go, Cobra, Lipgloss, GraphQL.

---

## Phase 0: The "Spike" (API Verification)
*Critical: Verify these assumptions before writing core logic.*
- [ ] **Auth Check:** Verify `HASHNODE_TOKEN` works with a simple `Me` query in the playground.
- [ ] **Series Safety:** Test if `removeSeries` mutation deletes the posts inside it or just orphans them. (If it deletes posts, we need a major design change).
- [ ] **Draft Visibility:** Verify that `publication.posts` query returns `published: false` items when using an Auth Token.
- [ ] **Image Uploads:** Investigate if there is a standalone `uploadImage` mutation (required for future inline image support).

---

## Phase 1: The Foundation (Skeleton)
- [ ] **Project Setup:**
    - [ ] Run `go mod init github.com/yourname/hnsync`.
    - [ ] Create folder structure (`cmd/`, `internal/state`, `internal/provider`, `internal/diff`).
- [ ] **Configuration:**
    - [ ] Create `internal/config` to load `HASHNODE_TOKEN` from env.
    - [ ] Create `hnsync.yaml` loader (for Publication ID).
- [ ] **State Engine (The Lock File):**
    - [ ] Define `ArticleState` and `SeriesState` structs.
    - [ ] Implement `ParseLockFile(path)` for the custom line-based format (`SER` / `ART`).
    - [ ] Implement `SaveLockFile(path)` that sorts keys alphabetically (deterministic output).

---

## Phase 2: The "Read" Layer (Logic)
- [ ] **Hashnode Provider (Client):**
    - [ ] Setup `hasura/go-graphql-client`.
    - [ ] Implement `GetAllPosts()` (fetching ID, Slug, Title, UpdatedAt).
    - [ ] Implement `GetSeries()` (fetching ID, Name).
- [ ] **Markdown Engine:**
    - [ ] Implement `ParseFile(path)` using `adrg/frontmatter`.
    - [ ] Define the official Frontmatter struct (`slug`, `title`, `published`, `series`).
    - [ ] Implement `CalculateHash(content)` (SHA256).
- [ ] **Diff Calculator (The Brain):**
    - [ ] Create `Plan` struct.
    - [ ] Implement logic: Map Local Files <-> Remote IDs via Lock File.
    - [ ] Implement logic: Detect Drift (`Remote.UpdatedAt` > `Lock.LastSync`).
    - [ ] Implement logic: Determine Action (`CREATE`, `UPDATE`, `DELETE`, `SKIP`).

---

## Phase 3: The CLI (User Experience)
- [ ] **CLI Skeleton:**
    - [ ] Setup `spf13/cobra` root command.
    - [ ] Implement `hnsync init` (generates empty config & lock file).
- [ ] **The `Plan` Command:**
    - [ ] Wire up the "Diff Calculator".
    - [ ] **TUI:** Use `charmbracelet/lipgloss` to render the output table.
        - [ ] Green for `+ CREATE`.
        - [ ] Yellow for `~ UPDATE`.
        - [ ] Red for `- DELETE`.

---

## Phase 4: The "Write" Layer (Execution)
- [ ] **Mutations:**
    - [ ] Implement `CreatePost(input)` mutation.
    - [ ] Implement `UpdatePost(input)` mutation.
    - [ ] Implement `CreateSeries(name)` mutation.
- [ ] **The `Apply` Command:**
    - [ ] Re-run Plan logic (Safety check).
    - [ ] **Step 1:** Resolve Series (Create if missing, get IDs).
    - [ ] **Step 2:** Iterate and Execute Post mutations.
    - [ ] **Step 3:** Update `state.lock` with new IDs and Hashes.
    - [ ] **Step 4:** Save `state.lock` to disk.

---

## Phase 5: Onboarding (Import)
- [ ] **HTML Converter:** Integrate a library like `godown` to convert Hashnode HTML body -> Markdown.
- [ ] **The `Import` Command:**
    - [ ] Fetch all remote posts.
    - [ ] Generate `.md` files in `./posts/`.
    - [ ] Generate Frontmatter from metadata.
    - [ ] Populate the initial lock file to prevent "Double Creation" on next run.

---

## Phase 6: Polish & Distribution
- [ ] **Safety:** Add confirmation prompt ("Do you want to perform these actions? yes/no") to `apply`.
- [ ] **Docs:** Write `README.md` with setup instructions.
- [ ] **Release:** Setup GitHub Action to build binaries (`goreleaser`).

---

## Future Scope (Post-V1)
- [ ] **Inline Image Handling:** Parse Markdown for `./img.png`, upload to Hashnode, replace URL in memory.
- [ ] **Canonical URL Sync:** Support cross-posting logic.
- [ ] **Series Pruning:** Logic to delete remote series if they become empty locally.
