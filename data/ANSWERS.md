# Off-by-One Verified Answer Database

> **312 verified answers** across 242 problem classes | 100% hit rate

| # | Problem | Lang | Preview |
|---|---------|------|---------|
| 1 | unknown | go | The fix is simple — a one-liner using `echo`:  ```bash echo Hello World ```  Or with expli... |
| 2 | js-array-dedupe | go | The most robust approach uses `Array.filter()` with a `Map` as a tracking mechanism. This ... |
| 3 | shell-say-hello-test | go | The problem requires printing "hello world" to stdout. In bash, the `echo` command prints ... |
| 4 | shell-sum-column-numbers | go | To sum a column of numbers from stdin using `awk`, use this one-liner:  ```bash awk '{sum ... |
| 5 | go-string-reverse | go | To correctly reverse a string in Go with Unicode support, you must iterate over **runes** ... |
| 6 | python-count-word-freq | go | The function reads a text file, extracts words using regex (sequences of `[a-zA-Z']+`), st... |
| 7 | shell-find-duplicate-files | go | A concise shell one-liner (and script form) to find duplicate files by MD5 hash in a direc... |
| 8 | shell-echo-date | go | To print the current date in ISO 8601 format (`YYYY-MM-DD`), use one of the following shel... |
| 9 | test-self-dogfood | go | Count lines containing `ERROR` in `log.txt` using `grep -c`:  ```bash grep -c 'ERROR' log.... |
| 10 | off-by-one-self-test | go | The output is exactly `OK` followed by a newline (hex: `4f 4b 0a`), which is the correct a... |
| 11 | off-by-one-self-test | go | The solution is straightforward: print `OK` to stdout. In bash, this is done with:  ```bas... |
| 12 | off-by-one-self-test | bash | Edge cases tested: - `echo "OK"` produces `OK` followed by a newline (hex: `4f 4b 0a`) - `... |
| 13 | self-dogfood-tick23 | bash | Use the `echo` command to print "Hello World" to stdout:  ```bash echo "Hello World" ```  ... |
| 14 | shell-script | bash | Edge cases verified:  | Case | Result | |------|--------| | **Single-digit month** (e.g., ... |
| 15 | shell-script-debug | bash | The most common root cause is **`GLOBIGNORE` being set** in the environment. When `GLOBIGN... |
| 16 | shell-script | bash | The error `test: missing operand` occurs when using `[ -f $file ]` or `test -f $file` with... |
| 17 | echo-hello | bash | The fix is straightforward — use `echo` with the string `hello world`:  ```bash echo hello... |
| 18 | shell-script-e2e | bash | The fix is a simple shell script that prints the exact string `'Hello Off-by-One'` using `... |
| 19 | test | go | No problem was provided beyond the placeholder text "test". There are no code changes, fix... |
| 20 | shell-echo-hello-fix | shell | The fix is straightforward — use `echo` to print "hello world" to stdout.  ```shell echo "... |
| 21 | go-generics-stack | go | The implementation uses Go 1.26 generics with a slice-backed `Stack[T any]` type:  **`stac... |
| 22 | python-async-generator | python | **File: `/home/kara/fibonacci_async.py`**  ```python from typing import AsyncGenerator   a... |
| 23 | js-flatten-deep-object | javascript | The function recursively traverses a nested object, accumulating dot-notation keys as it d... |
| 24 | foreman-tick82-e2e | go | The fix implements a Go HTTP server with a `/healthz` health-check endpoint and comprehens... |
| 25 | foreman-tick82-e2e | go | **Problem Analysis:** Foreman Tick #82 – Host parameter inheritance broken when creating h... |
| 26 | foreman-tick83-e2e | go | The problem `foreman-tick83-e2e` addresses a pagination bug in the Foreman API's Python en... |
| 27 | foreman-tick84-e2e | go | The problem `foreman-tick84-e2e` describes an end-to-end test scenario. Without a concrete... |
| 28 | foreman-tick84-e2e | go | The `foreman-tick84-e2e` problem concerns an end-to-end test failure in a Foreman provisio... |
| 29 | shell-echo-date-real | shell | Use the `date` command with the `--iso-8601=seconds` flag (short form: `-Iseconds`) to pri... |
| 30 | foreman-tick85-e2e | shell | The `foreman-tick85-e2e` self-dogfood E2E test validates pi's ability to modify its own so... |
| 31 | foreman-tick86-e2e | go | The key insight: strings in Go are immutable sequences of bytes (UTF-8). To reverse a stri... |
| 32 | foreman-tick86-e2e-1784889020 | go | The Go function reverses a string by converting it to a slice of runes (which correctly ha... |
| 33 | tick88-foreman-audit | go | The **tick88-foreman-audit** problem requires an E2E (End-to-End) system that audits forem... |
| 34 | reverse-linked-list-tick88 | go | The classic **iterative approach** reverses a singly linked list in O(n) time and O(1) spa... |
| 35 | tick89-e2e | go | The classic two-pointer technique: advance from both ends, skip non-alphanumeric character... |
| 36 | foreman-e2e-verification-pipeline | go | No fix is required. The `foreman-e2e-verification-pipeline` is fully operational at Tick 9... |
| 37 | tick90-e2e | go | The `tick90-e2e` problem validates end-to-end correctness of Go's `time.Ticker`. The solut... |
| 38 | tick90-e2e | python | The function uses **integer binary search** to find an exact integer cube root, avoiding f... |
| 39 | go-worker-pool-rate-limit | go | The implementation is a concurrent worker pool in Go with: - **Configurable rate limiting*... |
| 40 | sql-recursive-cte-org-chart | sql | The recursive CTE uses two parts:  1. **Base case** — employees with no manager (`manager_... |
| 41 | python-streaming-json-parser | python | The file `/home/kara/streaming_json_parser.py` implements a streaming JSON array parser wi... |
| 42 | js-async-pipeline-orchestrator | javascript | ### The Code: `/home/kara/pipeline.js`  The implementation has two main components:  **`ru... |
| 43 | shell-log-analyzer-parser | shell | The script `/home/kara/analyze_nginx.sh` parses nginx combined-format access logs (with op... |
| 44 | go-lru-cache-ttl | go | **File: `/home/kara/lru.go`**  The implementation uses three core data structures for O(1)... |
| 45 | python-sliding-window-rate-limiter | python | ### Architecture  The solution has three layers:  ``` RateLimiter (public API)   ├── InMem... |
| 46 | js-virtual-dom-diff | javascript | The implementation is in `/home/kara/vdom.js`. Here's the architecture:  ### Core Algorith... |
| 47 | python-bloom-filter | python | **File: `/home/kara/bloom_filter.py`**  ```python import math import struct import hashlib... |
| 48 | js-recursive-descent-parser | javascript | ### File: `/home/kara/test-parser/parser.js`  The parser consists of three classes:  **`Le... |
| 49 | go-raft-log-replication | go | The implementation is in `/home/kara/raft/` with two files:  ### `raft.go` — Core Raft Log... |
| 50 | go-tcp-proxy-pool-v2 | go | The implementation is a Go 1.26 TCP proxy (`go-tcp-proxy-pool-v2`) composed of four module... |

... and 262 more answers in [verified-answers.jsonl](./verified-answers.jsonl)
