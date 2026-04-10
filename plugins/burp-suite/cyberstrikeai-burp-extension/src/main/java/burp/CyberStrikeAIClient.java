package burp;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;

final class CyberStrikeAIClient {

    static final class Config {
        final String baseUrl; // e.g. http://127.0.0.1:8080
        final String password;
        final AgentMode agentMode;

        Config(String baseUrl, String password, AgentMode agentMode) {
            this.baseUrl = baseUrl;
            this.password = password;
            this.agentMode = agentMode;
        }
    }

    enum AgentMode {
        SINGLE,
        MULTI
    }

    interface StreamListener {
        void onEvent(String type, String message, String rawJson);
        void onError(String message, Exception e);
        void onDone();
    }

    String loginAndValidate(Config cfg) throws IOException {
        String token = login(cfg.baseUrl, cfg.password);
        validate(cfg.baseUrl, token);
        return token;
    }

    private String login(String baseUrl, String password) throws IOException {
        URL url = new URL(baseUrl + "/api/auth/login");
        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
        conn.setRequestMethod("POST");
        conn.setDoOutput(true);
        conn.setRequestProperty("Content-Type", "application/json");
        conn.setRequestProperty("Accept", "application/json");
        String body = "{\"password\":\"" + escapeJson(password) + "\"}";
        try (OutputStream os = conn.getOutputStream()) {
            os.write(body.getBytes(StandardCharsets.UTF_8));
        }
        int code = conn.getResponseCode();
        String contentType = conn.getHeaderField("Content-Type");
        String resp = readAll(code >= 200 && code < 300 ? conn.getInputStream() : conn.getErrorStream());

        // Friendly diagnosis: HTML usually means wrong host/port (e.g., hit Burp UI/proxy page).
        if (looksLikeHtml(resp) || (contentType != null && contentType.toLowerCase().contains("text/html"))) {
            throw new IOException("Login failed: server returned HTML, not API JSON. Check IP/Port and ensure you point to CyberStrikeAI backend.");
        }

        String serverError = SimpleJson.extractStringField(resp, "error");
        if (code < 200 || code >= 300) {
            if (!serverError.isEmpty()) {
                throw new IOException("Login failed (" + code + "): " + serverError);
            }
            throw new IOException("Login failed (" + code + ").");
        }

        if (!serverError.isEmpty()) {
            throw new IOException("Login failed: " + serverError);
        }

        String token = SimpleJson.extractStringField(resp, "token");
        if (token.isEmpty()) {
            throw new IOException("Login response missing token. Check backend address and credentials.");
        }
        return token;
    }

    private void validate(String baseUrl, String token) throws IOException {
        URL url = new URL(baseUrl + "/api/auth/validate");
        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
        conn.setRequestMethod("GET");
        conn.setRequestProperty("Authorization", "Bearer " + token);
        int code = conn.getResponseCode();
        String resp = readAll(code >= 200 && code < 300 ? conn.getInputStream() : conn.getErrorStream());
        if (code < 200 || code >= 300) {
            throw new IOException("Validate failed (" + code + "): " + resp);
        }
    }

