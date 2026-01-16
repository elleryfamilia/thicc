package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
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
	"github.com/ellery/thicc/internal/thicc"
	"github.com/ellery/thicc/internal/update"
	"github.com/ellery/thicc/internal/util"
)

var (
	// Command line flags
	flagVersion   = flag.Bool("version", false, "Show the version number and information")
	flagConfigDir = flag.String("config-dir", "", "Specify a custom location for the configuration directory")
	flagOptions   = flag.Bool("options", false, "Show all option help")
	flagDebug     = flag.Bool("debug", false, "Enable debug mode (prints debug info to ./log.txt)")
	flagProfile   = flag.Bool("profile", false, "Enable CPU profiling (writes profile info to ./thicc.prof)")
	flagPlugin    = flag.String("plugin", "", "Plugin command")
	flagClean     = flag.Bool("clean", false, "Clean configuration directory")
	flagUpdate    = flag.Bool("update", false, "Check for updates and install if available")
	flagUninstall = flag.Bool("uninstall", false, "Uninstall thicc from your system")
	flagReportBug = flag.Bool("report-bug", false, "Report a bug or issue")
	optionFlags   map[string]*string

	sighup chan os.Signal

	timerChan chan func()

	// THICC: Global layout manager
	thiccLayout *layout.LayoutManager

	// THICC: Dashboard state
	thiccDashboard       *dashboard.Dashboard
	showDashboard        bool
	pendingInstallCmd    string // Install command to run after dashboard exits
)

