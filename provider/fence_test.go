package provider

import "testing"

func TestFence(t *testing.T) {
	cases := []struct{ name, code, lang, want string }{
		{"language", "select 1", "sql", "```sql\nselect 1\n```"},
		{"no language", "a\nb", "", "```\na\nb\n```"},
		{"blank ends", "\n  \n  a\n\n  b\n\n", "", "```\n  a\n\n  b\n```"},
		{"line endings", "a\r\nb", "", "```\na\nb\n```"},
		{"backticks inside", "x\n```\ny", "", "````\nx\n```\ny\n````"},
		{"longer run inside", "a ````` b", "", "``````\na ````` b\n``````"},
		{"language is one word", "x", " t`sql extra ", "```tsql\nx\n```"},
		{"empty", "", "", "```\n```"},
	}
	for _, c := range cases {
		if got := Fence(c.code, c.lang); got != c.want {
			t.Errorf("%s:\n got  %q\n want %q", c.name, got, c.want)
		}
	}
}
