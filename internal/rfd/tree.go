package rfd

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// A Tree is a read-only view of an RFD directory: the working copy, or the
// version of it that a change is checked against.
type Tree interface {
	// ReadDir lists the names in a directory, or "." for the RFD root.
	ReadDir(name string) ([]string, error)
	// ReadFile reads a slash-separated path relative to the RFD root. A
	// missing file returns an error that wraps fs.ErrNotExist.
	ReadFile(name string) ([]byte, error)
}

// DirTree reads RFDs from a directory on disk.
type DirTree string

func (d DirTree) ReadDir(name string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(string(d), filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names, nil
}

func (d DirTree) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(string(d), filepath.FromSlash(name)))
}

// GitTree reads RFDs from a commit, at the path of an RFD directory inside a
// git work tree.
type GitTree struct {
	dir string
	rev string
}

// GitHub sends an all-zero SHA as the previous revision of a new branch.
var zeroRev = regexp.MustCompile(`^0+$`)

// NewGitTree returns the RFD directory dir as of rev. It returns nil when rev
// is empty or all zeros, meaning there is no base revision.
func NewGitTree(dir, rev string) (Tree, error) {
	if rev == "" || zeroRev.MatchString(rev) {
		return nil, nil
	}
	if err := exec.Command("git", "-C", dir, "cat-file", "-e", rev+"^{commit}").Run(); err != nil {
		return nil, fmt.Errorf("cannot resolve RFD base revision: %s", rev)
	}
	return &GitTree{dir: dir, rev: rev}, nil
}

func (g *GitTree) object(name string) string {
	// "./" makes the path relative to dir rather than the repository root.
	if name = path.Clean(name); name == "." {
		name = ""
	}
	return g.rev + ":./" + name
}

func (g *GitTree) git(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", g.dir}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g *GitTree) exists(object, kind string) bool {
	out, err := g.git("cat-file", "-t", object)
	return err == nil && strings.TrimSpace(string(out)) == kind
}

func (g *GitTree) ReadDir(name string) ([]string, error) {
	object := g.object(name)
	if !g.exists(object, "tree") {
		return nil, fmt.Errorf("%s: %w", object, fs.ErrNotExist)
	}
	// Without --full-tree, ls-tree also filters by the path of dir.
	out, err := g.git("ls-tree", "--full-tree", "--name-only", "-z", object)
	if err != nil {
		return nil, err
	}
	return strings.FieldsFunc(string(out), func(r rune) bool { return r == 0 }), nil
}

func (g *GitTree) ReadFile(name string) ([]byte, error) {
	object := g.object(name)
	if !g.exists(object, "blob") {
		return nil, fmt.Errorf("%s: %w", object, fs.ErrNotExist)
	}
	return g.git("cat-file", "blob", object)
}

func notExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
