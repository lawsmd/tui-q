package people

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lawsmd/tui-q/ticket"
)

func TestPeopleAreKeptMostRecentFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "people", "site.json")
	s := Open(path)
	if got := s.Match(""); len(got) != 0 {
		t.Fatalf("a new store holds %+v", got)
	}
	alex, sam := ticket.User{ID: "u-1", Name: "Alex Example"}, ticket.User{ID: "u-2", Name: "Sam Sample"}
	if err := s.Note(alex, ticket.User{ID: "", Name: "No ID"}, ticket.User{ID: "u-9"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Note(sam); err != nil {
		t.Fatal(err)
	}
	if got := s.Match(" "); !reflect.DeepEqual(got, []ticket.User{sam, alex}) {
		t.Fatalf("everyone = %+v", got)
	}
	if got := s.Match("EXAM"); !reflect.DeepEqual(got, []ticket.User{alex}) {
		t.Fatalf("match = %+v", got)
	}
	if got := s.Match("nobody"); len(got) != 0 {
		t.Fatalf("match = %+v", got)
	}

	// Mentioned again, a person moves to the front and takes the new name.
	renamed := ticket.User{ID: "u-1", Name: "Alex Renamed"}
	if err := s.Note(renamed); err != nil {
		t.Fatal(err)
	}
	if got := Open(path).Match(""); !reflect.DeepEqual(got, []ticket.User{renamed, sam}) {
		t.Fatalf("read back = %+v", got)
	}
}

func TestPeopleAreLimitedAndStayInMemoryWithNoPath(t *testing.T) {
	s := Open("")
	for i := 0; i < Limit+5; i++ {
		if err := s.Note(ticket.User{ID: fmt.Sprint("u-", i), Name: fmt.Sprint("Person ", i)}); err != nil {
			t.Fatal(err)
		}
	}
	got := s.Match("")
	if len(got) != Limit || got[0].ID != fmt.Sprint("u-", Limit+4) {
		t.Fatalf("%d people, first %+v", len(got), got[0])
	}
}

func TestLinks(t *testing.T) {
	md := "hi [@Alex Example](mention:id%3A1) and [@Sam](mention:u-2), again [@Alex Example](mention:id%3A1)\n" +
		"[a link](https://example.com) @plain [@](mention:u-3)"
	want := []ticket.User{{ID: "id:1", Name: "Alex Example"}, {ID: "u-2", Name: "Sam"}}
	if got := Links(md); !reflect.DeepEqual(got, want) {
		t.Fatalf("Links = %+v", got)
	}
}
