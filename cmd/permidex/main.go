// Command permidex browses GCP predefined IAM roles and permissions.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/laluowen/permidex/internal/cache"
	"github.com/laluowen/permidex/internal/iam"
	"github.com/laluowen/permidex/internal/index"
	"github.com/laluowen/permidex/internal/tui"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "permidex:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return browse()
	}
	switch args[0] {
	case "refresh":
		return refresh(ctx)
	case "help", "-h", "--help":
		fmt.Println("usage: permidex [refresh]\n\n  (no args)  browse the cached roles\n  refresh    fetch roles from GCP into the cache")
		return nil
	default:
		return fmt.Errorf("unknown command %q (try `permidex help`)", args[0])
	}
}

func refresh(ctx context.Context) error {
	path, err := cache.Path()
	if err != nil {
		return err
	}
	hc, err := iam.NewADCClient(ctx)
	if err != nil {
		return err
	}
	roles, err := iam.Client{HTTP: hc}.Roles(ctx)
	if err != nil {
		return err
	}
	snap := cache.Snapshot{FetchedAt: time.Now().UTC(), Roles: roles}
	if err := cache.Write(path, snap); err != nil {
		return err
	}
	fmt.Printf("wrote %d roles to %s\n", len(roles), path)
	return nil
}

func browse() error {
	path, err := cache.Path()
	if err != nil {
		return err
	}
	snap, err := cache.Read(path)
	if errors.Is(err, cache.ErrMissing) {
		return fmt.Errorf("no cache at %s: run `permidex refresh`", path)
	}
	if err != nil {
		return err
	}
	m := tui.New(index.Build(snap.Roles), snap.FetchedAt)
	final, err := tea.NewProgram(m, tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return err
	}
	if fm, ok := final.(*tui.Model); ok {
		fmt.Print(fm.Selection())
	}
	return nil
}
