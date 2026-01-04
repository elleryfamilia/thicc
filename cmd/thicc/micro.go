package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/pprof"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/go-errors/errors"
	isatty "github.com/mattn/go-isatty"
	"github.com/micro-editor/tcell/v2"
	lua "github.com/yuin/gopher-lua"
	"github.com/ellery/thicc/internal/action"
	"github.com/ellery/thicc/internal/aiterminal"
	"github.com/ellery/thicc/internal/buffer"
	"github.com/ellery/thicc/internal/clipboard"
	"github.com/ellery/thicc/internal/config"
	"github.com/ellery/thicc/internal/dashboard"
	"github.com/ellery/thicc/internal/layout"
	"github.com/ellery/thicc/internal/screen"
	"github.com/ellery/thicc/internal/shell"
	"github.com/ellery/thicc/internal/util"
)

var (
	// Command line flags
	flagVersion   = flag.Bool("version", false, "Show the version number and information")
	flagConfigDir = flag.String("config-dir", "", "Specify a custom location for the configuration directory")
	flagOptions   = flag.Bool("options", false, "Show all option help")
	flagDebug     = flag.Bool("debug", false, "Enable debug mode (prints debug info to ./log.txt)")
	flagProfile   = flag.Bool("profile", false, "Enable CPU profiling (writes profile info to ./micro.prof)")
	flagPlugin    = flag.String("plugin", "", "Plugin command")
	flagClean     = flag.Bool("clean", false, "Clean configuration directory")
	optionFlags   map[string]*string

	sighup chan os.Signal

	timerChan chan func()

	// THOCK: Global layout manager
	thiccLayout *layout.LayoutManager

	// THOCK: Dashboard state
	thiccDashboard       *dashboard.Dashboard
	showDashboard        bool
	pendingInstallCmd    string // Install command to run after dashboard exits
)

func InitFlags() {
	// Note: keep this in sync with the man page in assets/packaging/micro.1
	flag.Usage = func() {
		fmt.Println("Usage: micro [OPTION]... [FILE]... [+LINE[:COL]] [+/REGEX]")
		fmt.Println("       micro [OPTION]... [FILE[:LINE[:COL]]]...  (only if the `parsecursor` option is enabled)")
		fmt.Println("-clean")
		fmt.Println("    \tClean the configuration directory and exit")
		fmt.Println("-config-dir dir")
		fmt.Println("    \tSpecify a custom location for the configuration directory")
		fmt.Println("FILE:LINE[:COL] (only if the `parsecursor` option is enabled)")
		fmt.Println("FILE +LINE[:COL]")
		fmt.Println("    \tSpecify a line and column to start the cursor at when opening a buffer")
		fmt.Println("+/REGEX")
		fmt.Println("    \tSpecify a regex to search for when opening a buffer")
		fmt.Println("-options")
		fmt.Println("    \tShow all options help and exit")
		fmt.Println("-debug")
		fmt.Println("    \tEnable debug mode (enables logging to ./log.txt)")
		fmt.Println("-profile")
		fmt.Println("    \tEnable CPU profiling (writes profile info to ./micro.prof")
		fmt.Println("    \tso it can be analyzed later with \"go tool pprof micro.prof\")")
		fmt.Println("-version")
		fmt.Println("    \tShow the version number and information and exit")

		fmt.Print("\nMicro's plugins can be managed at the command line with the following commands.\n")
		fmt.Println("-plugin install [PLUGIN]...")
		fmt.Println("    \tInstall plugin(s)")
		fmt.Println("-plugin remove [PLUGIN]...")
		fmt.Println("    \tRemove plugin(s)")
		fmt.Println("-plugin update [PLUGIN]...")
		fmt.Println("    \tUpdate plugin(s) (if no argument is given, updates all plugins)")
		fmt.Println("-plugin search [PLUGIN]...")
		fmt.Println("    \tSearch for a plugin")
		fmt.Println("-plugin list")
		fmt.Println("    \tList installed plugins")
		fmt.Println("-plugin available")
		fmt.Println("    \tList available plugins")

		fmt.Print("\nMicro's options can also be set via command line arguments for quick\nadjustments. For real configuration, please use the settings.json\nfile (see 'help options').\n\n")
		fmt.Println("-<option> value")
		fmt.Println("    \tSet `option` to `value` for this session")
		fmt.Println("    \tFor example: `micro -syntax off file.c`")
		fmt.Println("\nUse `micro -options` to see the full list of configuration options")
	}

	optionFlags = make(map[string]*string)

	for k, v := range config.DefaultAllSettings() {
		optionFlags[k] = flag.String(k, "", fmt.Sprintf("The %s option. Default value: '%v'.", k, v))
	}

	flag.Parse()

	if *flagVersion {
		// If -version was passed
		fmt.Println("Version:", util.Version)
		fmt.Println("Commit hash:", util.CommitHash)
		fmt.Println("Compiled on", util.CompileDate)
		exit(0)
	}

	if *flagOptions {
		// If -options was passed
		var keys []string
		m := config.DefaultAllSettings()
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := m[k]
			fmt.Printf("-%s value\n", k)
			fmt.Printf("    \tDefault value: '%v'\n", v)
		}
		exit(0)
	}

	if util.Debug == "OFF" && *flagDebug {
		util.Debug = "ON"
	}
}

