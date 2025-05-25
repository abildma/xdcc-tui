<!--
  Title: xdcc-tui
  Description: An interactive TUI tool for xdcc file search and retrieval.
  Author: ostafen (original), ayeah (TUI version)

 <meta name="google-site-verification" content="4Rjg8YnufgHBYdLu-gAUsmJasHk03XKYhUXtRMNZdsk" />
-->

# XDCC TUI - Interactive XDCC Tools

This project provides a user-friendly terminal interface to search and download files from IRC networks through the XDCC protocol. It is based on the popular [goirc](https://github.com/fluffle/goirc) library and features a modern TUI (Terminal User Interface) for improved usability.

## Features
- Interactive TUI with keyboard navigation
- File search from multiple search engines
- Multiple file selection and batch downloads
- Real-time search results and download progress
- Visual file selection with checkboxes
- Command-line mode for scripting compatibility

## Installation

Assuming you have the latest version of Go installed on your system, you can use the **make** command to build an executable:

```bash 
git clone https://github.com/your-username/xdcc-tui.git
cd xdcc-tui
make # this will output a bin/xdcc executable
```

## Usage

### TUI Mode (Recommended)

To start the interactive TUI mode, simply run:

```bash
foo@bar:~$ xdcc tui
```

This will launch the interactive terminal interface where you can:

1. **Search for files:**
   - Type your search keywords in the search field
   - Press Enter to perform the search
   - Results will appear in a scrollable list

2. **Select files for download:**
   - Use Up/Down arrows to navigate the results
   - Press Enter to select/deselect a file (a checkmark will appear)
   - Select multiple files for batch downloading

3. **Download files:**
   - After selecting files, press 'd' to start downloading
   - Progress will be displayed in real-time

4. **Keyboard shortcuts:**
   - Tab: Toggle between search and results view
   - Enter: Search (in search mode) or select file (in results mode)
   - d: Start download for selected files
   - q: Quit the application

### Command Line Mode (Legacy)

The original command-line interface is still available for scripting or non-interactive use:

#### Search for files

```bash
foo@bar:~$ xdcc search keyword1 keyword2 ...
```

For example, to search for the latest ISO of Ubuntu:

```bash
foo@bar:~$ xdcc search ubuntu iso
```

This displays a table with file name, size, and URL.

#### Download files

```bash
foo@bar:~$ xdcc get url1 url2 ... [-o /path/to/an/output/directory]
```

You can also specify a .txt input file containing a list of URLs (one per line) using the **-i** switch.

## Notes

This software has been written as a development exercise and comes with no warranty. Use it at your own risk.
Moreover, the developer is not responsible for any illecit use of it.