package chezmoi

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Client wraps the chezmoi CLI. All operations shell out to the chezmoi binary
// so that every command is observable in the TUI console log, exactly like
// lazygit shows git commands.
type Client struct {
	cmdLog     func(string)                        // nil = silent; wired by GUI after construction
	runnerFunc func(args ...string) ([]byte, error) // nil = real exec; non-nil = test injection
}

// NewClient creates a Client. It does not verify chezmoi is on PATH — that is
// the caller's (app.go's) responsibility.
func NewClient() *Client {
	return &Client{}
}

// SetCmdLog wires a logging callback. Should be called by the GUI layer after
// the console view exists and before the event loop starts.
func (c *Client) SetCmdLog(fn func(string)) { c.cmdLog = fn }

// log emits msg to the console if a logger is wired.
func (c *Client) log(msg string) {
	if c.cmdLog != nil {
		c.cmdLog(msg)
	}
}

// run logs then executes a chezmoi subcommand, returning stdout.
// stderr is captured and included in the error on non-zero exit.
func (c *Client) run(args ...string) ([]byte, error) {
	c.log("$ chezmoi " + strings.Join(args, " "))
	return c.exec(args...)
}

// runQuiet executes a chezmoi subcommand without logging to the console.
// Used for read-only preview commands (diff, cat) that fire automatically on
// every navigation keystroke and would flood the console log.
func (c *Client) runQuiet(args ...string) ([]byte, error) {
	return c.exec(args...)
}

// expandPath prefixes a home-relative path with "~/" so chezmoi can locate it
// regardless of the current working directory. Paths already starting with "/"
// or "~" are returned unchanged. chezmoi status and chezmoi managed output
// paths relative to the home directory (e.g. ".bashrc", ".config/nvim/..."),
// which chezmoi itself interprets as CWD-relative if not prefixed.
func expandPath(p string) string {
	if p == "" || p[0] == '/' || p[0] == '~' {
		return p
	}
	return "~/" + p
}

// exec is the shared execution backend for run and runQuiet.
func (c *Client) exec(args ...string) ([]byte, error) {
	if c.runnerFunc != nil {
		return c.runnerFunc(args...)
	}
	out, err := exec.Command("chezmoi", args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("chezmoi %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("chezmoi %s: %w", args[0], err)
	}
	return out, nil
}

// Status returns files that differ between the chezmoi source state and the
// target state, as reported by `chezmoi status`.
//
// Output format per line: "XY path" where XY is a two-character status code
// and path is the target path relative to the home directory.
func (c *Client) Status() ([]*StatusFile, error) {
	out, err := c.run("status")
	if err != nil {
		return nil, err
	}
	var files []*StatusFile
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		path := strings.TrimSpace(line[2:])
		if path == "" {
			continue
		}
		files = append(files, &StatusFile{Path: path, Status: status})
	}
	return files, scanner.Err()
}

// Managed returns all regular files tracked by chezmoi.
func (c *Client) Managed() ([]*ManagedFile, error) {
	out, err := c.run("managed", "--include=files")
	if err != nil {
		return nil, err
	}
	var files []*ManagedFile
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, NewManagedFile(line))
		}
	}
	return files, scanner.Err()
}

// Scripts returns all scripts tracked by chezmoi.
// Returns nil (no error) when chezmoi exits non-zero due to no scripts existing.
func (c *Client) Scripts() ([]*Script, error) {
	out, err := c.run("managed", "--include=scripts")
	if err != nil {
		// chezmoi may exit non-zero when there are no scripts — treat as empty.
		return nil, nil
	}
	var scripts []*Script
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			scripts = append(scripts, NewScript(line))
		}
	}
	return scripts, scanner.Err()
}

// Diff returns the unified diff between the chezmoi source and the target for
// a specific path. An empty diff means the file is up to date.
// Uses runQuiet — fired automatically on navigation, not logged to the console.
func (c *Client) Diff(target string) (string, error) {
	out, err := c.runQuiet("diff", expandPath(target))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Cat returns the rendered source content of a managed target path.
// Uses runQuiet — fired automatically on navigation, not logged to the console.
func (c *Client) Cat(target string) (string, error) {
	out, err := c.runQuiet("cat", expandPath(target))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Apply applies a specific target path. If target is empty, applies all managed
// files (equivalent to `chezmoi apply` with no arguments).
func (c *Client) Apply(target string) error {
	args := []string{"apply"}
	if target != "" {
		args = append(args, expandPath(target))
	}
	_, err := c.run(args...)
	return err
}

// Add adds a target path to chezmoi management (`chezmoi add <target>`).
func (c *Client) Add(target string) error {
	_, err := c.run("add", expandPath(target))
	return err
}

// ReAdd updates the chezmoi source from the target path, pulling any changes
// made directly to the target back into the source (`chezmoi re-add <target>`).
func (c *Client) ReAdd(target string) error {
	_, err := c.run("re-add", expandPath(target))
	return err
}

// Forget removes a target from chezmoi management without deleting the target
// file (`chezmoi forget --force <target>`). --force skips interactive prompts
// since the user has already confirmed intent by pressing the keybind.
func (c *Client) Forget(target string) error {
	_, err := c.run("forget", "--force", expandPath(target))
	return err
}
