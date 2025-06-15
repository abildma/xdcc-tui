<!--
  Title: xdcc-tui
  Description: An interactive TUI tool for xdcc file search and retrieval.
  Author: ostafen (original), ayeah (TUI version)

 <meta name="google-site-verification" content="4Rjg8YnufgHBYdLu-gAUsmJasHk03XKYhUXtRMNZdsk" />
-->

# XDCC-CLI

A fast, keyboard-driven XTDC search & download tool for the terminal.

* 💻  Bubble Tea TUI with paging & live download progress (speed / %)
* 🔎  Instant search over multiple XDCC indexers (xdcc.eu, SunXDCC …)
* ⬇️  Queue several packs at once; transfers run concurrently

> Built with Go 1.19+, Bubble Tea, Lipgloss and fluffle/goirc.

This project provides a user-friendly terminal interface to search and download files from IRC networks through the XDCC protocol. It is based on the popular [goirc](https://github.com/fluffle/goirc) library and features a modern TUI (Terminal User Interface) for improved usability.

## Features
- Interactive TUI with keyboard navigation
- File search from multiple search engines
- Multiple file selection and batch downloads
- Real-time search results and download progress
- Visual file selection with checkboxes

## Installation

### Prerequisites
* Go 1.19+ in your `$PATH`
* `git`

### Quick run (no compile step)

```bash
git clone https://github.com/abildma/xdcc-cli.git
cd xdcc-cli
go run ./cmd            # launches the TUI immediately
```

### Build binary

```bash
make                 # outputs ./bin/xdcc
# or manually
GO111MODULE=on go build -o xdcc ./cmd
```
---

### Disclaimer
This software is provided **as-is** for educational purposes. The authors take no responsibility for how you use it.

This software has been written as a development exercise and comes with no warranty. Use it at your own risk.
Moreover, the developer is not responsible for any illecit use of it.