    void streamTest(Config cfg, String token, String message, StreamListener listener) {
        String path = (cfg.agentMode == AgentMode.MULTI) ? "/api/multi-agent/stream" : "/api/agent-loop/stream";
        String urlStr = cfg.baseUrl + path;

        Map<String, Object> payload = new HashMap<>();
        payload.put("message", message);
        payload.put("conversationId", "");
        payload.put("role", "");

        new Thread(() -> {
            HttpURLConnection conn = null;
            try {
                URL url = new URL(urlStr);
                conn = (HttpURLConnection) url.openConnection();
                conn.setRequestMethod("POST");
                conn.setDoOutput(true);
                conn.setRequestProperty("Content-Type", "application/json");
                conn.setRequestProperty("Accept", "text/event-stream");
                conn.setRequestProperty("Authorization", "Bearer " + token);

                String body = toJson(payload);
                try (OutputStream os = conn.getOutputStream()) {
                    os.write(body.getBytes(StandardCharsets.UTF_8));
                }

                int code = conn.getResponseCode();
                InputStream is = (code >= 200 && code < 300) ? conn.getInputStream() : conn.getErrorStream();
                if (is == null) {
                    throw new IOException("No response body (HTTP " + code + ")");
                }

                try (BufferedReader br = new BufferedReader(new InputStreamReader(is, StandardCharsets.UTF_8))) {
                    String line;
                    while ((line = br.readLine()) != null) {
                        // SSE format: "data: {json}"
                        if (line.startsWith("data:")) {
                            String json = line.substring("data:".length()).trim();
                            if (!json.isEmpty()) {
                                String type = SimpleJson.extractStringField(json, "type");
                                String msg = SimpleJson.extractStringField(json, "message");
                                listener.onEvent(type, msg, json);
                                if ("done".equals(type)) {
                                    break;
                                }
                            }
                        }
                    }
                }
                listener.onDone();
            } catch (Exception e) {
                listener.onError(e.getMessage(), e);
            } finally {
                if (conn != null) {
                    conn.disconnect();
                }
            }
        }, "CyberStrikeAI-Stream").start();
    }

    void cancelByConversationId(String baseUrl, String token, String conversationId) throws IOException {
        if (conversationId == null || conversationId.trim().isEmpty()) {
            throw new IOException("Missing conversationId.");
        }
        URL url = new URL(baseUrl + "/api/agent-loop/cancel");
        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
        conn.setRequestMethod("POST");
        conn.setDoOutput(true);
        conn.setRequestProperty("Content-Type", "application/json");
        conn.setRequestProperty("Accept", "application/json");
        conn.setRequestProperty("Authorization", "Bearer " + token);

        String body = "{\"conversationId\":\"" + escapeJson(conversationId.trim()) + "\"}";
        try (OutputStream os = conn.getOutputStream()) {
            os.write(body.getBytes(StandardCharsets.UTF_8));
        }

        int code = conn.getResponseCode();
        String resp = readAll(code >= 200 && code < 300 ? conn.getInputStream() : conn.getErrorStream());
        if (code < 200 || code >= 300) {
            String serverError = SimpleJson.extractStringField(resp, "error");
            if (!serverError.isEmpty()) {
                throw new IOException("Cancel failed (" + code + "): " + serverError);
            }
            throw new IOException("Cancel failed (" + code + ").");
        }
    }

    private static String toJson(Map<String, Object> payload) {
        String message = payload.get("message") != null ? String.valueOf(payload.get("message")) : "";
        String conversationId = payload.get("conversationId") != null ? String.valueOf(payload.get("conversationId")) : "";
        String role = payload.get("role") != null ? String.valueOf(payload.get("role")) : "";
        return "{"
                + "\"message\":\"" + escapeJson(message) + "\","
                + "\"conversationId\":\"" + escapeJson(conversationId) + "\","
                + "\"role\":\"" + escapeJson(role) + "\""
                + "}";
    }

    private static String escapeJson(String s) {
        if (s == null) return "";
        StringBuilder sb = new StringBuilder(s.length() + 16);
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '\\': sb.append("\\\\"); break;
                case '"': sb.append("\\\""); break;
                case '\n': sb.append("\\n"); break;
                case '\r': sb.append("\\r"); break;
                case '\t': sb.append("\\t"); break;
                default:
                    if (c < 0x20) {
                        sb.append(String.format("\\u%04x", (int) c));
                    } else {
                        sb.append(c);
                    }
            }
        }
        return sb.toString();
    }

    private static String readAll(InputStream is) throws IOException {
        if (is == null) return "";
        try (BufferedReader br = new BufferedReader(new InputStreamReader(is, StandardCharsets.UTF_8))) {
            StringBuilder sb = new StringBuilder();
            String line;
            while ((line = br.readLine()) != null) {
                sb.append(line).append('\n');
            }
            return sb.toString().trim();
        }
    }

    private static boolean looksLikeHtml(String s) {
        if (s == null) return false;
        String t = s.trim().toLowerCase();
        return t.startsWith("<!doctype html") || t.startsWith("<html") || t.contains("<head>") || t.contains("<body");
    }
}

