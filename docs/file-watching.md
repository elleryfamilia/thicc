# File System Watching

THICC uses `fsnotify` to automatically keep the file browser, quick find index, and file pickers in sync with external file system changes.

## Architecture

```
External file change (create/delete/rename/modify)
        |
        v
    fsnotify
        |
        v
   FileWatcher (internal/filemanager/watcher.go)
        |
        +-- Check if path is in SkipDirs --> ignore if yes
        |
        v
   Debounce (100ms window)
        |
        v
   onChange callback
        |
        +---> Tree.Refresh() --> File browser updates
        |
        +---> FileIndex.Refresh() --> Quick find rebuilds
```

## Components

### FileWatcher (`internal/filemanager/watcher.go`)

The core watcher component that wraps `fsnotify`:

```go
type FileWatcher struct {
    watcher    *fsnotify.Watcher
    root       string
    skipDirs   map[string]bool
    onChange   func()
    debounceMs int
    stop       chan struct{}
    stopped    bool
    mu         sync.Mutex
}
```

**Key features:**
- **Recursive watching**: Walks the directory tree and adds watches for each directory
- **Skip list filtering**: Ignores directories in `SkipDirs`
- **Debouncing**: Batches rapid changes within 100ms to prevent excessive refreshes
- **Dynamic watch addition**: Automatically watches newly created directories

### SkipDirs (`internal/filemanager/tree.go`)

A comprehensive list of directories to skip during scanning and watching. Currently contains 164 entries covering:

| Category | Count | Examples |
|----------|-------|----------|
| Version Control | 6 | `.git`, `.svn`, `.hg`, `.fossil` |
| JavaScript/Node | 24 | `node_modules`, `.next`, `.vite`, `.turbo` |
| Python | 17 | `__pycache__`, `.venv`, `.mypy_cache` |
| Rust | 2 | `target`, `.cargo` |
| Java/Kotlin/Scala | 7 | `.gradle`, `.m2`, `.bsp`, `.metals` |
| .NET/C# | 5 | `bin`, `obj`, `packages`, `.nuget` |
| Mobile (iOS) | 5 | `DerivedData`, `Pods`, `xcuserdata` |
| Mobile (Android) | 5 | `.cxx`, `.externalNativeBuild`, `intermediates` |
| Flutter/Dart | 5 | `.dart_tool`, `.pub-cache` |
| Elixir/Erlang | 5 | `_build`, `deps`, `.elixir_ls` |
| Haskell | 3 | `.stack-work`, `dist-newstyle` |
| C/C++/CMake | 7 | `CMakeFiles`, `cmake-build-*`, `_deps` |
| Cloud/DevOps | 12 | `.terraform`, `.aws-sam`, `.vercel`, `cdk.out` |
| IDEs/Editors | 11 | `.idea`, `.vscode`, `.vs`, `.fleet` |
| Build/Dist | 12 | `dist`, `build`, `coverage`, `.cache` |
| Misc | ~38 | Various language-specific dirs |

## Integration Points

### Tree (File Browser)

```go
// Enable watching after tree loads
tree.SetOnRefresh(func() {
    // Trigger UI redraw
})
tree.EnableWatching()

// Cleanup on close
tree.Close()
```

### FileIndex (Quick Find)

```go
// Enable watching after index builds
fileIndex.EnableWatching()

// Cleanup on close
fileIndex.Close()
```

## Performance Considerations

### Why Skip Directories?

1. **Watch count limits**: Linux has a per-user limit on inotify watches (typically 8192-65536)
2. **Event storms**: `node_modules` can have 100,000+ files; watching all would overwhelm the system
3. **Irrelevant changes**: Build outputs and caches aren't source files users need to see update

### Debouncing

The 100ms debounce window prevents:
- Multiple refreshes during `npm install` (thousands of files)
- Rapid rebuilds during file save operations
- UI flicker from fast successive updates

### Watch Strategy

- **Directories only**: fsnotify reports file changes via their parent directory
- **O(1) skip check**: HashMap lookup for each directory name
- **Lazy watching**: New directories are watched when created

## Adding New Skip Directories

To add directories for a new language/framework, edit `SkipDirs` in `internal/filemanager/tree.go`:

```go
var SkipDirs = map[string]bool{
    // ... existing entries ...

    // ===================
    // YOUR LANGUAGE
    // ===================
    "your-cache-dir": true,
    "your-build-dir": true,
}
```

**Guidelines for adding entries:**
1. **Cache directories**: Anything that's regenerated (e.g., `__pycache__`)
2. **Dependency directories**: Package manager outputs (e.g., `node_modules`)
3. **Build outputs**: Compiled artifacts (e.g., `target`, `dist`)
4. **IDE-specific**: Editor cache/config that changes frequently

**Do NOT skip:**
- Source directories users need to see
- Directories with mixed content (like `vendor` in Go projects)

## Troubleshooting

### File changes not detected

1. Check if directory is in `SkipDirs`
2. Verify watch was added: look for `THICC Watcher: Started watching` in logs
3. Check system watch limits: `cat /proc/sys/fs/inotify/max_user_watches` (Linux)

### Too many refreshes

1. Increase debounce time in `NewFileWatcher` (default 100ms)
2. Add frequently-changing directories to `SkipDirs`

### High CPU/memory

1. Add large generated directories to `SkipDirs`
2. Check for recursive symlinks in watched directories

## Platform Notes

| Platform | Backend | Notes |
|----------|---------|-------|
| Linux | inotify | Watch limits configurable via sysctl |
| macOS | kqueue/FSEvents | Generally higher limits than Linux |
| Windows | ReadDirectoryChangesW | Works but may have latency |

## Future Improvements

- [ ] Per-project skip list configuration
- [ ] .gitignore parsing for dynamic skip list
- [ ] Watch count monitoring and warnings
- [ ] Configurable debounce time
