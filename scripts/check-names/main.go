// Command check-names fails when text bound for this repository matches a
// pattern on a private denylist. The denylist holds the names this
// repository must never contain; it lives outside the repository so that
// the list itself cannot leak them.
//
// Usage:
//
//	go run ./scripts/check-names              staged files (pre-commit)
//	go run ./scripts/check-names -message F   a commit message file (commit-msg)
//	go run ./scripts/check-names -history     every commit on every ref (pre-push)
//
// The denylist is the file named by TQ_DENYLIST, or
// ~/.config/_local/tui-q-denylist.txt. Each line is a regular expression
// matched without regard to case. Blank lines and lines starting with # are
// skipped.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	message := flag.String("message", "", "check this commit message file")
	history := flag.Bool("history", false, "check every commit on every ref")
	flag.Parse()

	patterns, path, err := loadDenylist()
	if err != nil {
		fmt.Fprintln(os.Stderr, "check-names:", err)
		os.Exit(2)
	}

	var hits []string
	switch {
	case *message != "":
		data, err := os.ReadFile(*message)
		if err != nil {
			fmt.Fprintln(os.Stderr, "check-names:", err)
			os.Exit(2)
		}
		hits = scan("commit message", data, patterns)
	case *history:
		// Messages, author and committer identities, and every diff.
		out, err := git("log", "--all", "-p", "--format=commit %H%n%an <%ae>%n%cn <%ce>%n%B")
		if err != nil {
			fmt.Fprintln(os.Stderr, "check-names:", err)
			os.Exit(2)
		}
		hits = scan("history", out, patterns)
	default:
		out, err := git("diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z")
		if err != nil {
			fmt.Fprintln(os.Stderr, "check-names:", err)
			os.Exit(2)
		}
		for _, name := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
			if name == "" {
				continue
			}
			hits = append(hits, scan("path", []byte(name), patterns)...)
			blob, err := git("show", ":"+name)
			if err != nil {
				fmt.Fprintln(os.Stderr, "check-names:", err)
				os.Exit(2)
			}
			hits = append(hits, scan(name, blob, patterns)...)
		}
	}

	if len(hits) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "check-names: text matches the denylist (%s):\n", path)
	for _, h := range hits {
		fmt.Fprintln(os.Stderr, "  "+h)
	}
	fmt.Fprintln(os.Stderr, "This repository must not name an employer, site, project key or field ID.")
	fmt.Fprintln(os.Stderr, "Move the specifics into the instance module and try again.")
	os.Exit(1)
}

func loadDenylist() ([]*regexp.Regexp, string, error) {
	path := os.Getenv("TQ_DENYLIST")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", err
		}
		path = filepath.Join(home, ".config", "_local", "tui-q-denylist.txt")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, path, fmt.Errorf("no denylist at %s: create it with one regular expression per line, "+
			"or point TQ_DENYLIST at one. Commits are refused without it", path)
	}
	defer f.Close()

	var patterns []*regexp.Regexp
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		re, err := regexp.Compile("(?i)" + line)
		if err != nil {
			return nil, path, fmt.Errorf("%s line %d: %w", path, n, err)
		}
		patterns = append(patterns, re)
	}
	if err := sc.Err(); err != nil {
		return nil, path, err
	}
	if len(patterns) == 0 {
		return nil, path, fmt.Errorf("%s holds no patterns", path)
	}
	return patterns, path, nil
}

// scan returns one line per match: where it was found and the text that
// matched.
func scan(where string, data []byte, patterns []*regexp.Regexp) []string {
	var hits []string
	for n, line := range bytes.Split(data, []byte("\n")) {
		for _, re := range patterns {
			if m := re.Find(line); m != nil {
				hits = append(hits, fmt.Sprintf("%s:%d: %q", where, n+1, m))
			}
		}
	}
	return hits
}

func git(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
