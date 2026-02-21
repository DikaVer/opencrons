// format.go converts markdown text to Telegram-compatible HTML.
//
// Claude Code outputs standard markdown which doesn't map cleanly to
// Telegram's MarkdownV2 (which requires escaping many common characters
// like backslashes, periods, etc.). HTML mode is more forgiving and
// preserves characters like backslashes in Windows paths verbatim.
package telegram

import (
	"html"
	"regexp"
	"strings"
)

var (
	boldRe   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	italicRe = regexp.MustCompile(`\*([^*]+?)\*`)
	linkRe   = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// markdownToHTML converts markdown text to Telegram-compatible HTML.
// Handles code blocks, inline code, bold, italic, links, and headers.
func markdownToHTML(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	inCode := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				out = append(out, "</code></pre>")
				inCode = false
			} else {
				lang := strings.TrimPrefix(trimmed, "```")
				lang = strings.TrimSpace(lang)
				if lang != "" {
					out = append(out, `<pre><code class="language-`+html.EscapeString(lang)+`">`)
				} else {
					out = append(out, "<pre><code>")
				}
				inCode = true
			}
			continue
		}

		if inCode {
			out = append(out, html.EscapeString(line))
			continue
		}

		// Headers → bold
		if strings.HasPrefix(trimmed, "#") {
			hdr := strings.TrimLeft(trimmed, "#")
			hdr = strings.TrimSpace(hdr)
			if hdr != "" {
				out = append(out, "<b>"+html.EscapeString(hdr)+"</b>")
				continue
			}
		}

		out = append(out, processInline(line))
	}

	if inCode {
		out = append(out, "</code></pre>")
	}

	return strings.Join(out, "\n")
}

// processInline handles inline formatting: code spans, bold, italic, links.
func processInline(line string) string {
	type seg struct {
		text   string
		isCode bool
	}

	var segs []seg
	rest := line

	for rest != "" {
		idx := strings.Index(rest, "`")
		if idx < 0 {
			segs = append(segs, seg{text: rest})
			break
		}
		if idx > 0 {
			segs = append(segs, seg{text: rest[:idx]})
		}
		end := strings.Index(rest[idx+1:], "`")
		if end < 0 {
			// Unmatched backtick — treat as regular text
			segs = append(segs, seg{text: rest[idx:]})
			break
		}
		segs = append(segs, seg{text: rest[idx+1 : idx+1+end], isCode: true})
		rest = rest[idx+1+end+1:]
	}

	var b strings.Builder
	for _, s := range segs {
		if s.isCode {
			b.WriteString("<code>")
			b.WriteString(html.EscapeString(s.text))
			b.WriteString("</code>")
		} else {
			b.WriteString(formatTextHTML(s.text))
		}
	}
	return b.String()
}

// formatTextHTML applies HTML escaping and converts markdown formatting to HTML tags.
func formatTextHTML(text string) string {
	text = html.EscapeString(text)
	text = boldRe.ReplaceAllString(text, "<b>$1</b>")
	text = italicRe.ReplaceAllString(text, "<i>$1</i>")
	text = linkRe.ReplaceAllString(text, `<a href="$2">$1</a>`)
	return text
}
