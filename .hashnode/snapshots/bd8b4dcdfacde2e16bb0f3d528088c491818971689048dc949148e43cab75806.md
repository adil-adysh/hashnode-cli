---
title: "The Hnsync Test Protocol: A Comprehensive Guide"
slug: "hnsync-test-protocol"
subtitle: "Testing metadata, code blocks, and images in one go."
series: "Engineering Logs"
tags: ["testing", "hashnode", "cli", "go"]
published: false
cover_image: "" 
canonical_url: ""
# This is a comment in the frontmatter to test the parser's robustness
---

# The Hnsync Test Protocol

This is a **bold** statement. This is an *italic* statement. And this is a [link to Google](https://google.com) to ensure standard Markdown rendering works.

## 1. Testing Lists

We need to ensure the HTML conversion handles nested lists correctly:

* Item 1
* Item 2
    * Sub-item A (Testing nesting)
    * Sub-item B

## 2. Testing Code Blocks

This is the most important part for a developer blog. Does the syntax highlighting work?

```go
package main

import "fmt"

func main() {
    // This checks if the parser preserves indentation
    fmt.Println("Hello, Hashnode!")
}