// DoPluginFlags parses and executes any flags that require LoadAllPlugins (-plugin and -clean)
func DoPluginFlags() {
	if *flagClean || *flagPlugin != "" {
		config.LoadAllPlugins()

		if *flagPlugin != "" {
			args := flag.Args()

			config.PluginCommand(os.Stdout, *flagPlugin, args)
		} else if *flagClean {
			CleanConfig()
		}

		exit(0)
	}
}

// LoadInput determines which files should be loaded into buffers
// based on the input stored in flag.Args()
// Returns:
//   - buffers: the opened file buffers
//   - firstFilePath: path of the first file opened (empty if no files)
//   - fileCount: number of actual files opened (not directories)
func LoadInput(args []string) ([]*buffer.Buffer, string, int) {
	// There are a number of ways micro should start given its input

	// 1. If it is given a files in flag.Args(), it should open those

	// 2. If there is no input file and the input is not a terminal, that means
	// something is being piped in and the stdin should be opened in an
	// empty buffer

	// 3. If there is no input file and the input is a terminal, an empty buffer
	// should be opened

	// 4. If a directory is given, change to that directory and show file browser

	buffers := make([]*buffer.Buffer, 0, len(args))
	firstFilePath := "" // Path of the first file opened
	fileCount := 0      // Count of actual files opened (not directories)

	files := make([]string, 0, len(args))

	flagStartPos := buffer.Loc{-1, -1}
	posFlagr := regexp.MustCompile(`^\+(\d+)(?::(\d+))?$`)
	posIndex := -1

	searchText := ""
	searchFlagr := regexp.MustCompile(`^\+\/(.+)$`)
	searchIndex := -1

	for i, a := range args {
		posMatch := posFlagr.FindStringSubmatch(a)
		if len(posMatch) == 3 && posMatch[2] != "" {
			line, err := strconv.Atoi(posMatch[1])
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			col, err := strconv.Atoi(posMatch[2])
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			flagStartPos = buffer.Loc{col - 1, line - 1}
			posIndex = i
		} else if len(posMatch) == 3 && posMatch[2] == "" {
			line, err := strconv.Atoi(posMatch[1])
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			flagStartPos = buffer.Loc{0, line - 1}
			posIndex = i
		} else {
			searchMatch := searchFlagr.FindStringSubmatch(a)
			if len(searchMatch) == 2 {
				searchText = searchMatch[1]
				searchIndex = i
			} else {
				files = append(files, a)
			}
		}
	}

	command := buffer.Command{
		StartCursor:      flagStartPos,
		SearchRegex:      searchText,
		SearchAfterStart: searchIndex > posIndex,
	}

	if len(files) > 0 {
		// Option 1
		// We go through each file and load it
		for i := 0; i < len(files); i++ {
			// Check if this is a directory
			if info, err := os.Stat(files[i]); err == nil && info.IsDir() {
				// Change to the directory
				absPath, err := filepath.Abs(files[i])
				if err != nil {
					screen.TermMessage(err)
					continue
				}
				if err := os.Chdir(absPath); err != nil {
					screen.TermMessage(err)
					continue
				}
				// Open with file browser in this directory (create empty buffer)
				log.Printf("THICC: Changed to directory: %s, opening with file browser", absPath)
				buffers = append(buffers, buffer.NewBufferFromString("", "", buffer.BTDefault))
				continue
			}

			buf, err := buffer.NewBufferFromFileWithCommand(files[i], buffer.BTDefault, command)
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			// If the file didn't exist, input will be empty, and we'll open an empty buffer
			buffers = append(buffers, buf)
			fileCount++
			// Track the first file's path for file browser root
			if firstFilePath == "" {
				absPath, err := filepath.Abs(files[i])
				if err == nil {
					firstFilePath = absPath
				} else {
					firstFilePath = files[i]
				}
			}
		}
	} else {
		btype := buffer.BTDefault
		if !isatty.IsTerminal(os.Stdout.Fd()) {
			btype = buffer.BTStdout
		}

		if !isatty.IsTerminal(os.Stdin.Fd()) {
			// Option 2
			// The input is not a terminal, so something is being piped in
			// and we should read from stdin
			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				screen.TermMessage("Error reading from stdin: ", err)
				input = []byte{}
			}
			buffers = append(buffers, buffer.NewBufferFromStringWithCommand(string(input), "", btype, command))
		} else {
			// Option 3, just open an empty buffer
			buffers = append(buffers, buffer.NewBufferFromStringWithCommand("", "", btype, command))
		}
	}

	return buffers, firstFilePath, fileCount
}

