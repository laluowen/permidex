// Command permidex browses cloud IAM roles, permissions and services offline, from a local cache.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/laluowen/permidex/v2/internal/aws"
	"github.com/laluowen/permidex/v2/internal/browse"
	"github.com/laluowen/permidex/v2/internal/cache"
	"github.com/laluowen/permidex/v2/internal/gcp"
)

const usage = `usage: permidex <provider> [--refresh|-r] | permidex --cache-dir|-c

providers:
  gcp             browse GCP predefined IAM roles and permissions
  aws             browse AWS actions, managed policies and services

options:
  --refresh|-r    refresh that provider's cache from the cloud instead of browsing
  --cache-dir|-c  print the cache directory and exit

Run "permidex <provider> --refresh|-r" once before browsing.`

// provider is one catalogue the CLI can browse or refresh.
type provider struct {
	name    string
	source  string
	path    func() (string, error)
	refresh func(ctx context.Context, path string, progress io.Writer) (string, error)
	open    func(path string) (browse.Source, error)
}

var providers = map[string]provider{
	"gcp": {
		name:   "gcp",
		source: "GCP",
		path:   gcp.CachePath,
		refresh: func(ctx context.Context, path string, progress io.Writer) (string, error) {
			n, err := gcp.Refresh(ctx, path, progress)
			return fmt.Sprintf("wrote %d roles to %s", n, path), err
		},
		open: func(path string) (browse.Source, error) {
			snap, err := gcp.Load(path)
			if err != nil {
				return nil, err
			}
			return gcp.NewSource(snap), nil
		},
	},
	"aws": {
		name:   "aws",
		source: "AWS",
		path:   aws.CachePath,
		refresh: func(ctx context.Context, path string, progress io.Writer) (string, error) {
			services, policies, err := aws.Refresh(ctx, path, progress)
			return fmt.Sprintf("wrote %d services and %d policies to %s", services, policies, path), err
		},
		open: func(path string) (browse.Source, error) {
			snap, err := aws.Load(path)
			if err != nil {
				return nil, err
			}
			return aws.NewSource(snap), nil
		},
	},
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := run(ctx, os.Args[1:])
	switch {
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(os.Stderr, "permidex: interrupted; the existing cache is unchanged")
		stop()
		os.Exit(130)
	case err != nil:
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

// cacheDirectory resolves the cache directory. An ignored XDG_CACHE_HOME is reported as a warning on stderr.
func cacheDirectory() (string, error) {
	dir, warning, err := cache.Resolve()
	if err != nil {
		return "", err
	}
	if warning != "" {
		fmt.Fprintln(os.Stderr, "permidex: warning:", warning)
	}
	return dir, nil
}

func run(ctx context.Context, args []string) error {
	if wantsCacheDir(args) {
		dir, err := cacheDirectory()
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
	default:
		p, ok := providers[args[0]]
		if !ok {
			return fmt.Errorf("unknown provider %q (try `permidex help`)", args[0])
		}
		return providerCmd(ctx, p, args[1:])
	}
}

func providerCmd(ctx context.Context, p provider, args []string) error {
	fs := flag.NewFlagSet("permidex "+p.name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var refresh bool
	fs.BoolVar(&refresh, "r", false, "refresh the cache from "+p.source)
	fs.BoolVar(&refresh, "refresh", false, "refresh the cache from "+p.source)
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
	if _, err := cacheDirectory(); err != nil {
		return err
	}
	path, err := p.path()
	if err != nil {
		return err
	}
	if refresh {
		summary, err := p.refresh(ctx, path, os.Stderr)
		if err != nil {
			return err
		}
		fmt.Println(summary)
		return nil
	}
	src, err := p.open(path)
	if errors.Is(err, cache.ErrMissing) {
		return fmt.Errorf("no cache at %s: run `permidex %s --refresh|-r`", path, p.name)
	}
	if err != nil {
		return err
	}
	return browseSource(src)
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
