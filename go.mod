module github.com/ellery/thicc

require (
	github.com/blang/semver v3.5.1+incompatible
	github.com/creack/pty v1.1.18
	github.com/dustin/go-humanize v1.0.1
	github.com/fsnotify/fsnotify v1.9.0
	github.com/go-errors/errors v1.0.1
	github.com/google/uuid v1.6.0
	github.com/hinshun/vt10x v0.0.0-20220301184237-5011da428d02
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/mattn/go-isatty v0.0.20
	github.com/mattn/go-runewidth v0.0.16
	github.com/micro-editor/json5 v1.0.1-micro
	github.com/micro-editor/tcell/v2 v2.0.11
	github.com/micro-editor/terminal v0.0.0-20250324214352-e587e959c6b5
	github.com/mitchellh/go-homedir v1.1.0
	github.com/sahilm/fuzzy v0.1.1
	github.com/sergi/go-diff v1.1.0
	github.com/stretchr/testify v1.4.0
	github.com/yuin/gopher-lua v1.1.1
	github.com/zyedidia/clipper v0.1.1
	github.com/zyedidia/glob v0.0.0-20170209203856-dd4023a66dc3
	golang.org/x/image v0.15.0
	golang.org/x/text v0.14.0
	gopkg.in/yaml.v2 v2.2.8
	layeh.com/gopher-luar v1.0.11
	modernc.org/sqlite v1.34.4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gdamore/encoding v1.0.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/lucasb-eyer/go-colorful v1.0.3 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/zyedidia/poller v1.0.1 // indirect
	golang.org/x/sys v0.28.0 // indirect
	modernc.org/gc/v3 v3.0.0-20240107210532-573471604cb6 // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/strutil v1.2.0 // indirect
	modernc.org/token v1.1.0 // indirect
)

replace github.com/kballard/go-shellquote => github.com/micro-editor/go-shellquote v0.0.0-20250101105543-feb6c39314f5

replace layeh.com/gopher-luar v1.0.11 => github.com/layeh/gopher-luar v1.0.11

go 1.24.0