func checkBackup(name string) error {
	target := filepath.Join(config.ConfigDir, name)
	backup := target + util.BackupSuffix
	if info, err := os.Stat(backup); err == nil {
		input, err := os.ReadFile(backup)
		if err == nil {
			t := info.ModTime()
			msg := fmt.Sprintf(buffer.BackupMsg, target, t.Format("Mon Jan _2 at 15:04, 2006"), backup)
			choice := screen.TermPrompt(msg, []string{"r", "i", "a", "recover", "ignore", "abort"}, true)

			if choice%3 == 0 {
				// recover
				err := os.WriteFile(target, input, util.FileMode)
				if err != nil {
					return err
				}
				return os.Remove(backup)
			} else if choice%3 == 1 {
				// delete
				return os.Remove(backup)
			} else if choice%3 == 2 {
				// abort
				return errors.New("Aborted")
			}
		}
	}
	return nil
}

func exit(rc int) {
	for _, b := range buffer.OpenBuffers {
		if !b.Modified() {
			b.Fini()
		}
	}

	if screen.Screen != nil {
		screen.Screen.Fini()
	}

	os.Exit(rc)
}

func main() {
	defer func() {
		if util.Stdout.Len() > 0 {
			fmt.Fprint(os.Stdout, util.Stdout.String())
		}
		exit(0)
	}()

	var err error

	InitFlags()

	if *flagProfile {
		f, err := os.Create("micro.prof")
		if err != nil {
			log.Fatal("error creating CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("error starting CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	InitLog()
	log.Println("THICC: After InitLog")

	err = config.InitConfigDir(*flagConfigDir)
	log.Println("THICC: After InitConfigDir")
	if err != nil {
		screen.TermMessage(err)
	}

	log.Println("THICC: Before InitRuntimeFiles")
	config.InitRuntimeFiles(true)
	log.Println("THICC: Before InitPlugins")
	config.InitPlugins()
	log.Println("THICC: After InitPlugins")

	err = checkBackup("settings.json")
	if err != nil {
		screen.TermMessage(err)
		exit(1)
	}

	err = config.ReadSettings()
	if err != nil {
		screen.TermMessage(err)
	}
	err = config.InitGlobalSettings()
	if err != nil {
		screen.TermMessage(err)
	}

	// flag options
	for k, v := range optionFlags {
		if *v != "" {
			nativeValue, err := config.GetNativeValue(k, *v)
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			if err = config.OptionIsValid(k, nativeValue); err != nil {
				screen.TermMessage(err)
				continue
			}
			config.GlobalSettings[k] = nativeValue
			config.VolatileSettings[k] = true
		}
	}

	DoPluginFlags()

	log.Println("THICC: Before screen.Init")
	err = screen.Init()
	log.Println("THICC: After screen.Init")
	if err != nil {
		fmt.Println(err)
		fmt.Println("Fatal: Micro could not initialize a Screen.")
		exit(1)
	}
	m := clipboard.SetMethod(config.GetGlobalOption("clipboard").(string))
	clipErr := clipboard.Initialize(m)

	defer func() {
		if err := recover(); err != nil {
			if screen.Screen != nil {
				screen.Screen.Fini()
			}
			if e, ok := err.(*lua.ApiError); ok {
				fmt.Println("Lua API error:", e)
			} else {
				fmt.Println("thicc encountered an error:", errors.Wrap(err, 2).ErrorStack(), "\nIf you can reproduce this error, please report it at https://github.com/elleryfamilia/thicc/issues")
			}
			// immediately backup all buffers with unsaved changes
			for _, b := range buffer.OpenBuffers {
				if b.Modified() {
					b.Backup()
				}
			}
			exit(1)
		}
	}()

	err = config.LoadAllPlugins()
	if err != nil {
		screen.TermMessage(err)
	}

	err = checkBackup("bindings.json")
	if err != nil {
		screen.TermMessage(err)
		exit(1)
	}

	action.InitBindings()
	action.InitCommands()

	log.Println("THICC: Before preinit")
	err = config.RunPluginFn("preinit")
	if err != nil {
		screen.TermMessage(err)
	}
	log.Println("THICC: After preinit")

	log.Println("THICC: Before InitGlobals")
	action.InitGlobals()
	log.Println("THICC: After InitGlobals")

	buffer.SetMessager(action.InfoBar)
	args := flag.Args()
	log.Println("THICC: Before LoadInput, args:", args)

	// THOCK: Check if we should show the dashboard (no args and interactive terminal)
	if len(args) == 0 && isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) {
		log.Println("THICC: No args provided, showing dashboard")
		showDashboard = true
	}

	var b []*buffer.Buffer
	var firstFilePath string // Path of first file opened (for file browser root)
	var fileCount int        // Number of files opened
	if !showDashboard {
		b, firstFilePath, fileCount = LoadInput(args)
		log.Println("THICC: After LoadInput, buffers:", len(b), "firstFilePath:", firstFilePath, "fileCount:", fileCount)

		// THOCK: Always open at least an empty buffer
		if len(b) == 0 {
			log.Println("THICC: No buffers, creating empty buffer")
			b = append(b, buffer.NewBufferFromString("", "", buffer.BTDefault))
		}
		log.Println("THICC: Buffers count:", len(b))

		log.Println("THICC: Calling InitTabs")
		action.InitTabs(b)
		log.Println("THICC: InitTabs completed")
	}

	// THOCK: Create layout manager (panels will be initialized later after screen size is known)
	// Create it now even for dashboard - we'll preload the terminal in the background
	log.Println("THICC: About to call InitThiccLayout")
	InitThiccLayout()
	log.Println("THICC: InitThiccLayout completed")

	err = config.RunPluginFn("init")
	if err != nil {
		screen.TermMessage(err)
	}

	err = config.RunPluginFn("postinit")
	if err != nil {
		screen.TermMessage(err)
	}

	err = config.InitColorscheme()
	if err != nil {
		screen.TermMessage(err)
	}

	if clipErr != nil {
		log.Println(clipErr, " or change 'clipboard' option")
	}

	config.StartAutoSave()
	if a := config.GetGlobalOption("autosave").(float64); a > 0 {
		config.SetAutoTime(a)
	}

	screen.Events = make(chan tcell.Event)

	util.Sigterm = make(chan os.Signal, 1)
	sighup = make(chan os.Signal, 1)
	signal.Notify(util.Sigterm, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT)
	signal.Notify(sighup, syscall.SIGHUP)

	timerChan = make(chan func())

	// Here is the event loop which runs in a separate thread
	go func() {
		for {
			screen.Lock()
			e := screen.Screen.PollEvent()
			screen.Unlock()
			if e != nil {
				screen.Events <- e
			}
		}
	}()

	// clear the drawchan so we don't redraw excessively
	// if someone requested a redraw before we started displaying
	for len(screen.DrawChan()) > 0 {
		<-screen.DrawChan()
	}

	// wait for initial resize event
	select {
	case event := <-screen.Events:
		if !showDashboard {
			action.Tabs.HandleEvent(event)
		}
	case <-time.After(10 * time.Millisecond):
		// time out after 10ms
	}

	// THOCK: Show dashboard if no files were specified
	dashboardWasShown := false
	if showDashboard {
		log.Println("THICC: Starting dashboard")
		dashboardWasShown = true
		InitDashboard()

		// THOCK: Preload terminal in background while dashboard is showing
		// Only preload for default shell - AI tools don't need it (no prompt injection delay)
		// If user changes selection while on dashboard, we'll handle it in TransitionToEditor
		if thiccLayout != nil && thiccDashboard != nil {
			cmd := thiccDashboard.GetSelectedAIToolCommand()
			if cmd == nil {
				// Default shell selected - preload to avoid ugly-to-pretty prompt transition
				w, h := screen.Screen.Size()
				log.Printf("THICC: Preloading shell terminal in background (%dx%d)", w, h)
				thiccLayout.PreloadTerminal(w, h)
			} else {
				log.Printf("THICC: AI tool selected (%v), skipping preload", cmd)
			}
		}

		// Run dashboard event loop
		for showDashboard {
			DoDashboardEvent()
		}

		log.Println("THICC: Dashboard exited, transitioning to editor")
		// TransitionToEditor already initialized the layout, so skip below
	}

	// THOCK: Initialize layout panels now that screen size is known
	// Skip if we just came from dashboard (TransitionToEditor already did this)
	if thiccLayout != nil && !dashboardWasShown {
		if fileCount > 0 {
			// File(s) opened directly - hide terminal, show tool selector when first toggled
			log.Println("THICC: Files opened directly - hiding terminal, will show tool selector on toggle")
			thiccLayout.TerminalVisible = false
			thiccLayout.TerminalInitialized = false

			// Set file browser root to the first file's directory
			if firstFilePath != "" {
				fileDir := filepath.Dir(firstFilePath)
				log.Printf("THICC: Setting file browser root to: %s", fileDir)
				thiccLayout.Root = fileDir
			}

			// Hide file browser when multiple files are opened
			if fileCount > 1 {
				log.Printf("THICC: Multiple files (%d) opened - hiding file browser", fileCount)
				thiccLayout.TreeVisible = false
			}

			// Initialize layout without terminal
			log.Println("THICC: Initializing layout panels (file mode - no terminal)")
			if err := thiccLayout.Initialize(screen.Screen); err != nil {
				log.Printf("THICC: Failed to initialize layout: %v", err)
				thiccLayout = nil // Fallback to standard micro
			} else {
				log.Println("THICC: Layout panels initialized successfully (file mode)")
			}
		} else {
			// Directory or no args (non-dashboard path) - normal initialization with terminal
			// Load AI tool preference for non-dashboard startup
			prefsStore := dashboard.NewPreferencesStore()
			prefsStore.Load()
			if selectedTool := prefsStore.GetSelectedAITool(); selectedTool != "" {
				// Find matching tool and get command line
				// Skip "Shell (default)" - let terminal use its built-in shell handling
				// which includes the pretty prompt injection
				tools := aiterminal.GetAvailableToolsOnly()
				for _, t := range tools {
					if t.Command == selectedTool {
						if t.Name == "Shell (default)" {
							log.Printf("THICC: Default shell selected, using built-in shell handling")
						} else {
							log.Printf("THICC: Setting AI tool command from preferences: %v", t.GetCommandLine())
							thiccLayout.SetAIToolCommand(t.GetCommandLine())
						}
						break
					}
				}
			}

			// Preload terminal to give it time to initialize the pretty prompt
			// The terminal is created in a goroutine, and prompt injection waits 1000ms
			// before sending the source command. We wait 1500ms total to ensure:
			// 1. Terminal creation completes
			// 2. 1000ms prompt injection delay
			// 3. source command executes and clears screen
			w, h := screen.Screen.Size()
			log.Printf("THICC: Preloading terminal before layout init (%dx%d)", w, h)
			thiccLayout.PreloadTerminal(w, h)
			time.Sleep(1500 * time.Millisecond)

			log.Println("THICC: Initializing layout panels")
			if err := thiccLayout.Initialize(screen.Screen); err != nil {
				log.Printf("THICC: Failed to initialize layout: %v", err)
				thiccLayout = nil // Fallback to standard micro
			} else {
				log.Println("THICC: Layout panels initialized successfully")
				// Mark terminal as initialized since it was created with saved preferences
				thiccLayout.TerminalInitialized = true
			}
		}
	}

	for {
		DoEvent()
	}
}

// DoEvent runs the main action loop of the editor
func DoEvent() {
	var event tcell.Event

	// Display everything
	screen.Screen.Fill(' ', config.DefStyle)
	screen.Screen.HideCursor()

	if thiccLayout != nil {
		// THOCK 3-pane layout
		thiccLayout.RenderFrame(screen.Screen) // File browser + terminal

		// Constrain editor to middle region
		thiccLayout.ConstrainEditorRegion()

		// Render editor tabs in middle region
		action.Tabs.Display()                       // Tab bar (if multiple tabs)
		for _, ep := range action.MainTab().Panes {
			ep.Display() // Editor pane(s)
		}

		// Draw overlays ON TOP of editor (focus border, etc.)
		thiccLayout.RenderOverlay(screen.Screen)
	} else {
		// Fallback: Standard micro rendering (for compatibility)
		action.Tabs.Display()
		for _, ep := range action.MainTab().Panes {
			ep.Display()
		}
		action.MainTab().Display()
	}

	action.InfoBar.Display()

	// THOCK: Control cursor visibility based on which panel has focus
	// Editor always shows its cursor during Display(), so we need to override after rendering
	if thiccLayout != nil && thiccLayout.ActivePanel != 1 {
		if thiccLayout.ActivePanel >= 2 && thiccLayout.ActivePanel <= 4 {
			// Terminal focused - re-show terminal cursor (editor overwrote it)
			thiccLayout.ShowTerminalCursor(screen.Screen)
		} else {
			// Tree focused - hide cursor (tree doesn't have a cursor)
			screen.Screen.HideCursor()
		}
	}

	screen.Screen.Show()

	// Check for new events
	select {
	case f := <-shell.Jobs:
		// If a new job has finished while running in the background we should execute the callback
		f.Function(f.Output, f.Args)
	case <-config.Autosave:
		for _, b := range buffer.OpenBuffers {
			b.AutoSave()
		}
	case <-shell.CloseTerms:
		action.Tabs.CloseTerms()
	case event = <-screen.Events:
	case <-screen.DrawChan():
		for len(screen.DrawChan()) > 0 {
			<-screen.DrawChan()
		}
	case f := <-timerChan:
		f()
	case <-sighup:
		exit(0)
	case <-util.Sigterm:
		exit(0)
	}

	if e, ok := event.(*tcell.EventError); ok {
		log.Println("tcell event error: ", e.Error())

		if e.Err() == io.EOF {
			// shutdown due to terminal closing/becoming inaccessible
			exit(0)
		}
		return
	}

	if event != nil {
		if resize, ok := event.(*tcell.EventResize); ok {
			// Handle resize events
			action.InfoBar.HandleEvent(event)
			action.Tabs.HandleEvent(event)

			// THOCK: Resize layout panels
			if thiccLayout != nil {
				w, h := resize.Size()
				thiccLayout.Resize(w, h)
				// Constrain editor after resize
				thiccLayout.ConstrainEditorRegion()
			}
		} else if action.InfoBar.HasPrompt {
			action.InfoBar.HandleEvent(event)
		} else if thiccLayout != nil {
			// THOCK: Try layout manager first
			if !thiccLayout.HandleEvent(event) {
				// Layout didn't consume event - pass to tabs for global actions (quit, etc.)
				// Always pass through for editor, and for global keys like Ctrl+Q from any panel
				log.Println("THICC: Layout returned false, passing to Tabs.HandleEvent")
				action.Tabs.HandleEvent(event)
			}
		} else {
			// Fallback: Standard micro event handling
			action.Tabs.HandleEvent(event)
		}
	}

	err := config.RunPluginFn("onAnyEvent")
	if err != nil {
		screen.TermMessage(err)
	}
}

// InitThiccLayout initializes the THOCK 3-pane layout (tree + editor + terminal)
func InitThiccLayout() {
	log.Println("THICC: InitThiccLayout started")

	// Get current working directory for tree root
	root, err := os.Getwd()
	if err != nil {
		log.Println("THICC: Error getting working directory:", err)
		root = "."
	}
	log.Println("THICC: Using root directory:", root)

	// Create layout manager
	thiccLayout = layout.NewLayoutManager(root)

	log.Println("THICC: Layout manager created")

	// Register tree pane actions
	registerThiccActions()

	log.Println("THICC: InitThiccLayout completed successfully")

	// TODO: Add tree pane after verifying basic editor works
	// The tree pane integration was causing hangs, so we'll add it back
	// after confirming the basic editor loads properly
}

// registerThiccActions registers keybindings for THOCK layout
func registerThiccActions() {
	// Register focus commands for new layout
	action.MakeCommand("treefocus", func(bp *action.BufPane, args []string) {
		if thiccLayout != nil {
			thiccLayout.FocusTree()
			action.InfoBar.Message("Focused file tree")
		}
	}, nil)

	action.MakeCommand("editorfocus", func(bp *action.BufPane, args []string) {
		if thiccLayout != nil {
			thiccLayout.FocusEditor()
			action.InfoBar.Message("Focused editor")
		}
	}, nil)

	action.MakeCommand("termfocus", func(bp *action.BufPane, args []string) {
		if thiccLayout != nil {
			thiccLayout.FocusTerminal()
			action.InfoBar.Message("Focused terminal")
		}
	}, nil)

	action.InfoBar.Message("THICC initialized - Ctrl+Space to switch panels | Ctrl-Q to quit")
}

// InitDashboard initializes the dashboard screen
func InitDashboard() {
	log.Println("THICC: InitDashboard started")

	thiccDashboard = dashboard.NewDashboard(screen.Screen)

	// Set up callbacks
	thiccDashboard.OnNewFile = func() {
		log.Println("THOCK Dashboard: New File selected")
		showDashboard = false
		TransitionToEditor(nil, "")
	}

	thiccDashboard.OnOpenProject = func(path string) {
		log.Println("THOCK Dashboard: Opening project:", path)
		showDashboard = false
		// Change to the project folder
		if err := os.Chdir(path); err != nil {
			log.Printf("THOCK Dashboard: Failed to chdir to %s: %v", path, err)
		}
		TransitionToEditor(nil, "")
		// Add to recent projects
		thiccDashboard.RecentStore.AddProject(path, true)
		// Focus the file browser so user can see the project
		if thiccLayout != nil {
			thiccLayout.FocusTree()
		}
	}

	thiccDashboard.OnOpenFile = func(path string) {
		log.Println("THOCK Dashboard: Opening file:", path)
		showDashboard = false
		TransitionToEditor(nil, path)
		// Add to recent projects
		thiccDashboard.RecentStore.AddProject(path, false)
	}

	thiccDashboard.OnOpenFolder = func(path string) {
		log.Println("THOCK Dashboard: Opening folder:", path)
		showDashboard = false
		// Change to the folder
		if err := os.Chdir(path); err != nil {
			log.Printf("THOCK Dashboard: Failed to chdir to %s: %v", path, err)
		}
		TransitionToEditor(nil, "")
		// Add to recent projects
		thiccDashboard.RecentStore.AddProject(path, true)
	}

	thiccDashboard.OnInstallTool = func(cmd string) {
		log.Printf("THOCK Dashboard: Install tool selected: %s", cmd)
		pendingInstallCmd = cmd
		TransitionToEditor(nil, "") // Initialize editor with empty buffer
		showDashboard = false
	}

	thiccDashboard.OnExit = func() {
		log.Println("THOCK Dashboard: Exit selected")
		exit(0)
	}

	log.Println("THICC: InitDashboard completed")
}

// DoDashboardEvent runs a single iteration of the dashboard event loop
func DoDashboardEvent() {
	// Clear and render dashboard
	screen.Screen.Fill(' ', config.DefStyle)
	screen.Screen.HideCursor()

	thiccDashboard.Render(screen.Screen)
	screen.Screen.Show()

	// Wait for event
	select {
	case event := <-screen.Events:
		if e, ok := event.(*tcell.EventResize); ok {
			w, h := e.Size()
			thiccDashboard.Resize(w, h)
		} else {
			thiccDashboard.HandleEvent(event)
		}
	case <-util.Sigterm:
		exit(0)
	case <-sighup:
		exit(0)
	}
}

// TransitionToEditor switches from dashboard to editor mode
func TransitionToEditor(buffers []*buffer.Buffer, filePath string) {
	log.Println("THICC: TransitionToEditor started")

	// Create buffer(s) if needed
	if len(buffers) == 0 {
		if filePath != "" {
			// Open the specified file
			buf, err := buffer.NewBufferFromFile(filePath, buffer.BTDefault)
			if err != nil {
				log.Printf("THICC: Failed to open file %s: %v", filePath, err)
				// Fall back to empty buffer
				buffers = append(buffers, buffer.NewBufferFromString("", "", buffer.BTDefault))
			} else {
				buffers = append(buffers, buf)
			}
		} else {
			// Create empty buffer
			buffers = append(buffers, buffer.NewBufferFromString("", "", buffer.BTDefault))
		}
	}

	// Initialize tabs with the new buffers
	action.InitTabs(buffers)

	// Initialize the layout (only if not already created)
	if thiccLayout == nil {
		InitThiccLayout()
	}

	// Configure AI tool auto-launch from dashboard preferences (if not already set during preload)
	if thiccLayout != nil && thiccDashboard != nil {
		if cmd := thiccDashboard.GetSelectedAIToolCommand(); cmd != nil {
			log.Printf("THICC: Setting AI tool command from dashboard: %v", cmd)
			thiccLayout.SetAIToolCommand(cmd)
		}
	}

	// Initialize layout panels
	if thiccLayout != nil {
		if err := thiccLayout.Initialize(screen.Screen); err != nil {
			log.Printf("THICC: Failed to initialize layout: %v", err)
			thiccLayout = nil
		} else {
			// Mark Terminal 1 as initialized since tool was selected from dashboard
			thiccLayout.TerminalInitialized = true
		}
	}

	// Handle pending install command from dashboard
	if pendingInstallCmd != "" && thiccLayout != nil {
		log.Printf("THICC: Executing pending install command: %s", pendingInstallCmd)
		thiccLayout.SpawnTerminalWithInstallCommand(pendingInstallCmd)
		pendingInstallCmd = "" // Clear after use
	}

	log.Println("THICC: TransitionToEditor completed")
}
