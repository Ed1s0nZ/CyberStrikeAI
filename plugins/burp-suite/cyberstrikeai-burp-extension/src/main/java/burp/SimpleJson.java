package burp;

import java.util.HashMap;
import java.util.Map;

/**
 * Minimal JSON extractor for the SSE payloads we emit:
 * {"type":"...","message":"...","data":...}
 *
 * This is NOT a general-purpose JSON parser; it's intentionally small to avoid external deps.
 */
final class SimpleJson {
    private SimpleJson() {}

    static Map<String, String> extractTopLevelStringFields(String json, String... keys) {
        Map<String, String> out = new HashMap<>();
        if (json == null) return out;
        for (String key : keys) {
            out.put(key, extractStringField(json, key));
        }
        return out;
    }

    static String extractStringField(String json, String key) {
        if (json == null || key == null) return "";
        String needle = "\"" + key + "\"";
        int k = json.indexOf(needle);
        if (k < 0) return "";
        int colon = json.indexOf(':', k + needle.length());
        if (colon < 0) return "";
        int i = colon + 1;
        while (i < json.length() && Character.isWhitespace(json.charAt(i))) i++;
        if (i >= json.length() || json.charAt(i) != '"') return "";
        i++; // after opening quote
        StringBuilder sb = new StringBuilder();
        boolean esc = false;
        while (i < json.length()) {
            char c = json.charAt(i++);
            if (esc) {
                switch (c) {
                    case '"': sb.append('"'); break;
                    case '\\': sb.append('\\'); break;
                    case '/': sb.append('/'); break;
                    case 'b': sb.append('\b'); break;
                    case 'f': sb.append('\f'); break;
                    case 'n': sb.append('\n'); break;
                    case 'r': sb.append('\r'); break;
                    case 't': sb.append('\t'); break;
                    case 'u':
                        if (i + 3 < json.length()) {
                            String hex = json.substring(i, i + 4);
                            try {
                                sb.append((char) Integer.parseInt(hex, 16));
                                i += 4;
                            } catch (NumberFormatException ignored) {
                                // best-effort: keep raw
                                sb.append("\\u").append(hex);
                                i += 4;
                            }
                        }
                        break;
                    default:
                        sb.append(c);
                }
                esc = false;
                continue;
            }
            if (c == '\\') {
                esc = true;
                continue;
            }
            if (c == '"') {
                break;
            }
            sb.append(c);
        }
        return sb.toString();
    }
}

