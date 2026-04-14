package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/koyashimano/docker-volume-manager/internal/commands"
	"github.com/koyashimano/docker-volume-manager/internal/config"
)

const version = "1.0.0"

var (
	// Global flags
	globalFlags    = flag.NewFlagSet("dvm", flag.ExitOnError)
	composePath    string
	projectName    string
	noCompose      bool
	verbose        bool
	quiet          bool
	configPath     string
	showVersion    bool
	showHelp       bool
)

func init() {
	globalFlags.StringVar(&composePath, "file", "", "Compose file path")
	globalFlags.StringVar(&composePath, "f", "", "Compose file path (shorthand)")
	globalFlags.StringVar(&projectName, "project", "", "Project name override")
	globalFlags.StringVar(&projectName, "p", "", "Project name override (shorthand)")
	globalFlags.BoolVar(&noCompose, "no-compose", false, "Disable Compose integration")
	globalFlags.BoolVar(&verbose, "verbose", false, "Verbose output")
	globalFlags.BoolVar(&verbose, "v", false, "Verbose output (shorthand)")
	globalFlags.BoolVar(&quiet, "quiet", false, "Minimal output")
	globalFlags.BoolVar(&quiet, "q", false, "Minimal output (shorthand)")
	globalFlags.StringVar(&configPath, "config", "", "Config file path")
	globalFlags.BoolVar(&showVersion, "version", false, "Show version")
	globalFlags.BoolVar(&showHelp, "help", false, "Show help")
	globalFlags.BoolVar(&showHelp, "h", false, "Show help (shorthand)")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	// Parse global flags
	globalFlags.Parse(os.Args[1:])

	if showVersion {
		fmt.Printf("dvm version %s\n", version)
		os.Exit(0)
	}

	if showHelp {
		printUsage()
		os.Exit(0)
	}

	// Get command
	args := globalFlags.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(0)
	}

	command := args[0]
	commandArgs := args[1:]

	// Load config
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.GetConfigPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Ensure directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directories: %v\n", err)
		os.Exit(1)
	}

	// Create context
	ctx, err := commands.NewContext(cfg, verbose, quiet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing: %v\n", err)
		os.Exit(1)
	}
	defer ctx.Close()

	// Load compose file unless --no-compose
	if !noCompose {
		if err := ctx.LoadCompose(composePath, projectName); err != nil {
			if command != "list" && command != "clean" && command != "history" {
				if verbose {
					fmt.Fprintf(os.Stderr, "Warning: Could not load compose file: %v\n", err)
				}
			}
		}
	}

	// Execute command
	exitCode := runCommand(ctx, command, commandArgs)
	os.Exit(int(exitCode))
}

func runCommand(ctx *commands.Context, command string, args []string) commands.ExitCode {
	var err error

	switch command {
	case "list", "ls":
		err = runList(ctx, args)
	case "backup":
		err = runBackup(ctx, args)
	case "restore":
		err = runRestore(ctx, args)
	case "archive":
		err = runArchive(ctx, args)
	case "swap":
		err = runSwap(ctx, args)
	case "clean":
		err = runClean(ctx, args)
	case "history":
		err = runHistory(ctx, args)
	case "inspect":
		err = runInspect(ctx, args)
	case "clone":
		err = runClone(ctx, args)
	case "browse":
		err = runBrowse(ctx, args)
	case "help":
		printUsage()
		return commands.ExitSuccess
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		return commands.ExitError
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return commands.GetExitCode(err)
	}

	return commands.ExitSuccess
}

func runList(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	all := fs.Bool("all", false, "Show all volumes")
	allShort := fs.Bool("a", false, "Show all volumes (shorthand)")
	unused := fs.Bool("unused", false, "Show only unused volumes")
	unusedShort := fs.Bool("u", false, "Show only unused volumes (shorthand)")
	stale := fs.Int("stale", 0, "Show volumes not accessed for N days")
	size := fs.Bool("size", false, "Show volume disk usage")
	sizeShort := fs.Bool("s", false, "Show volume disk usage (shorthand)")
	format := fs.String("format", "table", "Output format: table/json/csv")

	setSubcommandUsage(fs)
	fs.Parse(args)

	if len(fs.Args()) > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	opts := commands.ListOptions{
		All:    *all || *allShort,
		Unused: *unused || *unusedShort,
		Stale:  *stale,
		Size:   *size || *sizeShort,
		Format: *format,
	}

	return ctx.List(opts)
}

