package provider

import "testing"

func TestKeyRef(t *testing.T) {
	for _, c := range []struct{ in, project, want string }{
		{"4039", "ABC", "ABC-4039"},
		{" #4039 ", "abc", "ABC-4039"},
		{"abc-12", "", "ABC-12"},
		{"XY-7", "ABC", "XY-7"},
		{"https://site-2.example.com/browse/ABC-4039", "", "ABC-4039"},
		{"https://example.com/browse/ABC-4039?focusedCommentId=12345", "", "ABC-4039"},
		{"https://example.com/projects/ABC/boards/5?selectedIssue=ABC-88", "XY", "ABC-88"},
		{"https://example.com/projects/ABC/queues/custom/12/ABC-9\n", "", "ABC-9"},
	} {
		got, err := KeyRef(c.in, c.project)
		if err != nil || got != c.want {
			t.Errorf("KeyRef(%q, %q) = %q, %v; want %q", c.in, c.project, got, err, c.want)
		}
	}
	for _, in := range []string{"", "  ", "4039", "hello", "https://site-2.example.com/", "https://site-2.example.com"} {
		if got, err := KeyRef(in, ""); err == nil {
			t.Errorf("KeyRef(%q) = %q, want an error", in, got)
		}
	}
}

func TestNumberRef(t *testing.T) {
	for in, want := range map[string]string{
		"123":  "123",
		"#123": "123",
		"https://example.com/org/Team-2/_workitems/edit/123":        "123",
		"https://example.com/org/proj/_workitems/edit/123/":         "123",
		"https://example.com/org/proj/_sprints/board/7?workitem=45": "45",
		"https://example.com/org/proj/_backlogs/7/8":                "8",
		"https://example.com:8080/org/proj/_workitems/edit/5#99":    "99",
	} {
		got, err := NumberRef(in)
		if err != nil || got != want {
			t.Errorf("NumberRef(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "https://example.com:8080/org/proj2"} {
		if got, err := NumberRef(in); err == nil {
			t.Errorf("NumberRef(%q) = %q, want an error", in, got)
		}
	}
}
