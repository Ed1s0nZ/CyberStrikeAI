package burp;

import java.util.ArrayList;
import java.util.List;

/**
 * Minimal Markdown -> HTML renderer for Burp UI.
 * Supports: headings (#..######), fenced code blocks (```), inline code (`),
 * bold (**), lists (-/*), paragraphs, and basic escaping.
 *
 * Not a full CommonMark implementation; kept dependency-free on purpose.
 */
final class MarkdownRenderer {
    private MarkdownRenderer() {}

    static String toHtml(String markdown) {
        if (markdown == null) markdown = "";

        List<String> lines = splitLines(markdown);
        StringBuilder out = new StringBuilder(4096);
        out.append("<html><head><meta charset='utf-8'>")
                .append("<style>")
                // Swing's HTML renderer does not reliably apply default heading sizes,
                // so we explicitly define font sizes to keep a clear hierarchy.
                .append("body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Arial,sans-serif;font-size:13px;line-height:1.45;margin:10px;color:#111;}")
                .append("code,pre{font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace;}")
                // Keep inline code readable (Swing may render it too small otherwise).
                .append("code{font-size:0.95em;background:#f6f8fa;border:1px solid #e5e7eb;border-radius:4px;padding:0 4px;}")
                .append("pre{font-size:0.95em;background:#f6f8fa;border:1px solid #e5e7eb;border-radius:6px;padding:10px;overflow:auto;}")
                .append("pre code{font-size:1em;background:transparent;border:none;padding:0;}")
                .append("p{margin:0.55em 0;}")
                .append("h1{font-size:20px;margin:0.85em 0 0.45em 0;}")
                .append("h2{font-size:18px;margin:0.85em 0 0.45em 0;}")
                .append("h3{font-size:16px;margin:0.8em 0 0.4em 0;}")
                .append("h4{font-size:14px;margin:0.8em 0 0.4em 0;}")
                .append("h5{font-size:13px;margin:0.75em 0 0.35em 0;}")
                .append("h6{font-size:13px;margin:0.75em 0 0.35em 0;}")
                .append("ul{margin:0.4em 0 0.6em 1.2em;padding:0;}")
                .append("</style></head><body>");

        boolean inCode = false;
        boolean inList = false;
        StringBuilder codeBuf = new StringBuilder();

        for (String raw : lines) {
            String line = raw == null ? "" : raw;

            if (line.trim().startsWith("```")) {
                if (!inCode) {
                    inCode = true;
                    codeBuf.setLength(0);
                } else {
                    // close code
                    out.append("<pre><code>")
                            .append(escapeHtml(codeBuf.toString()))
                            .append("</code></pre>");
                    inCode = false;
                }
                continue;
            }

            if (inCode) {
                codeBuf.append(line).append("\n");
                continue;
            }

            String trimmed = line.trim();
            if (trimmed.isEmpty()) {
                if (inList) {
                    out.append("</ul>");
                    inList = false;
                }
                out.append("<div style='height:6px'></div>");
                continue;
            }

            // headings
            int h = headingLevel(trimmed);
            if (h > 0) {
                if (inList) {
                    out.append("</ul>");
                    inList = false;
                }
                String text = trimmed.substring(h).trim();
                out.append("<h").append(h).append(">")
                        .append(inlineFormat(text))
                        .append("</h").append(h).append(">");
                continue;
            }

            // list items
            if (trimmed.startsWith("- ") || trimmed.startsWith("* ")) {
                if (!inList) {
                    out.append("<ul>");
                    inList = true;
                }
                String item = trimmed.substring(2).trim();
                out.append("<li>").append(inlineFormat(item)).append("</li>");
                continue;
            }

            // normal paragraph
            if (inList) {
                out.append("</ul>");
                inList = false;
            }
            out.append("<p>").append(inlineFormat(trimmed)).append("</p>");
        }

        if (inCode) {
            out.append("<pre><code>")
                    .append(escapeHtml(codeBuf.toString()))
                    .append("</code></pre>");
        }
        if (inList) {
            out.append("</ul>");
        }

        out.append("</body></html>");
        return out.toString();
    }

    private static int headingLevel(String s) {
        int i = 0;
        while (i < s.length() && s.charAt(i) == '#') i++;
        if (i >= 1 && i <= 6 && i < s.length() && Character.isWhitespace(s.charAt(i))) return i;
        return 0;
    }

    private static String inlineFormat(String text) {
        // escape first, then apply simple replacements using placeholders
        String escaped = escapeHtml(text);

        // inline code: `code`
        escaped = replaceInlineCode(escaped);

        // bold: **text**
        escaped = replaceBold(escaped);

        return escaped;
    }

    private static String replaceInlineCode(String s) {
        StringBuilder out = new StringBuilder(s.length() + 16);
        boolean in = false;
        StringBuilder buf = new StringBuilder();
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            if (c == '`') {
                if (!in) {
                    in = true;
                    buf.setLength(0);
                } else {
                    out.append("<code>").append(buf).append("</code>");
                    in = false;
                }
                continue;
            }
            if (in) buf.append(c);
            else out.append(c);
        }
        if (in) {
            // unmatched backtick: keep as literal
            out.append("`").append(buf);
        }
        return out.toString();
    }

    private static String replaceBold(String s) {
        // simple non-nested **...**
        StringBuilder out = new StringBuilder(s.length() + 16);
        int i = 0;
        while (i < s.length()) {
            int start = s.indexOf("**", i);
            if (start < 0) {
                out.append(s.substring(i));
                break;
            }
            int end = s.indexOf("**", start + 2);
            if (end < 0) {
                out.append(s.substring(i));
                break;
            }
            out.append(s.substring(i, start));
            out.append("<b>").append(s, start + 2, end).append("</b>");
            i = end + 2;
        }
        return out.toString();
    }

    private static String escapeHtml(String s) {
        if (s == null) return "";
        return s.replace("&", "&amp;")
                .replace("<", "&lt;")
                .replace(">", "&gt;")
                .replace("\"", "&quot;");
    }

    private static List<String> splitLines(String s) {
        String[] parts = s.split("\\r?\\n", -1);
        List<String> lines = new ArrayList<>(parts.length);
        for (String p : parts) lines.add(p);
        return lines;
    }
}