func runBackup(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	output := fs.String("output", "", "Output directory")
	outputShort := fs.String("o", "", "Output directory (shorthand)")
	format := fs.String("format", "", "Compression format: tar.gz/tar.zst")
	noCompress := fs.Bool("no-compress", false, "No compression")
	tag := fs.String("tag", "", "Tag for backup")
	tagShort := fs.String("t", "", "Tag for backup (shorthand)")
	stop := fs.Bool("stop", false, "Stop containers before backup")

	setSubcommandUsage(fs)
	fs.Parse(args)

	outDir := *output
	if outDir == "" {
		outDir = *outputShort
	}

	tagVal := *tag
	if tagVal == "" {
		tagVal = *tagShort
	}

	opts := commands.BackupOptions{
		Output:     outDir,
		Format:     *format,
		NoCompress: *noCompress,
		Tag:        tagVal,
		Stop:       *stop,
		Services:   fs.Args(),
	}

	return ctx.Backup(opts)
}

func runRestore(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	selectBackup := fs.Bool("select", false, "Select backup interactively")
	selectShort := fs.Bool("s", false, "Select backup interactively (shorthand)")
	list := fs.Bool("list", false, "List available backups")
	listShort := fs.Bool("l", false, "List available backups (shorthand)")
	force := fs.Bool("force", false, "Force without confirmation")
	restart := fs.Bool("restart", false, "Restart containers after restore")

	setSubcommandUsage(fs)
	fs.Parse(args)

	target := ""
	if len(fs.Args()) > 0 {
		target = fs.Args()[0]
	}
	backupFile := ""
	if len(fs.Args()) > 1 {
		backupFile = fs.Args()[1]
	}
	if len(fs.Args()) > 2 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args()[2:])
	}

	opts := commands.RestoreOptions{
		Select:     *selectBackup || *selectShort,
		List:       *list || *listShort,
		Force:      *force,
		Restart:    *restart,
		Target:     target,
		BackupFile: backupFile,
	}

	return ctx.Restore(opts)
}

func runArchive(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("archive", flag.ExitOnError)
	output := fs.String("output", "", "Archive directory")
	outputShort := fs.String("o", "", "Archive directory (shorthand)")
	verify := fs.Bool("verify", false, "Verify integrity before delete")
	force := fs.Bool("force", false, "Force without confirmation")

	setSubcommandUsage(fs)
	fs.Parse(args)

	outDir := *output
	if outDir == "" {
		outDir = *outputShort
	}

	opts := commands.ArchiveOptions{
		Output:   outDir,
		Verify:   *verify,
		Force:    *force,
		Services: fs.Args(),
	}

	return ctx.Archive(opts)
}

func runSwap(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("swap", flag.ExitOnError)
	empty := fs.Bool("empty", false, "Swap to empty volume")
	noBackup := fs.Bool("no-backup", false, "Don't backup current volume")
	restart := fs.Bool("restart", false, "Restart containers after swap")

	setSubcommandUsage(fs)
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		return fmt.Errorf("service name required")
	}

	service := fs.Args()[0]
	source := ""
	if len(fs.Args()) > 1 {
		source = fs.Args()[1]
	}
	if len(fs.Args()) > 2 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args()[2:])
	}

	opts := commands.SwapOptions{
		Empty:    *empty,
		NoBackup: *noBackup,
		Restart:  *restart,
		Service:  service,
		Source:   source,
	}

	return ctx.Swap(opts)
}

func runClean(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	unused := fs.Bool("unused", false, "Clean unused volumes")
	unusedShort := fs.Bool("u", false, "Clean unused volumes (shorthand)")
	stale := fs.Int("stale", 0, "Clean volumes not accessed for N days")
	dryRun := fs.Bool("dry-run", false, "Show what would be cleaned")
	dryRunShort := fs.Bool("n", false, "Show what would be cleaned (shorthand)")
	archive := fs.Bool("archive", false, "Archive before cleaning")
	archiveShort := fs.Bool("a", false, "Archive before cleaning (shorthand)")
	force := fs.Bool("force", false, "Force without confirmation")

	setSubcommandUsage(fs)
	fs.Parse(args)

	if len(fs.Args()) > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	opts := commands.CleanOptions{
		Unused:  *unused || *unusedShort,
		Stale:   *stale,
		DryRun:  *dryRun || *dryRunShort,
		Archive: *archive || *archiveShort,
		Force:   *force,
	}

	return ctx.Clean(opts)
}

func runHistory(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	limit := fs.Int("limit", 10, "Number of records to show")
	limitShort := fs.Int("n", 10, "Number of records to show (shorthand)")
	all := fs.Bool("all", false, "Show all projects")
	allShort := fs.Bool("a", false, "Show all projects (shorthand)")

	setSubcommandUsage(fs)
	fs.Parse(args)

	service := ""
	if len(fs.Args()) > 0 {
		service = fs.Args()[0]
	}
	if len(fs.Args()) > 1 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args()[1:])
	}

	// Determine which limit flag was actually set
	var (
		limitSet      bool
		limitShortSet bool
	)
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "limit":
			limitSet = true
		case "n":
			limitShortSet = true
		}
	})

	lim := *limit
	if limitShortSet {
		lim = *limitShort
	} else if limitSet {
		lim = *limit
	}

	opts := commands.HistoryOptions{
		Limit:   lim,
		All:     *all || *allShort,
		Service: service,
	}

	return ctx.History(opts)
}

