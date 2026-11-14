// Command permidex browses cloud IAM roles, permissions and services offline, from a local cache.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/laluowen/permidex/v2/internal/aws"
	"github.com/laluowen/permidex/v2/internal/browse"
	"github.com/laluowen/permidex/v2/internal/cache"
	"github.com/laluowen/permidex/v2/internal/gcp"
	"github.com/spf13/cobra"
)

// provider describes one catalogue: its command text, where its cache lives, how to refresh it, and how to
// open it.
type provider struct {
	name    string
	short   string
	long    string
	source  string
	path    func() (string, error)
	refresh func(ctx context.Context, path string, progress io.Writer) (string, error)
	open    func(path string) (browse.Source, error)
}

var providers = []provider{
	{
		name:   "gcp",
		short:  "browse GCP predefined IAM roles and permissions",
		long:   "Browse GCP predefined IAM roles and permissions from the local cache.\n\nBuild the cache first with --refresh. That uses Application Default Credentials: run `gcloud auth application-default login` once.",
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
	{
		name:   "aws",
		short:  "browse AWS actions, managed policies and services",
		long:   "Browse AWS actions, AWS-managed policies and services from the local cache.\n\nBuild the cache first with --refresh. That uses the standard AWS credential chain and reads IAM (ListPolicies, GetPolicy, GetPolicyVersion). The service reference it also reads is public.",
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
	err := newRootCmd().ExecuteContext(ctx)
	switch {
	case errors.Is(err, context.Canceled):
		_, _ = fmt.Fprintln(os.Stderr, "permidex: interrupted; the existing cache is unchanged")
		stop()
		os.Exit(130)
	case err != nil:
		_, _ = fmt.Fprintln(os.Stderr, "permidex:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var cacheDir bool
	root := &cobra.Command{
		Use:   "permidex",
		Short: "browse cloud IAM roles, permissions and services offline",
		Long: `Browse cloud IAM roles, permissions and services from a local cache.

Each provider has its own cache, built with --refresh. Browsing never fetches on its own.

The cache directory is $PERMIDEX_CACHE_DIR if set (it must be an absolute path), otherwise
$XDG_CACHE_HOME/permidex if XDG_CACHE_HOME is an absolute path, otherwise ~/.cache/permidex.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cacheDir {
				dir, err := cacheDirectory(cmd)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), dir)
				return nil
			}
			return cmd.Help()
		},
	}
	root.Flags().BoolVarP(&cacheDir, "cache-dir", "c", false, "print the cache directory and exit")
	for _, p := range providers {
		root.AddCommand(providerCommand(p))
	}
	return root
}

func providerCommand(p provider) *cobra.Command {
	var refresh, cacheDir bool
	cmd := &cobra.Command{
		Use:           p.name,
		Short:         p.short,
		Long:          p.long,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cacheDir {
				dir, err := cacheDirectory(cmd)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(dir, p.name))
				return nil
			}
			path, err := providerPath(cmd, p)
			if err != nil {
				return err
			}
			if refresh {
				summary, err := p.refresh(cmd.Context(), path, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), summary)
				return nil
			}
			src, err := p.open(path)
			if errors.Is(err, cache.ErrMissing) {
				return fmt.Errorf("no cache at %s: run `permidex %s --refresh`", path, p.name)
			}
			if err != nil {
				return err
			}
			return browseSource(cmd, src)
		},
	}
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "refresh the cache from "+p.source+" instead of browsing")
	cmd.Flags().BoolVarP(&cacheDir, "cache-dir", "c", false, "print the directory holding this provider's cache and exit")
	return cmd
}

// cacheDirectory resolves the base cache directory. An ignored XDG_CACHE_HOME is reported as a warning on
// stderr. An invalid PERMIDEX_CACHE_DIR is an error.
func cacheDirectory(cmd *cobra.Command) (string, error) {
	dir, warning, err := cache.Resolve()
	if err != nil {
		return "", err
	}
	if warning != "" {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "permidex: warning:", warning)
	}
	return dir, nil
}

// providerPath resolves the cache file for p, with the same warnings and errors as cacheDirectory.
func providerPath(cmd *cobra.Command, p provider) (string, error) {
	if _, err := cacheDirectory(cmd); err != nil {
		return "", err
	}
	return p.path()
}

func browseSource(cmd *cobra.Command, src browse.Source) error {
	final, err := tea.NewProgram(browse.New(src), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return err
	}
	if m, ok := final.(*browse.Model); ok {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), m.Selection())
	}
	return nil
}
