package ado

import "testing"

func TestHTMLToMarkdown(t *testing.T) {
	for in, want := range map[string]string{
		"":                               "",
		"plain":                          "plain",
		"<p>One</p><p>Two</p>":           "One\n\nTwo",
		"<div>a<br>b</div>":              "a  \nb",
		"<h3>Steps </h3><p>x</p>":        "### Steps\n\nx",
		"<b>bold</b> and <em>it</em>":    "**bold** and *it*",
		"<code>x = 1</code>":             "`x = 1`",
		"<pre><code>a\n  b</code></pre>": "```\na\n  b\n```",
		`<pre><code class="language-sql">select 1;</code></pre>`:                    "```sql\nselect 1;\n```",
		`<pre class="lang-go">x := 1</pre>`:                                         "```go\nx := 1\n```",
		"<pre><div>a</div><div>b</div></pre>":                                       "```\na\nb\n```",
		"<pre>a<br>b</pre>":                                                         "```\na\nb\n```",
		"<pre>a\n```\nb</pre>":                                                      "````\na\n```\nb\n````",
		"<p>x</p><pre>\n\ncode\n\n</pre><p>y</p>":                                   "x\n\n```\ncode\n```\n\ny",
		`<a href="https://example.com/x">site</a>`:                                  "[site](https://example.com/x)",
		`<a href="https://example.com/x">https://example.com/x</a>`:                 "<https://example.com/x>",
		"<ul><li>one</li><li>two</li></ul>":                                         "- one\n- two",
		"<ol><li>one</li><li>two</li></ol>":                                         "1. one\n2. two",
		"<ul><li>a<ul><li>b</li></ul></li></ul>":                                    "- a\n  - b",
		"<table><tr><th>A</th><th>B</th></tr><tr><td>1</td><td>2</td></tr></table>": "A | B  \n1 | 2",
		`<img src="x.png" alt="graph">`:                                             "[image: graph]",
		"a &amp; b &lt;c&gt;":                                                       "a & b <c>",
	} {
		if got := HTMLToMarkdown(in); got != want {
			t.Errorf("HTMLToMarkdown(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}
