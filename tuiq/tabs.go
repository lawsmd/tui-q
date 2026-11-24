package tuiq

import (
	"fmt"
	"io"

	"github.com/lawsmd/tui-q/internal/session"
)

// tabsNotice opens the output of "tq tabs". The reader is an LLM agent
// that has its own access to the ticketing platform and can run slate's
// commands, and the notice tells it what the list is for.
const tabsNotice = `Each block is one running tq, named by its process ID, with the tickets open in its
tabs. A * marks the ticket on screen. Under a ticket are the tickets it is linked to.
The comment box of the ticket on screen is the editor that "slate ls" lists with the
same process ID and ticket: read and change its draft with "slate read" and "slate edit".
This list gives keys, statuses and titles only. Read ticket details through your own
access to the ticketing platform, not through tq.
`

// tabs prints the tickets open in the tabs of every interface running on
// the site whose session files are in dir. A ticket is printed as its key,
// status and title, and each ticket it is linked to as the relation, the
// key and the status, and nothing more. It reads the files those
// interfaces keep and asks the platform nothing. It is run as "tq tabs" by
// an agent that has been asked about the tickets the user has open.
func tabs(w io.Writer, dir string, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("tabs takes no argument %q", args[0])
	}
	states, err := session.List(dir)
	if err != nil {
		return err
	}
	fmt.Fprint(w, "Tickets open in tq\n", tabsNotice)
	if len(states) == 0 {
		fmt.Fprint(w, "\nNo tq is running.\n")
		return nil
	}
	for _, s := range states {
		fmt.Fprintf(w, "\ntq %d\n", s.PID)
		if len(s.Tabs) == 0 {
			fmt.Fprint(w, "  no ticket is open\n")
			continue
		}
		keyW, numW := 0, len(fmt.Sprint(len(s.Tabs)))
		for _, t := range s.Tabs {
			keyW = max(keyW, len(t.Key))
		}
		for i, t := range s.Tabs {
			mark := " "
			if t.Shown {
				mark = "*"
			}
			fmt.Fprintf(w, "%s %*d. %-*s  [%s]  %s\n", mark, numW, i+1, keyW, t.Key, t.Status, t.Title)
			relW, linkW := 0, 0
			for _, l := range t.Links {
				relW, linkW = max(relW, len(l.Relation)), max(linkW, len(l.Key))
			}
			for _, l := range t.Links {
				fmt.Fprintf(w, "  %*s    %-*s  %-*s  [%s]\n", numW, "", relW, l.Relation, linkW, l.Key, l.Status)
			}
		}
	}
	return nil
}