func InitFlags() {
	flag.Usage = func() {
		fmt.Println("Usage: thicc [OPTIONS] [PATH]")
		fmt.Println("")
		fmt.Println("  thicc              Open dashboard")
		fmt.Println("  thicc .            Open current directory")
		fmt.Println("  thicc <file>       Open a file")
		fmt.Println("  thicc <dir>        Open a directory")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -version           Show version and exit")
		fmt.Println("  -update            Check for updates and install if available")
		fmt.Println("  -uninstall         Uninstall thicc from your system")
		fmt.Println("  -report-bug        Report a bug or issue")
		fmt.Println("  -clean             Clean configuration directory and exit")
		fmt.Println("  -config-dir <dir>  Use custom configuration directory")
		fmt.Println("  -debug             Enable debug logging to ./log.txt")
		fmt.Println("")
		fmt.Println("Navigation:")
		fmt.Println("  Ctrl+Space         Switch between panes")
		fmt.Println("  Ctrl+Q             Quit")
		fmt.Println("")
		fmt.Println("Panes:")
		fmt.Println("  Alt+1              Toggle file browser")
		fmt.Println("  Alt+2              Toggle editor")
		fmt.Println("  Alt+3              Toggle terminal 1")
		fmt.Println("  Alt+4              Toggle terminal 2")
		fmt.Println("  Alt+5              Toggle terminal 3")
		fmt.Println("")
		fmt.Println("For more info: https://github.com/elleryfamilia/thicc")
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

// DoUpdateFlag handles the --update flag
func DoUpdateFlag() {
	if !*flagUpdate {
		return
	}

	channel := config.GetGlobalOption("updatechannel").(string)
	fmt.Printf("Checking for updates (channel: %s)...\n", channel)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	info, err := update.CheckForUpdate(ctx, channel)
	if err != nil {
		fmt.Printf("Error checking for updates: %v\n", err)
		os.Exit(1)
	}

	if info == nil {
		fmt.Printf("You are already running the latest version (%s)\n", util.Version)
		os.Exit(0)
	}

	fmt.Printf("Update available: %s → %s\n", info.CurrentVersion, info.LatestVersion)
	fmt.Println("Downloading...")

	progressFn := func(downloaded, total int64) {
		if total > 0 {
			pct := int(float64(downloaded) / float64(total) * 100)
			fmt.Printf("\rDownloading... %d%%", pct)
		}
	}

	if err := update.DownloadAndInstall(info, progressFn); err != nil {
		fmt.Printf("\nUpdate failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSuccessfully updated to %s\n", info.LatestVersion)
	fmt.Println("Please restart thicc to use the new version.")
	os.Exit(0)
}

// DoUninstallFlag handles the --uninstall flag
func DoUninstallFlag() {
	if !*flagUninstall {
		return
	}

	// Get the executable path
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Error finding executable: %v\n", err)
		os.Exit(1)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		os.Exit(1)
	}

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "thicc")

	fmt.Println("This will uninstall thicc from your system.")
	fmt.Printf("  Binary: %s\n", execPath)
	fmt.Printf("  Config: %s\n", configDir)
	fmt.Print("\nContinue? [y/N] ")

	var response string
	fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		fmt.Println("Uninstall cancelled.")
		os.Exit(0)
	}

	// Remove config directory
	if _, err := os.Stat(configDir); err == nil {
		fmt.Print("Remove configuration? [y/N] ")
		fmt.Scanln(&response)
		if response == "y" || response == "Y" {
			if err := os.RemoveAll(configDir); err != nil {
				fmt.Printf("Warning: could not remove config: %v\n", err)
			} else {
				fmt.Printf("Removed %s\n", configDir)
			}
		}
	}

	// Remove the binary
	// We need to delete ourselves - this works on Unix because the file
	// can be deleted while running (inode stays until process exits)
	if err := os.Remove(execPath); err != nil {
		if os.IsPermission(err) {
			fmt.Printf("\nPermission denied. Try: sudo thicc -uninstall\n")
		} else {
			fmt.Printf("Error removing binary: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Printf("Removed %s\n", execPath)
	fmt.Println("\nthicc has been uninstalled.")
	os.Exit(0)
}

// SystemInfo contains system information for bug reports
type SystemInfo struct {
	Version     string
	CommitHash  string
	CompileDate string
	GoVersion   string
	OS          string
	Arch        string
	Terminal    string
	TermEnv     string
}

func collectSystemInfo() SystemInfo {
	return SystemInfo{
		Version:     util.Version,
		CommitHash:  util.CommitHash,
		CompileDate: util.CompileDate,
		GoVersion:   runtime.Version(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Terminal:    os.Getenv("TERM_PROGRAM"),
		TermEnv:     os.Getenv("TERM"),
	}
}

func (s SystemInfo) String() string {
	terminal := s.Terminal
	if terminal == "" {
		terminal = "unknown"
	}
	return fmt.Sprintf(`System Info:
  Version: %s
  Commit: %s
  Compiled: %s
  OS/Arch: %s/%s
  Terminal: %s (TERM=%s)
  Go: %s`,
		s.Version, s.CommitHash, s.CompileDate,
		s.OS, s.Arch,
		terminal, s.TermEnv,
		s.GoVersion)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func buildGitHubIssueURL(errorMsg string, info SystemInfo, isCrash bool) string {
	baseURL := "https://github.com/elleryfamilia/thicc/issues/new"

	var template, title string
	if isCrash {
		template = "crash_report.md"
		// Clean up error message for title
		cleanErr := truncateString(errorMsg, 50)
		title = "Crash Report: " + cleanErr
	} else {
		template = "bug_report.md"
		title = "Bug Report"
	}

	body := fmt.Sprintf(`## System Information
%s

## Description
<!-- Describe the bug or what you were doing when the crash occurred -->

`, info.String())

	if isCrash && errorMsg != "" {
		body += fmt.Sprintf("## Error\n```\n%s\n```\n", errorMsg)
	}

	return fmt.Sprintf("%s?template=%s&title=%s&body=%s",
		baseURL,
		url.QueryEscape(template),
		url.QueryEscape(title),
		url.QueryEscape(body))
}

func openBrowser(urlStr string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "linux":
		cmd = exec.Command("xdg-open", urlStr)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func getCrashLogPath() string {
	return filepath.Join(config.ConfigDir, "crash.log")
}

func saveCrashLog(errorMsg, stackTrace string, info SystemInfo) {
	content := fmt.Sprintf(`thicc crash report
Generated: %s

%s

Error: %s

Stack trace:
%s
`, time.Now().Format(time.RFC3339), info.String(), errorMsg, stackTrace)

	_ = os.WriteFile(getCrashLogPath(), []byte(content), 0644)
}

// DoReportBugFlag handles the --report-bug flag
func DoReportBugFlag() {
	if !*flagReportBug {
		return
	}

	fmt.Println("Collecting system information...")
	fmt.Println()
	info := collectSystemInfo()
	fmt.Println(info.String())
	fmt.Println()

	issueURL := buildGitHubIssueURL("", info, false)
	fmt.Println("Opening browser to create issue...")

	if err := openBrowser(issueURL); err != nil {
		fmt.Println("(If browser doesn't open, visit the URL below)")
	}
	fmt.Println()
	fmt.Println(issueURL)
	os.Exit(0)
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
		f, err := os.Create("thicc.prof")
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

	// Load THICC-specific settings early (before colorscheme init)
	log.Println("THICC: Loading THICC settings")
	thicc.LoadSettings()
	config.ReloadThiccBackground()
	config.InitDoubleClickThreshold()
	log.Println("THICC: THICC settings loaded")

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

	// Handle --update flag (needs config loaded for updatechannel setting)
	DoUpdateFlag()

	// Handle --uninstall flag
	DoUninstallFlag()

	// Handle --report-bug flag
	DoReportBugFlag()

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
		fmt.Println("Fatal: thicc could not initialize a screen.")
		exit(1)
	}
	m := clipboard.SetMethod(config.GetGlobalOption("clipboard").(string))
	clipErr := clipboard.Initialize(m)

	defer func() {
		if err := recover(); err != nil {
			if screen.Screen != nil {
				screen.Screen.Fini()
			}

			info := collectSystemInfo()
			var errorMsg string
			var stackTrace string

			if e, ok := err.(*lua.ApiError); ok {
				errorMsg = fmt.Sprintf("Lua API error: %v", e)
				stackTrace = e.StackTrace
			} else {
				errorMsg = fmt.Sprintf("%v", err)
				stackTrace = errors.Wrap(err, 2).ErrorStack()
			}

			// Print crash report
			fmt.Println()
			fmt.Println("thicc encountered an unexpected error!")
			fmt.Println()
			fmt.Printf("Error: %s\n", errorMsg)
			fmt.Println()
			fmt.Println("Stack trace:")
			fmt.Println(stackTrace)
			fmt.Println()
			fmt.Println(info.String())
			fmt.Println()

			// Save crash log (include debug.Stack for full trace)
			fullStack := stackTrace + "\n\nFull runtime stack:\n" + string(debug.Stack())
			saveCrashLog(errorMsg, fullStack, info)

			// Show issue URL
			issueURL := buildGitHubIssueURL(errorMsg, info, true)
			fmt.Println("Please report this issue:")
			fmt.Printf("  %s\n", issueURL)
			fmt.Println()
			fmt.Println("Or run: thicc --report-bug")
			fmt.Printf("\nFull crash log saved to: %s\n", getCrashLogPath())

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

	// THICC: Check if we should show the dashboard (no args and interactive terminal)
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

		// THICC: Always open at least an empty buffer
		if len(b) == 0 {
			log.Println("THICC: No buffers, creating empty buffer")
			b = append(b, buffer.NewBufferFromString("", "", buffer.BTDefault))
		}
		log.Println("THICC: Buffers count:", len(b))

		log.Println("THICC: Calling InitTabs")
		action.InitTabs(b)
		log.Println("THICC: InitTabs completed")
	}

	// THICC: Create layout manager (panels will be initialized later after screen size is known)
	// Only create now if NOT showing dashboard - dashboard will create it after user selects a project
	if !showDashboard {
		log.Println("THICC: About to call InitThiccLayout")
		InitThiccLayout()
		log.Println("THICC: InitThiccLayout completed")
	}

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

	// THICC: Show dashboard if no files were specified
	dashboardWasShown := false
	if showDashboard {
		log.Println("THICC: Starting dashboard")
		dashboardWasShown = true
		InitDashboard()

		// THICC: Preload terminal in background while dashboard is showing
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

	// THICC: Initialize layout panels now that screen size is known
	// Skip if we just came from dashboard (TransitionToEditor already did this)
	if thiccLayout != nil && !dashboardWasShown {
		if fileCount > 0 {
			// File(s) opened directly - show only editor (hide file browser and terminal)
			log.Printf("THICC: File(s) opened directly (%d files) - showing editor only", fileCount)
			thiccLayout.TreeVisible = false
			thiccLayout.TerminalVisible = false
			thiccLayout.TerminalInitialized = false

			// Change working directory to the first file's directory
			// This ensures file browser and terminal start in the right place
			if firstFilePath != "" {
				fileDir := filepath.Dir(firstFilePath)
				log.Printf("THICC: Changing to file's directory: %s", fileDir)
				if err := os.Chdir(fileDir); err != nil {
					log.Printf("THICC: Failed to chdir to %s: %v", fileDir, err)
				}
				thiccLayout.Root = fileDir
			}

			// Initialize layout with editor only
			log.Println("THICC: Initializing layout panels (file mode - editor only)")
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
				// Hide editor for directory-only startup, focus terminal
				log.Println("THICC: Hiding editor, focusing terminal for project startup")
				thiccLayout.EditorVisible = false
				thiccLayout.ActivePanel = 2 // Focus terminal
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
		// THICC 3-pane layout
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

	// InfoBar display disabled for cleaner look
	// action.InfoBar.Display()

	// Render hint bar based on state (passthrough > quick command > multiplexer)
	if thiccLayout != nil {
		if thiccLayout.IsTerminalInPassthroughMode() {
			thiccLayout.RenderPassthroughHint(screen.Screen)
		} else if thiccLayout.QuickCommandMode {
			thiccLayout.RenderQuickCommandHints(screen.Screen)
		} else if thiccLayout.InMultiplexer {
			thiccLayout.RenderMultiplexerHint(screen.Screen)
		}
	}

	// THICC: Control cursor visibility based on which panel has focus
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

			// THICC: Resize layout panels
			if thiccLayout != nil {
				w, h := resize.Size()
				thiccLayout.Resize(w, h)
				// Constrain editor after resize
				thiccLayout.ConstrainEditorRegion()
			}
		} else if action.InfoBar.HasPrompt {
			action.InfoBar.HandleEvent(event)
		} else if thiccLayout != nil {
			// THICC: Try layout manager first
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

// InitThiccLayout initializes the THICC 3-pane layout (tree + editor + terminal)
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

// registerThiccActions registers keybindings for THICC layout
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
		log.Println("THICC Dashboard: New File selected")
		showDashboard = false
		TransitionToEditor(nil, "", true) // Show editor for new file
	}

	thiccDashboard.OnOpenProject = func(path string) {
		log.Println("THICC Dashboard: Opening project:", path)
		showDashboard = false
		// Change to the project folder
		if err := os.Chdir(path); err != nil {
			log.Printf("THICC Dashboard: Failed to chdir to %s: %v", path, err)
		}
		// Reset layout so it gets recreated with the new working directory
		thiccLayout = nil
		TransitionToEditor(nil, "", false) // Hide editor, show file browser + terminal
		// Add to recent projects
		thiccDashboard.RecentStore.AddProject(path, true)
	}

	thiccDashboard.OnOpenFile = func(path string) {
		log.Println("THICC Dashboard: Opening file:", path)
		showDashboard = false
		TransitionToEditor(nil, path, true) // Show editor with file
		// Add to recent projects
		thiccDashboard.RecentStore.AddProject(path, false)
	}

	thiccDashboard.OnOpenFolder = func(path string) {
		log.Println("THICC Dashboard: Opening folder:", path)
		showDashboard = false
		// Change to the folder
		if err := os.Chdir(path); err != nil {
			log.Printf("THICC Dashboard: Failed to chdir to %s: %v", path, err)
		}
		// Reset layout so it gets recreated with the new working directory
		thiccLayout = nil
		TransitionToEditor(nil, "", false) // Hide editor, show file browser + terminal
		// Add to recent projects
		thiccDashboard.RecentStore.AddProject(path, true)
	}

	thiccDashboard.OnNewFolder = func(path string) {
		log.Println("THICC Dashboard: New folder created, opening:", path)
		showDashboard = false
		// Change to the new folder
		if err := os.Chdir(path); err != nil {
			log.Printf("THICC Dashboard: Failed to chdir to %s: %v", path, err)
		}
		// Reset layout so it gets recreated with the new working directory
		thiccLayout = nil
		TransitionToEditor(nil, "", false) // Hide editor, show file browser + terminal
		// Add to recent projects
		thiccDashboard.RecentStore.AddProject(path, true)
	}

	thiccDashboard.OnInstallTool = func(cmd string) {
		log.Printf("THICC Dashboard: Install tool selected: %s", cmd)
		pendingInstallCmd = cmd
		// Don't call TransitionToEditor here - the subsequent action
		// (OnNewFile, OnOpenProject, etc.) will handle the transition
		// and SpawnTerminalWithInstallCommand will use pendingInstallCmd
	}

	thiccDashboard.OnExit = func() {
		log.Println("THICC Dashboard: Exit selected")
		exit(0)
	}

	// Check if this is first run and show onboarding guide
	if !thiccDashboard.PrefsStore.HasSeenOnboarding() {
		log.Println("THICC: First run detected, showing onboarding guide")
		thiccDashboard.ShowOnboardingGuide()
		thiccDashboard.PrefsStore.MarkOnboardingComplete()
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
// showEditor controls whether the editor panel is visible (false = file browser + terminal only)
func TransitionToEditor(buffers []*buffer.Buffer, filePath string, showEditor bool) {
	log.Printf("THICC: TransitionToEditor started (showEditor=%v)", showEditor)

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
		} else if showEditor {
			// Only create empty buffer if editor should be shown
			buffers = append(buffers, buffer.NewBufferFromString("", "", buffer.BTDefault))
		} else {
			// No file and no editor - create minimal placeholder buffer for tabs
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

			// Hide editor and focus terminal if showEditor is false
			if !showEditor {
				log.Println("THICC: Hiding editor, focusing terminal")
				thiccLayout.EditorVisible = false
				thiccLayout.ActivePanel = 2 // Focus terminal
			}
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
