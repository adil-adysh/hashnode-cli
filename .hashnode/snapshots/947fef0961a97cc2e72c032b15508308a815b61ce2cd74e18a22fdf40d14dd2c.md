# Implementation Spike: Hashnode API Behaviors

**Status:** Draft
**Date:** 2025-12-29
**Goal:** Verify critical assumptions about the Hashnode GraphQL API to prevent architectural failure in `hnsync`.

## 1. Series Deletion Behavior
* **Risk Level:** CRITICAL
* **Hypothesis:** Deleting a Series (`removeSeries`) does **NOT** delete the contained Posts. It only orphans them.
* **Test:**
    1. Create Series "Spike Test".
    2. Add Post "Test Post" to it.
    3. Delete Series "Spike Test".
    4. Query Post "Test Post".
* **Result:** [PENDING / CONFIRMED / BUSTED]
* **Impact:**
    * If CONFIRMED: We can delete unused series safely in `apply`.
    * If BUSTED: We must implement "Rescue Logic" to remove posts from series before deleting the series.

## 2. Draft Visibility
* **Risk Level:** HIGH
* **Hypothesis:** The `publication.posts` query returns drafts if the correct Auth Token is used.
* **Test:**
    1. Create a draft on Hashnode manually.
    2. Run query `posts(filter: { ... })` with token.
* **Result:** [PENDING]
* **Impact:**
    * If BUSTED: We cannot support "Importing" drafts.

## 3. Image Uploads
* **Risk Level:** HIGH
* **Hypothesis:** An endpoint exists to upload a binary file and get a URL without attaching it to a post immediately.
* **Test:** Search docs/schema for `upload` mutation.
* **Result:** [PENDING]
* **Impact:**
    * If BUSTED: We cannot support local images (`![img](./local.png)`). Users must host images externally.

## 4. HTML to Markdown Fidelity
* **Risk Level:** MEDIUM
* **Hypothesis:** `godown` or similar libs can convert Hashnode's HTML back to Markdown without causing a "Diff Loop" (where the content looks different every sync).
* **Test:** Import a post with complex formatting (tables, code blocks), save it, run `plan`.
* **Result:** [PENDING]
