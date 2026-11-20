package tuiq

import (
	"testing"

	"github.com/lawsmd/tui-q/provider/fake"
)

func TestTicketArg(t *testing.T) {
	prov := fake.New(0)
	for _, c := range []struct {
		arg  string
		rest []string
		want string // empty when the argument is refused
	}{
		{arg: "https://example.com/browse/DEMO-104", want: "DEMO-104"},
		{arg: "https://example.com/browse/DEMO-104?focusedCommentId=5", want: "DEMO-104"},
		{arg: "demo-104", want: "DEMO-104"},
		{arg: "104", want: "DEMO-104"},
		{arg: "chekc"},
		{arg: "--queue"},
		{arg: "DEMO-104", rest: []string{"extra"}},
	} {
		got, err := ticketArg(prov, c.arg, c.rest)
		if (err != nil) != (c.want == "") || got != c.want {
			t.Errorf("ticketArg(%q, %q) = %q, %v; want %q", c.arg, c.rest, got, err, c.want)
		}
	}
}
