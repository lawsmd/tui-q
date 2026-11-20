package jira

import (
	"encoding/json"
	"testing"
)

func TestADFToMarkdown(t *testing.T) {
	cases := []struct{ name, adf, want string }{
		{
			"marks",
			`{"type":"doc","content":[{"type":"paragraph","content":[
				{"type":"text","text":"Plain "},
				{"type":"text","text":"bold ","marks":[{"type":"strong"}]},
				{"type":"text","text":"code","marks":[{"type":"code"}]},
				{"type":"text","text":" and "},
				{"type":"text","text":"a link","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}
			]}]}`,
			"Plain **bold** `code` and [a link](https://example.com)",
		},
		{
			"heading and paragraphs",
			`{"type":"doc","content":[
				{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Steps"}]},
				{"type":"paragraph","content":[{"type":"text","text":"First."}]},
				{"type":"paragraph","content":[{"type":"text","text":"Second."}]}
			]}`,
			"## Steps\n\nFirst.\n\nSecond.",
		},
		{
			"nested lists",
			`{"type":"doc","content":[{"type":"bulletList","content":[
				{"type":"listItem","content":[
					{"type":"paragraph","content":[{"type":"text","text":"one"}]},
					{"type":"orderedList","content":[
						{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"inner a"}]}]},
						{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"inner b"}]}]}
					]}
				]},
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"two"}]}]}
			]}]}`,
			"- one\n  1. inner a\n  2. inner b\n- two",
		},
		{
			"code block",
			`{"type":"doc","content":[{"type":"codeBlock","attrs":{"language":"sql"},"content":[
				{"type":"text","text":"select *\nfrom t"}
			]}]}`,
			"```sql\nselect *\nfrom t\n```",
		},
		{
			"quote, rule, mention, media",
			`{"type":"doc","content":[
				{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quoted"}]}]},
				{"type":"rule"},
				{"type":"paragraph","content":[{"type":"mention","attrs":{"text":"@Sam"}},{"type":"text","text":" look"}]},
				{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"x"}}]}
			]}`,
			"> quoted\n\n---\n\n@Sam look\n\n*[attachment]*",
		},
		{
			"table",
			`{"type":"doc","content":[{"type":"table","content":[
				{"type":"tableRow","content":[
					{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"A"}]}]},
					{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"B"}]}]}
				]},
				{"type":"tableRow","content":[
					{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"1"}]}]},
					{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"2|3"}]}]}
				]}
			]}]}`,
			"| A | B |\n| --- | --- |\n| 1 | 2\\|3 |",
		},
		{"code block holding a fence",
			`{"type":"doc","content":[{"type":"codeBlock","content":[
				{"type":"text","text":"a\n` + "```" + `\nb"}]}]}`,
			"````\na\n```\nb\n````"},
		{"null", `null`, ""},
		{"empty", ``, ""},
	}
	for _, c := range cases {
		if got := adfToMarkdown(json.RawMessage(c.adf), files{}); got != c.want {
			t.Errorf("%s:\n got  %q\n want %q", c.name, got, c.want)
		}
	}
}