func runInspect(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	files := fs.Bool("files", false, "Show files in volume")
	top := fs.Int("top", 0, "Show top N largest files")
	format := fs.String("format", "table", "Output format: table/json/yaml")

	setSubcommandUsage(fs)
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		return fmt.Errorf("service name required")
	}
	if len(fs.Args()) > 1 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args()[1:])
	}

	opts := commands.InspectOptions{
		Files:   *files,
		Top:     *top,
		Format:  *format,
		Service: fs.Args()[0],
	}

	return ctx.Inspect(opts)
}

func runBrowse(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("browse", flag.ExitOnError)
	all := fs.Bool("all", false, "Show hidden files")
	allShort := fs.Bool("a", false, "Show hidden files (shorthand)")
	long := fs.Bool("long", false, "Show detailed listing")
	longShort := fs.Bool("l", false, "Show detailed listing (shorthand)")
	recursive := fs.Bool("recursive", false, "List recursively")
	recursiveShort := fs.Bool("R", false, "List recursively (shorthand)")

	setSubcommandUsage(fs)
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		return fmt.Errorf("service name required")
	}

	service := fs.Args()[0]
	path := ""
	if len(fs.Args()) > 1 {
		path = fs.Args()[1]
	}
	if len(fs.Args()) > 2 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args()[2:])
	}

	opts := commands.BrowseOptions{
		All:       *all || *allShort,
		Long:      *long || *longShort,
		Recursive: *recursive || *recursiveShort,
		Path:      path,
		Service:   service,
	}

	return ctx.Browse(opts)
}

func runClone(ctx *commands.Context, args []string) error {
	fs := flag.NewFlagSet("clone", flag.ExitOnError)
	fs.Parse(args)

	if len(fs.Args()) < 2 {
		return fmt.Errorf("usage: dvm clone <service> <new-name>")
	}
	if len(fs.Args()) > 2 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args()[2:])
	}

	opts := commands.CloneOptions{
		Service: fs.Args()[0],
		NewName: fs.Args()[1],
	}

	return ctx.Clone(opts)
}

// setSubcommandUsage sets a custom usage function that displays flags with -- prefix
func setSubcommandUsage(fs *flag.FlagSet) {
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", fs.Name())

		// Map from base usage text to short flag name
		shortFlags := make(map[string]string)
		fs.VisitAll(func(f *flag.Flag) {
			if len(f.Name) == 1 {
				baseUsage := strings.TrimSuffix(f.Usage, " (shorthand)")
				shortFlags[baseUsage] = f.Name
			}
		})

		// Print long flags paired with their short flags
		fs.VisitAll(func(f *flag.Flag) {
			if len(f.Name) == 1 {
				return
			}

			short := shortFlags[f.Usage]

			var line string
			if short != "" {
				line = fmt.Sprintf("  -%s, --%s", short, f.Name)
			} else {
				line = fmt.Sprintf("      --%s", f.Name)
			}

			// Add type hint for non-bool flags
			typeName := flagTypeName(f)
			if typeName != "" {
				line += " " + typeName
			}

			// Pad to align descriptions
			if len(line) < 30 {
				line += strings.Repeat(" ", 30-len(line))
			} else {
				line += "  "
			}

			line += f.Usage

			// Add default value if non-zero
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
				if _, err := strconv.Atoi(f.DefValue); err == nil {
					line += fmt.Sprintf(" (default %s)", f.DefValue)
				} else {
					line += fmt.Sprintf(" (default %q)", f.DefValue)
				}
			}

			fmt.Fprintln(os.Stderr, line)
		})
	}
}

func flagTypeName(f *flag.Flag) string {
	if f.DefValue == "true" || f.DefValue == "false" {
		return ""
	}
	if _, err := strconv.Atoi(f.DefValue); err == nil {
		return "int"
	}
	return "string"
}

func printUsage() {
	fmt.Println(`dvm - Docker Volume Manager

Usage:
  dvm [global-options] <command> [command-options] [arguments]

Global Options:
  -f, --file <path>      Compose file path
  -p, --project <name>   Project name override
  --no-compose           Disable Compose integration
  -v, --verbose          Verbose output
  -q, --quiet            Minimal output
  --config <path>        Config file path
  --version              Show version
  -h, --help             Show help

Commands:
  list        List volumes
  backup      Backup volumes
  restore     Restore volumes from backup
  archive     Archive and delete volumes
  swap        Swap volume with another
  clean       Clean up unused volumes
  history     Show backup history
  inspect     Show detailed volume information
  clone       Clone a volume
  browse      Browse volume contents
  help        Show help

Examples:
  dvm list
  dvm backup db
  dvm restore db --select
  dvm swap db --empty --restart
  dvm clean --unused --dry-run

For more information: https://github.com/koyashimano/docker-volume-manager`)
}
