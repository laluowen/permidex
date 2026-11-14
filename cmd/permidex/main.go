// Command permidex browses cloud IAM roles and permissions offline, from a local cache.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/laluowen/permidex/v2/internal/browse"
	"github.com/laluowen/permidex/v2/internal/cache"
	"github.com/laluowen/permidex/v2/internal/gcp"
)

const usage = `usage: permidex <provider> [--refresh|-r]

providers:
  gcp             browse GCP predefined IAM roles and permissions

options:
  --refresh|-r    refresh that provider's cache from the cloud instead of browsing
  --cache-dir|-c  print the cache directory and exit

Run "permidex <provider> --refresh|-r" once before browsing.`

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "permidex:", err)
		os.Exit(1)
	}
}

// providerFirst moves the first non-flag argument, the provider, to the front, so that both
// `permidex gcp --refresh|-r` and `permidex --refresh|-r gcp` work.
func providerFirst(args []string) []string {
	for i, a := range args {
		if !strings.HasPrefix(a, "-") {
			out := append([]string{a}, args[:i]...)
			return append(out, args[i+1:]...)
		}
	}
	return args
}

// wantsCacheDir reports whether the arguments ask for the cache directory, with --cache-dir or -c.
func wantsCacheDir(args []string) bool {
	return slices.ContainsFunc(args, func(a string) bool { return a == "-c" || a == "--cache-dir" })
}

func run(ctx context.Context, args []string) error {
	if wantsCacheDir(args) {
		dir, err := cache.Dir()
		if err != nil {
			return err
		}
		fmt.Println(dir)
		return nil
	}
	args = providerFirst(args)
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return errors.New("no provider given")
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Println(usage)
		return nil
	case "gcp":
		return gcpCmd(ctx, args[1:])
	default:
		return fmt.Errorf("unknown provider %q (try `permidex help`)", args[0])
	}
}

func gcpCmd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("permidex gcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var refresh bool
	fs.BoolVar(&refresh, "r", false, "refresh the cache from GCP")
	fs.BoolVar(&refresh, "refresh", false, "refresh the cache from GCP")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Println(usage)
			return nil
		}
		return fmt.Errorf("%w (see `permidex help`)", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q (see `permidex help`)", fs.Arg(0))
	}
	path, err := gcp.CachePath()
	if err != nil {
		return err
	}
	if refresh {
		n, err := gcp.Refresh(ctx, path, os.Stderr)
		if err != nil {
			return err
		}
		fmt.Printf("wrote %d roles to %s\n", n, path)
		return nil
	}
	snap, err := gcp.Load(path)
	if errors.Is(err, cache.ErrMissing) {
		return fmt.Errorf("no cache at %s: run `permidex gcp --refresh|-r`", path)
	}
	if err != nil {
		return err
	}
	return browseSource(gcp.NewSource(snap))
}

func browseSource(src browse.Source) error {
	final, err := tea.NewProgram(browse.New(src), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return err
	}
	if m, ok := final.(*browse.Model); ok {
		fmt.Print(m.Selection())
	}
	return nil
}
