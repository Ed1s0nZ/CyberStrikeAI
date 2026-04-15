package burp;

import javax.swing.*;
import java.awt.*;
import java.util.ArrayList;
import java.util.List;

public class BurpExtender implements IBurpExtender, IContextMenuFactory {
    private IBurpExtenderCallbacks callbacks;
    private IExtensionHelpers helpers;

    private CyberStrikeAITab tab;
    private final CyberStrikeAIClient client = new CyberStrikeAIClient();
    private String lastInstruction = HttpMessageFormatter.defaultInstruction();

    @Override
    public void registerExtenderCallbacks(IBurpExtenderCallbacks callbacks) {
        this.callbacks = callbacks;
        this.helpers = callbacks.getHelpers();

        callbacks.setExtensionName("能盾智御 Extension");

        this.tab = new CyberStrikeAITab();
        callbacks.addSuiteTab(tab);

        callbacks.registerContextMenuFactory(this);

        callbacks.printOutput("能盾智御 extension loaded.");
    }

    @Override
    public List<JMenuItem> createMenuItems(IContextMenuInvocation invocation) {
        List<JMenuItem> items = new ArrayList<>();

        JMenuItem sendItem = new JMenuItem("Send to 能盾智御 (stream test)");
        sendItem.addActionListener(e -> {
            IHttpRequestResponse[] selected = invocation.getSelectedMessages();
            if (selected == null || selected.length == 0) {
                return;
            }
            sendMessage(selected[0]);
        });

        items.add(sendItem);
        return items;
    }

    private void sendMessage(IHttpRequestResponse msg) {
        if (msg == null) return;
        CyberStrikeAIClient.Config cfg = tab.currentConfig();
        String token = tab.getToken();
        if (token == null || token.trim().isEmpty()) {
            JOptionPane.showMessageDialog(tab.getUiComponent(),
                    "Please click Validate first to obtain a token.",
                    "能盾智御", JOptionPane.WARNING_MESSAGE);
            return;
        }

        String instruction = showInstructionEditor(tab.getUiComponent(), lastInstruction);
        if (instruction == null) {
            return;
        }
        lastInstruction = instruction;

        String prompt = HttpMessageFormatter.toPrompt(helpers, msg, instruction);
        String title = HttpMessageFormatter.getRequestTitle(helpers, msg);
        String agentModeStr = (cfg.agentMode == CyberStrikeAIClient.AgentMode.MULTI) ? "Multi Agent" : "Single Agent";
        String runId = tab.startNewRun(title, agentModeStr, msg);
        tab.appendProgressToRun(runId, "\n[server] " + cfg.baseUrl + "\n\n");

        client.streamTest(cfg, token, prompt, new CyberStrikeAIClient.StreamListener() {
            @Override
            public void onEvent(String type, String message, String rawJson) {
                if (type == null) type = "";
                switch (type) {
                    case "response_delta":
                    case "eino_agent_reply_stream_delta":
                        tab.appendFinalToRun(runId, message);
                        break;
                    case "response":
                        tab.appendFinalToRun(runId, "\n\n--- Final Response ---\n");
                        tab.appendFinalToRun(runId, message);
                        tab.setFinalResponse(runId, message);
                        break;
                    case "progress":
                        tab.appendProgressToRun(runId, "\n[progress] " + message + "\n");
                        tab.setRunStatus(runId, "running");
                        break;
                    case "cancelled":
                        tab.appendProgressToRun(runId, "\n[cancelled] " + message + "\n");
                        tab.setRunStatus(runId, "cancelled");
                        break;
                    case "error":
                        tab.appendProgressToRun(runId, "\n[error] " + message + "\n");
                        tab.setRunStatus(runId, "error");
                        break;
                    case "thinking_stream_start":
                        if (tab.isShowDebugEvents()) {
                            tab.resetThinkingStream(runId);
                        }
                        break;
                    case "thinking_stream_delta":
                    case "tool_call":
                    case "tool_result":
                    case "tool_result_delta":
                        if (tab.isShowDebugEvents() && message != null && !message.isEmpty()) {
                            if ("thinking_stream_delta".equals(type)) {
                                tab.appendThinkingDelta(runId, message);
                            } else {
                                tab.appendProgressToRun(runId, "\n[" + type + "] " + message + "\n");
                            }
                        }
                        break;
                    case "conversation":
                        if (rawJson != null) {
                            String convId = SimpleJson.extractStringField(rawJson, "conversationId");
                            if (convId != null && !convId.trim().isEmpty()) {
                                tab.setRunConversationId(runId, convId);
                            }
                        }
                        if (tab.isShowDebugEvents() && message != null && !message.isEmpty()) {
                            tab.appendProgressToRun(runId, "\n[" + type + "] " + message + "\n");
                        }
                        break;
                    case "done":
                        break;
                    default:
                        if (tab.isShowDebugEvents() && message != null && !message.isEmpty()) {
                            tab.appendProgressToRun(runId, "\n[" + type + "] " + message + "\n");
                        }
                        break;
                }
            }

            @Override
            public void onError(String message, Exception e) {
                tab.appendProgressToRun(runId, "\n[error] " + message + "\n");
                tab.setRunStatus(runId, "error");
                callbacks.printError("能盾智御 stream error: " + message);
                if (e != null) {
                    callbacks.printError(e.toString());
                }
            }

            @Override
            public void onDone() {
                tab.appendProgressToRun(runId, "\n\n[done]\n");
                tab.setRunStatus(runId, "done");
            }
        });
    }

    private static String showInstructionEditor(Component parent, String initialValue) {
        JTextArea editor = new JTextArea(
                initialValue == null || initialValue.trim().isEmpty()
                        ? HttpMessageFormatter.defaultInstruction()
                        : initialValue,
                6,
                70
        );
        editor.setLineWrap(true);
        editor.setWrapStyleWord(true);
        editor.setFont(new Font(Font.SANS_SERIF, Font.PLAIN, 13));

        JPanel panel = new JPanel(new BorderLayout(0, 8));
        panel.add(new JLabel("Edit instruction before sending:"), BorderLayout.NORTH);
        panel.add(new JScrollPane(editor), BorderLayout.CENTER);

        int result = JOptionPane.showConfirmDialog(
                parent,
                panel,
                "Customize Prompt Instruction",
                JOptionPane.OK_CANCEL_OPTION,
                JOptionPane.PLAIN_MESSAGE
        );
        if (result != JOptionPane.OK_OPTION) {
            return null;
        }
        String value = editor.getText();
        if (value == null || value.trim().isEmpty()) {
            return HttpMessageFormatter.defaultInstruction();
        }
        return value.trim();
    }
}
