# hotreload

A CLI tool that watches a Go project for file changes, rebuilds it, and restarts the server automatically.

## How It Works

```
file change → watcher → debounce (500ms) → build → restart server
```

- **Watcher** monitors a directory recursively for `.go` file changes (ignores `.git`, `node_modules`, temp files)
- **Debouncer** squashes rapid saves into a single rebuild trigger
- **Builder** compiles the project; cancels in-flight builds if a new change arrives
- **Executor** kills the old process and starts the freshly compiled binary

## Prerequisites

- Go 1.26+

## Installation

```bash
git clone https://github.com/DevyanshuNegi/hotreload.git
cd hotreload
go build -o hotreload .
```

## Usage

```bash
hotreload --root <dir> --build <cmd> --exec <cmd>
```

### Flags

| Flag      | Required | Description                              |
|-----------|----------|------------------------------------------|
| `--root`  | Yes      | Directory to watch for file changes      |
| `--build` | Yes      | Build command to compile the project     |
| `--exec`  | Yes      | Path to the compiled binary to run       |

### Example

Watch the included test server:

```bash
# Run directly
go run main.go \
  --root ./testserver \
  --build "go build -o ./testserver/bin ./testserver" \
  --exec "./testserver/bin"

# Or build hotreload first, then use it
go build -o hotreload .
./hotreload \
  --root ./testserver \
  --build "go build -o ./testserver/bin ./testserver" \
  --exec "./testserver/bin"
```

This will:

1. Build and start the test server on `http://localhost:8080`
2. Watch `./testserver/` for any `.go` file changes
3. On change, wait 500ms, rebuild, and restart the server

### Using with your own project

```bash
./hotreload \
  --root ./my-app \
  --build "go build -o ./my-app/server ./my-app/cmd/server" \
  --exec "./my-app/server"
```

## Make Targets

```bash
make demo    # Run hotreload with the test server
make build   # Build the hotreload binary
make clean   # Remove built binaries
```

## Stopping

Press `Ctrl+C` to gracefully shut down both the watched server and hotreload.
