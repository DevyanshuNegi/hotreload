PLAN.md - Hot Reload Engine Implementation Guide
Context for Agent:
You are an expert Go backend engineer. We are building a CLI tool called hotreload that watches a Go project for file changes, rebuilds it, and restarts the server.
Strict Rules:

Use ONLY the Go standard library and github.com/fsnotify/fsnotify.

Do not use any existing hot-reload frameworks (air, reflex, realize).

Use log/slog for all logging.

Implement this strictly phase-by-phase. Do not jump ahead. Wait for the user to verify and commit before moving to the next phase.

Phase 1: CLI Scaffolding & Logging Foundation
Goal: Parse inputs and establish the entry point.

Create main.go.

Use the flag package to parse three mandatory string arguments:

--root: Directory to watch.

--build: Build command.

--exec: Run command.

Validate that none of the flags are empty. If they are, print usage instructions and exit.

Set up a global logger using log/slog (structured logging).

Agent prompt: "Execute Phase 1. Write the code, ensure it builds, and stop."

Phase 2: Advanced File Watching & Filtering
Goal: Watch the file system recursively and ignore garbage.

Create a watcher package.

Initialize fsnotify.NewWatcher().

Implement a recursive directory walk starting from --root. Add all directories to the watcher.

Bonus - Dynamic Directories: Listen for fsnotify.Create events. If a newly created item is a directory, automatically add it to the watcher. If a directory is deleted, remove it.

Bonus - Filtering: Create an ignore list. Ignore events from:

.git/

node_modules/

Any file not ending in .go

Temporary editor files (e.g., ending in ~ or .swp).

Agent prompt: "Execute Phase 2. Ensure recursive watching and the filter logic works cleanly. Stop for review."

Phase 3: Event Pipeline & Debouncing
Goal: Prevent rapid-fire rebuilds when an editor saves multiple files at once.

Create an event pipeline connecting the watcher to the build engine.

Implement a debounce mechanism (e.g., using time.AfterFunc or a timer in a goroutine).

If multiple file change events happen within a ~500ms window, squash them into a single trigger event.

Trigger an initial synthetic "change" event immediately on startup to satisfy the "first build immediately" requirement.

Agent prompt: "Execute Phase 3. Wire up the debouncer to the watcher and log when a valid, debounced trigger occurs. Stop for review."

Phase 4: The Build Engine with Cancellation
Goal: Run the build command and cancel it if a new change happens mid-build.

Create a runner or build package.

When a debounced trigger is received, parse the --build command (e.g., go build -o bin/server ./cmd/server) and run it using exec.CommandContext.

Stream the build logs directly to os.Stdout and os.Stderr (no buffering).

Crucial Requirement: Keep track of the current build context. If a new trigger arrives while a build is currently running, call the context's cancel() function to discard the previous build, and start the new one.

Agent prompt: "Execute Phase 4. Implement the context-cancellable build engine. Ensure logs stream in real-time. Stop for review."

Phase 5: Execution Engine & Process Management
Goal: Run the compiled binary, stream logs, and kill it ruthlessly when needed.

Once a build succeeds, parse and run the --exec command.

Stream server logs directly to os.Stdout/os.Stderr.

Crucial Requirement (Process Groups): When restarting the server, ensure the old process and ALL its child processes are killed. Set SysProcAttr = &syscall.SysProcAttr{Setpgid: true} on the command. To kill it, send syscall.SIGKILL to the negative Process Group ID (-pid).

Ensure resources are fully freed (wait for the process to exit) before starting the new instance.

Bonus - Stability: If the server exits with a non-zero code in under 2 seconds, do not attempt to restart it automatically. Log an error and wait for the next file change.

Agent prompt: "Execute Phase 5. Wire the successful build into the execution engine. Implement the process group killing logic carefully. Stop for review."

Phase 6: Demo Environment & Polish
Goal: Create the required test assets for submission.

Create a testserver/ directory containing a simple Go HTTP server (e.g., an endpoint returning "Hello World v1").

Make sure the test server purposefully spawns a background goroutine or child process to prove the process-group killing works.

Create a Makefile in the root directory with a demo target that runs:
go run main.go --root ./testserver --build "go build -o ./testserver/bin ./testserver/..." --exec "./testserver/bin"

Agent prompt: "Execute Phase 6. Build the test server and Makefile to match the exact command requirements."
