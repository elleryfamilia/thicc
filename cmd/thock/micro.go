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
	"github.com/ellery/thock/internal/action"
	"github.com/ellery/thock/internal/buffer"
	"github.com/ellery/thock/internal/clipboard"
	"github.com/ellery/thock/internal/config"
	"github.com/ellery/thock/internal/layout"
	"github.com/ellery/thock/internal/screen"
	"github.com/ellery/thock/internal/shell"
	"github.com/ellery/thock/internal/util"
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
	thockLayout *layout.LayoutManager
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
func LoadInput(args []string) []*buffer.Buffer {
	// There are a number of ways micro should start given its input

	// 1. If it is given a files in flag.Args(), it should open those

	// 2. If there is no input file and the input is not a terminal, that means
	// something is being piped in and the stdin should be opened in an
	// empty buffer

	// 3. If there is no input file and the input is a terminal, an empty buffer
	// should be opened

	buffers := make([]*buffer.Buffer, 0, len(args))

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
			buf, err := buffer.NewBufferFromFileWithCommand(files[i], buffer.BTDefault, command)
			if err != nil {
				screen.TermMessage(err)
				continue
			}
			// If the file didn't exist, input will be empty, and we'll open an empty buffer
			buffers = append(buffers, buf)
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

	return buffers
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
	log.Println("THOCK: After InitLog")

	err = config.InitConfigDir(*flagConfigDir)
	log.Println("THOCK: After InitConfigDir")
	if err != nil {
		screen.TermMessage(err)
	}

	log.Println("THOCK: Before InitRuntimeFiles")
	config.InitRuntimeFiles(true)
	log.Println("THOCK: Before InitPlugins")
	config.InitPlugins()
	log.Println("THOCK: After InitPlugins")

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

	log.Println("THOCK: Before screen.Init")
	err = screen.Init()
	log.Println("THOCK: After screen.Init")
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
				fmt.Println("Micro encountered an error:", errors.Wrap(err, 2).ErrorStack(), "\nIf you can reproduce this error, please report it at https://github.com/zyedidia/micro/issues")
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

	log.Println("THOCK: Before preinit")
	err = config.RunPluginFn("preinit")
	if err != nil {
		screen.TermMessage(err)
	}
	log.Println("THOCK: After preinit")

	log.Println("THOCK: Before InitGlobals")
	action.InitGlobals()
	log.Println("THOCK: After InitGlobals")

	buffer.SetMessager(action.InfoBar)
	args := flag.Args()
	log.Println("THOCK: Before LoadInput, args:", args)
	b := LoadInput(args)
	log.Println("THOCK: After LoadInput, buffers:", len(b))

	// THOCK: Always open at least an empty buffer
	if len(b) == 0 {
		log.Println("THOCK: No buffers, creating empty buffer")
		b = append(b, buffer.NewBufferFromString("", "", buffer.BTDefault))
	}
	log.Println("THOCK: Buffers count:", len(b))

	log.Println("THOCK: Calling InitTabs")
	action.InitTabs(b)
	log.Println("THOCK: InitTabs completed")

	// THOCK: Create layout manager (panels will be initialized later after screen size is known)
	log.Println("THOCK: About to call InitThockLayout")
	InitThockLayout()
	log.Println("THOCK: InitThockLayout completed")

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
		action.Tabs.HandleEvent(event)
	case <-time.After(10 * time.Millisecond):
		// time out after 10ms
	}

	// THOCK: Initialize layout panels now that screen size is known
	if thockLayout != nil {
		log.Println("THOCK: Initializing layout panels")
		if err := thockLayout.Initialize(screen.Screen); err != nil {
			log.Printf("THOCK: Failed to initialize layout: %v", err)
			thockLayout = nil // Fallback to standard micro
		} else {
			log.Println("THOCK: Layout panels initialized successfully")
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

	if thockLayout != nil {
		// THOCK 3-pane layout
		thockLayout.RenderFrame(screen.Screen) // File browser + terminal

		// Constrain editor to middle region
		thockLayout.ConstrainEditorRegion()

		// Render editor tabs in middle region
		action.Tabs.Display()                       // Tab bar (if multiple tabs)
		for _, ep := range action.MainTab().Panes {
			ep.Display() // Editor pane(s)
		}

		// Draw overlays ON TOP of editor (focus border, etc.)
		thockLayout.RenderOverlay(screen.Screen)
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
	if thockLayout != nil && thockLayout.ActivePanel != 1 {
		if thockLayout.ActivePanel == 2 {
			// Terminal focused - re-show terminal cursor (editor overwrote it)
			thockLayout.ShowTerminalCursor(screen.Screen)
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
			if thockLayout != nil {
				w, h := resize.Size()
				thockLayout.Resize(w, h)
				// Constrain editor after resize
				thockLayout.ConstrainEditorRegion()
			}
		} else if action.InfoBar.HasPrompt {
			action.InfoBar.HandleEvent(event)
		} else if thockLayout != nil {
			// THOCK: Try layout manager first
			if !thockLayout.HandleEvent(event) {
				// Layout didn't consume event - pass to tabs for global actions (quit, etc.)
				// Always pass through for editor, and for global keys like Ctrl+Q from any panel
				log.Println("THOCK: Layout returned false, passing to Tabs.HandleEvent")
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

// InitThockLayout initializes the THOCK 3-pane layout (tree + editor + terminal)
func InitThockLayout() {
	log.Println("THOCK: InitThockLayout started")

	// Get current working directory for tree root
	root, err := os.Getwd()
	if err != nil {
		log.Println("THOCK: Error getting working directory:", err)
		root = "."
	}
	log.Println("THOCK: Using root directory:", root)

	// Create layout manager
	thockLayout = layout.NewLayoutManager(root)

	log.Println("THOCK: Layout manager created")

	// Register tree pane actions
	registerThockActions()

	log.Println("THOCK: InitThockLayout completed successfully")

	// TODO: Add tree pane after verifying basic editor works
	// The tree pane integration was causing hangs, so we'll add it back
	// after confirming the basic editor loads properly
}

// registerThockActions registers keybindings for THOCK layout
func registerThockActions() {
	// Register focus commands for new layout
	action.MakeCommand("treefocus", func(bp *action.BufPane, args []string) {
		if thockLayout != nil {
			thockLayout.FocusTree()
			action.InfoBar.Message("Focused file tree")
		}
	}, nil)

	action.MakeCommand("editorfocus", func(bp *action.BufPane, args []string) {
		if thockLayout != nil {
			thockLayout.FocusEditor()
			action.InfoBar.Message("Focused editor")
		}
	}, nil)

	action.MakeCommand("termfocus", func(bp *action.BufPane, args []string) {
		if thockLayout != nil {
			thockLayout.FocusTerminal()
			action.InfoBar.Message("Focused terminal")
		}
	}, nil)

	action.InfoBar.Message("THOCK initialized - Ctrl+Space to switch panels | Ctrl-Q to quit")
}
