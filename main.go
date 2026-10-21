// Command rfdtool checks, links, starts, and reports on a project's Requests
// for Discussion. A project describes its conventions in rfd.toml; see
// config.go.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/chaba-dev/rfdtool/internal/rfd"
)

func main() {
	cmd := &cli.Command{
		Name:  "rfdtool",
		Usage: "tools for a project's Requests for Discussion",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Usage:   "project configuration file, read when it exists",
				Value:   "rfd.toml",
				Sources: cli.EnvVars("RFD_CONFIG"),
			},
			&cli.StringFlag{
				Name:    "dir",
				Usage:   "RFD directory (config: dir)",
				Value:   "rfd",
				Sources: cli.EnvVars("RFD_DIR"),
			},
			&cli.StringSliceFlag{
				Name:    "private-projects",
				Usage:   "projects, an RFD's first label, whose RFDs are private by default and never public (config: private_projects)",
				Sources: cli.EnvVars("RFD_PRIVATE_PROJECTS"),
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "print each RFD's state and implementation progress",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := load(cmd)
					if err != nil {
						return err
					}
					report, err := rfd.Check(c.Dir, nil, c.PrivateProjects)
					if err != nil {
						return err
					}
					rfd.WriteStatus(os.Stdout, report.RFDs, palette(os.Stdout))
					return nil
				},
			},
			{
				Name:  "check",
				Usage: "validate RFDs, and the changes to them since a base revision",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "base-ref",
						Usage:   "git revision to check state transitions and final RFDs against",
						Sources: cli.EnvVars("RFD_BASE_REF"),
					},
					&cli.StringFlag{
						Name:    "base-dir",
						Usage:   "RFD directory to check state transitions and final RFDs against",
						Sources: cli.EnvVars("RFD_BASE_DIR"),
					},
				},
				Action: check,
			},
			{
				Name:  "link",
				Usage: "link every mention of another RFD to it",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := load(cmd)
					if err != nil {
						return err
					}
					changed, err := rfd.LinkAll(c.Dir)
					for _, path := range changed {
						fmt.Println(path)
					}
					return err
				},
			},
			{
				Name:      "new",
				Usage:     "start the next RFD from the template",
				ArgsUsage: "TITLE",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "labels",
						Usage: "comma-separated labels, starting with the project (config: labels)",
					},
					&cli.StringFlag{
						Name:  "authors",
						Usage: `authors as "Name <address>" (default: from git config)`,
					},
					&cli.StringFlag{
						Name:  "template",
						Usage: "template directory (config: template; default: the built-in template)",
					},
				},
				Action: create,
			},
		},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(ctx context.Context, cmd *cli.Command) error {
	c, err := load(cmd)
	if err != nil {
		return err
	}
	dir := c.Dir
	var base rfd.Tree
	switch baseRef, baseDir := cmd.String("base-ref"), cmd.String("base-dir"); {
	case baseRef != "" && baseDir != "":
		return fmt.Errorf("--base-ref and --base-dir are mutually exclusive")
	case baseDir != "":
		if info, err := os.Stat(baseDir); err != nil || !info.IsDir() {
			return fmt.Errorf("RFD base directory not found: %s", baseDir)
		}
		base = rfd.DirTree(baseDir)
	default:
		if base, err = rfd.NewGitTree(dir, baseRef); err != nil {
			return err
		}
	}

	report, err := rfd.Check(dir, base, c.PrivateProjects)
	if err != nil {
		return err
	}
	rfd.WriteIssues(os.Stderr, report.Issues, palette(os.Stderr))
	if len(report.Issues) > 0 {
		return cli.Exit("", 1)
	}
	return nil
}

func create(ctx context.Context, cmd *cli.Command) error {
	c, err := load(cmd)
	if err != nil {
		return err
	}
	title := strings.Join(cmd.Args().Slice(), " ")
	if c.Labels == "" {
		return fmt.Errorf("set --labels, or labels in the configuration file")
	}
	authors := cmd.String("authors")
	if authors == "" {
		name, email := gitConfig("user.name"), gitConfig("user.email")
		if name == "" || email == "" {
			return fmt.Errorf(`set --authors "Name <address>", or git's user.name and user.email`)
		}
		authors = name + " <" + email + ">"
	}
	templates := rfd.Template
	if c.Template != "" {
		templates = os.DirFS(c.Template)
	}
	entry, err := rfd.Create(c.Dir, templates, rfd.Draft{
		Title:   title,
		Authors: authors,
		Labels:  c.Labels,
	})
	if err != nil {
		return err
	}
	fmt.Println(filepath.Join(c.Dir, entry, "README.adoc"))
	return nil
}

func gitConfig(key string) string {
	out, err := exec.Command("git", "config", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// palette colors output to a terminal, following https://no-color.org and
// FORCE_COLOR.
func palette(f *os.File) rfd.Palette {
	if os.Getenv("NO_COLOR") != "" {
		return rfd.Palette{}
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return rfd.ANSI
	}
	if info, err := f.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return rfd.ANSI
	}
	return rfd.Palette{}
}
