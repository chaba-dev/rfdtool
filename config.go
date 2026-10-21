package main

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v3"
)

// A config holds a project's RFD conventions. A project sets them in
// rfd.toml, next to where rfdtool runs:
//
//	# Directory of the RFDs, one NNNN directory each.
//	dir = "rfd"
//	# Template directory that "rfdtool new" starts an RFD from, with
//	# README.adoc and IMPLEMENTATION.adoc. Without it, the built-in one.
//	template = "template"
//	# Labels "rfdtool new" gives an RFD, starting with its project.
//	labels = "core"
//	# Projects whose RFDs are private by default and never public.
//	private_projects = ["internal"]
//
// A flag or environment variable overrides the file. Paths in the file are
// relative to it.
type config struct {
	Dir             string   `toml:"dir"`
	Template        string   `toml:"template"`
	Labels          string   `toml:"labels"`
	PrivateProjects []string `toml:"private_projects"`
}

// load reads the configuration file, then applies the flags and
// environment variables that are set.
func load(cmd *cli.Command) (config, error) {
	var file config
	path := cmd.String("config")
	meta, err := toml.DecodeFile(path, &file)
	switch {
	case errors.Is(err, fs.ErrNotExist) && !cmd.IsSet("config"):
	case err != nil:
		return config{}, fmt.Errorf("read %s: %w", path, err)
	case len(meta.Undecoded()) > 0:
		return config{}, fmt.Errorf("%s: unknown setting %s", path, meta.Undecoded()[0])
	}
	relative := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(filepath.Dir(path), p)
	}

	c := config{
		Dir:             setting(cmd, "dir", relative(file.Dir)),
		Template:        setting(cmd, "template", relative(file.Template)),
		Labels:          setting(cmd, "labels", file.Labels),
		PrivateProjects: file.PrivateProjects,
	}
	if cmd.IsSet("private-projects") {
		c.PrivateProjects = nil
		for _, p := range cmd.StringSlice("private-projects") {
			c.PrivateProjects = append(c.PrivateProjects, strings.TrimSpace(p))
		}
	}
	return c, nil
}

// setting returns a string flag's value when it is set on the command line
// or in the environment, otherwise the configured value, otherwise the flag's
// default. Flags that the command does not have read as empty.
func setting(cmd *cli.Command, name, configured string) string {
	if !cmd.IsSet(name) && configured != "" {
		return configured
	}
	return cmd.String(name)
}
