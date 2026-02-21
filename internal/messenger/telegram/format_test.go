package telegram

import (
	"testing"
)

func TestMarkdownToHTML_WindowsPath(t *testing.T) {
	input := `My working directory is C:\Users\dika1\AppData\Roaming\opencrons\workspace`
	got := markdownToHTML(input)
	want := `My working directory is C:\Users\dika1\AppData\Roaming\opencrons\workspace`
	if got != want {
		t.Errorf("backslashes lost:\n got: %s\nwant: %s", got, want)
	}
}

func TestMarkdownToHTML_CodeBlock(t *testing.T) {
	input := "```\nC:\\Users\\test\n```"
	got := markdownToHTML(input)
	want := "<pre><code>\nC:\\Users\\test\n</code></pre>"
	if got != want {
		t.Errorf("code block:\n got: %s\nwant: %s", got, want)
	}
}

func TestMarkdownToHTML_CodeBlockWithLang(t *testing.T) {
	input := "```python\nprint(\"hello\")\n```"
	got := markdownToHTML(input)
	want := "<pre><code class=\"language-python\">\nprint(&#34;hello&#34;)\n</code></pre>"
	if got != want {
		t.Errorf("code block with lang:\n got: %s\nwant: %s", got, want)
	}
}

func TestMarkdownToHTML_InlineCode(t *testing.T) {
	input := "Use `fmt.Println` to print"
	got := markdownToHTML(input)
	want := "Use <code>fmt.Println</code> to print"
	if got != want {
		t.Errorf("inline code:\n got: %s\nwant: %s", got, want)
	}
}

func TestMarkdownToHTML_Bold(t *testing.T) {
	input := "This is **bold** text"
	got := markdownToHTML(input)
	want := "This is <b>bold</b> text"
	if got != want {
		t.Errorf("bold:\n got: %s\nwant: %s", got, want)
	}
}

func TestMarkdownToHTML_Italic(t *testing.T) {
	input := "This is *italic* text"
	got := markdownToHTML(input)
	want := "This is <i>italic</i> text"
	if got != want {
		t.Errorf("italic:\n got: %s\nwant: %s", got, want)
	}
}

func TestMarkdownToHTML_Link(t *testing.T) {
	input := "Visit [Google](https://google.com) now"
	got := markdownToHTML(input)
	want := `Visit <a href="https://google.com">Google</a> now`
	if got != want {
		t.Errorf("link:\n got: %s\nwant: %s", got, want)
	}
}

func TestMarkdownToHTML_Header(t *testing.T) {
	input := "## My Header"
	got := markdownToHTML(input)
	want := "<b>My Header</b>"
	if got != want {
		t.Errorf("header:\n got: %s\nwant: %s", got, want)
	}
}

func TestMarkdownToHTML_HTMLEscape(t *testing.T) {
	input := "Use <div> & \"quotes\""
	got := markdownToHTML(input)
	want := "Use &lt;div&gt; &amp; &#34;quotes&#34;"
	if got != want {
		t.Errorf("html escape:\n got: %s\nwant: %s", got, want)
	}
}

func TestMarkdownToHTML_Mixed(t *testing.T) {
	input := "Path: `C:\\Users\\test`\n\n**Note:** Use backslash \\ on Windows"
	got := markdownToHTML(input)
	want := "Path: <code>C:\\Users\\test</code>\n\n<b>Note:</b> Use backslash \\ on Windows"
	if got != want {
		t.Errorf("mixed:\n got: %s\nwant: %s", got, want)
	}
}
