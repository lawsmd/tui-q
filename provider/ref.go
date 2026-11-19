package provider

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	digits = regexp.MustCompile(`^#?([0-9]+)$`)
	// keyRef is a ticket key made of a project and a number, such as
	// ABC-123.
	keyRef = regexp.MustCompile(`(?i)\b[a-z][a-z0-9_]*-[0-9]+\b`)
	// numberRef is a number that fills a whole part of a link.
	numberRef = regexp.MustCompile(`(?:^|[/=#])([0-9]+)(?:$|[/?&#])`)
)

// afterHost drops the scheme and host of a link, which may hold text that
// looks like a ticket key. Text that is no link comes back whole.
func afterHost(s string) string {
	_, rest, ok := strings.Cut(s, "://")
	if !ok {
		return s
	}
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		return rest[i:]
	}
	return ""
}

// KeyRef reads what a user typed or pasted as the key of a ticket, on a
// platform whose keys are a project and a number. The text may be a key,
// a link to a ticket, or a bare number; a bare number is taken to be in
// project.
func KeyRef(input, project string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", fmt.Errorf("enter a ticket number, key or link")
	}
	if m := digits.FindStringSubmatch(s); m != nil {
		if project == "" {
			return "", fmt.Errorf("%s has no project: enter the full key, such as ABC-%s", s, m[1])
		}
		return strings.ToUpper(project) + "-" + m[1], nil
	}
	keys := keyRef.FindAllString(afterHost(s), -1)
	if len(keys) == 0 {
		return "", fmt.Errorf("no ticket key in %q", s)
	}
	return strings.ToUpper(keys[len(keys)-1]), nil
}

// NumberRef reads what a user typed or pasted as the ID of a ticket, on a
// platform whose IDs are numbers. The text may be a number or a link to a
// ticket, and the last number in a link is the one taken.
func NumberRef(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", fmt.Errorf("enter a ticket number or link")
	}
	if m := digits.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	rest := afterHost(s)
	var last string
	for rest != "" {
		loc := numberRef.FindStringSubmatchIndex(rest)
		if loc == nil {
			break
		}
		last = rest[loc[2]:loc[3]]
		// The mark that ends one number may start the next.
		rest = rest[loc[3]:]
	}
	if last == "" {
		return "", fmt.Errorf("no ticket number in %q", s)
	}
	return last, nil
}
