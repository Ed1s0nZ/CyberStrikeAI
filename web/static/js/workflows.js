(function () {
    'use strict';

    function _t(key, opts) {
        if (typeof window.t === 'function') {
            try {
                var translated = window.t(key, opts);
                if (typeof translated === 'string' && translated && translated !== key) {
                    return translated;
                }
            } catch (e) { /* ignore */ }
        }
        return key;
    }

    let workflows = [];
    let currentWorkflowId = '';
    let cy = null;
    let nodeSeq = 1;
    let edgeSeq = 1;
    let connectMode = false;
    let connectSourceId = '';
    let selectedElement = null;
    let workflowToolOptions = [];
    let workflowToolsLoaded = false;

    const KNOWN_NODE_LABELS = {
        start: ['开始', 'Start'],
        tool: ['工具', 'Tool'],
        agent: ['Agent'],
        condition: ['条件', 'Condition'],
        hitl: ['审批', 'Approval'],
        output: ['输出', 'Output'],
        end: ['结束', 'End']
    };
    const KNOWN_EDGE_LABELS = {
        yes: ['是', 'Yes'],
        no: ['否', 'No']
    };

    function wfNodeLabel(type) {
        const key = type && KNOWN_NODE_LABELS[type] ? 'workflows.nodes.' + type : 'workflows.nodes.default';
        return _t(key);
    }

    const AGENT_MODES = ['eino_single', 'deep', 'plan_execute', 'supervisor'];

    const WORKFLOW_EDIT_ICON = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>';

    function esc(text) {
        if (typeof escapeHtml === 'function') return escapeHtml(text == null ? '' : String(text));
        return String(text == null ? '' : text)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#39;');
    }

    const BINDING_FROM_OPTIONS = ['previous', 'inputs', 'outputs'];

    function bindingFromConfig(cfg, key, fallbackFrom, fallbackField) {
        const b = cfg && cfg[key];
        if (b && typeof b === 'object') {
            return {
                from: b.from || fallbackFrom,
                field: b.field || fallbackField
            };
        }
        return { from: fallbackFrom, field: fallbackField };
    }

    function bindingFieldHtml(prefix, labelKey, binding, hintKey) {
        const from = binding.from || 'previous';
        const field = binding.field || 'output';
        const label = _t(labelKey);
        const hint = hintKey ? _t(hintKey) : '';
        const options = BINDING_FROM_OPTIONS.map(v =>
            `<option value="${esc(v)}" ${v === from ? 'selected' : ''}>${esc(v)}</option>`
        ).join('');
        return `
            <div class="form-group">
                <label>${esc(label)}</label>
                <div class="workflow-binding-row" style="display:flex;gap:8px;">
                    <select id="${prefix}-from" onchange="updateWorkflowTypedConfig()" style="flex:1;">${options}</select>
                    <input type="text" id="${prefix}-field" value="${esc(field)}" placeholder="output" oninput="updateWorkflowTypedConfig()" style="flex:1;">
                </div>
                ${hint ? '<p class="workflow-config-hint">' + hint + '</p>' : ''}
            </div>`;
    }

    function readBinding(prefix) {
        return {
            from: (document.getElementById(prefix + '-from') || {}).value || 'previous',
            field: (document.getElementById(prefix + '-field') || {}).value || 'output'
        };
    }

    function defaultGraph() {
        return { nodes: [], edges: [], config: {} };
    }

    function defaultConfigForType(type) {
        switch (type) {
            case 'start':
                return { input_keys: 'message, conversationId, projectId' };
            case 'tool':
                return { tool_name: '', arguments: '{}', timeout_seconds: '' };
            case 'agent':
                return { agent_mode: 'eino_single', input_binding: { from: 'previous', field: 'output' }, instruction: '', output_key: 'agent_result' };
            case 'condition':
                return { expression: '{{previous.output}} != ""' };
            case 'hitl':
                return { prompt: _t('workflows.defaultHitlPrompt'), prompt_binding: { from: 'previous', field: 'output' }, reviewer: 'human' };
            case 'output':
                return { output_key: 'result', source_binding: { from: 'previous', field: 'output' } };
            case 'end':
                return { result_binding: { from: 'outputs', field: 'result' } };
            default:
                return {};
        }
    }

    function configWithDefaults(type, config) {
        return Object.assign(defaultConfigForType(type), config && typeof config === 'object' ? config : {});
    }

    function parseGraph(raw) {
        if (!raw) return defaultGraph();
        let graph = raw;
        if (typeof raw === 'string') {
            try {
                graph = JSON.parse(raw);
            } catch (_) {
                return defaultGraph();
            }
        }
        return {
            nodes: Array.isArray(graph.nodes) ? graph.nodes : [],
            edges: Array.isArray(graph.edges) ? graph.edges : [],
            config: graph.config && typeof graph.config === 'object' ? graph.config : {}
        };
    }

    function graphToElements(graph) {
        const nodes = (graph.nodes || []).map((node, index) => ({
            group: 'nodes',
            data: {
                id: node.id || `node-${index + 1}`,
                label: node.label || wfNodeLabel(node.type) || node.id || _t('workflows.nodeFallback', { n: index + 1 }),
                type: node.type || 'tool',
                config: configWithDefaults(node.type || 'tool', node.config)
            },
            position: node.position || { x: 120 + index * 80, y: 120 + index * 40 }
        }));
        const edges = (graph.edges || []).map((edge, index) => ({
            group: 'edges',
            data: {
                id: edge.id || `edge-${index + 1}`,
                source: edge.source,
                target: edge.target,
                label: edge.label || '',
                config: edge.config && typeof edge.config === 'object' ? edge.config : {}
            }
        })).filter(edge => edge.data.source && edge.data.target);
        return nodes.concat(edges);
    }

    function elementsToGraph() {
        if (!cy) return defaultGraph();
        return {
            nodes: cy.nodes().map(node => ({
                id: node.id(),
                type: node.data('type') || 'tool',
                label: node.data('label') || '',
                position: node.position(),
                config: node.data('config') || {}
            })),
            edges: cy.edges().map(edge => ({
                id: edge.id(),
                source: edge.source().id(),
                target: edge.target().id(),
                label: edge.data('label') || '',
                config: edge.data('config') || {}
            })),
            config: { schema_version: 1 }
        };
    }

    function updateEmptyState() {
        const empty = document.getElementById('workflow-canvas-empty');
        if (!empty || !cy) return;
        empty.style.display = cy.nodes().length ? 'none' : 'flex';
    }

    let workflowResizeObserver = null;

    function setupWorkflowResizeObserver(container) {
        if (workflowResizeObserver || typeof ResizeObserver === 'undefined' || !container) return;
        workflowResizeObserver = new ResizeObserver(function () {
            if (cy) cy.resize();
        });
        const canvasWrap = container.closest('.workflow-canvas-wrap');
        const pageContent = container.closest('.workflow-page-content');
        if (canvasWrap) workflowResizeObserver.observe(canvasWrap);
        if (pageContent) workflowResizeObserver.observe(pageContent);
    }

    function initCy() {
        const container = document.getElementById('workflow-canvas');
        if (!container || typeof cytoscape !== 'function') return;
        if (cy) {
            cy.resize();
            return;
        }
        cy = cytoscape({
            container,
            elements: [],
            wheelSensitivity: 0.18,
            style: [
                {
                    selector: 'node',
                    style: {
                        'shape': 'round-rectangle',
                        'width': 150,
                        'height': 52,
                        'background-color': '#1d4ed8',
                        'border-width': 1,
                        'border-color': '#60a5fa',
                        'label': 'data(label)',
                        'color': '#e5edff',
                        'font-size': 13,
                        'font-weight': 700,
                        'text-valign': 'center',
                        'text-halign': 'center',
                        'text-wrap': 'wrap',
                        'text-max-width': 132
                    }
                },
                { selector: 'node[type="start"]', style: { 'background-color': '#047857', 'border-color': '#34d399' } },
                { selector: 'node[type="tool"]', style: { 'background-color': '#1d4ed8', 'border-color': '#60a5fa' } },
                { selector: 'node[type="agent"]', style: { 'background-color': '#7c3aed', 'border-color': '#c4b5fd' } },
                { selector: 'node[type="condition"]', style: { 'shape': 'diamond', 'background-color': '#b45309', 'border-color': '#fbbf24', 'width': 118, 'height': 86 } },
                { selector: 'node[type="hitl"]', style: { 'background-color': '#0f766e', 'border-color': '#5eead4' } },
                { selector: 'node[type="output"]', style: { 'background-color': '#4338ca', 'border-color': '#a5b4fc' } },
                { selector: 'node[type="end"]', style: { 'background-color': '#be123c', 'border-color': '#fb7185' } },
                {
                    selector: 'edge',
                    style: {
                        'width': 2,
                        'line-color': '#64748b',
                        'target-arrow-color': '#64748b',
                        'target-arrow-shape': 'triangle',
                        'curve-style': 'bezier',
                        'label': 'data(label)',
                        'font-size': 11,
                        'color': '#cbd5e1',
                        'text-background-color': '#0f172a',
                        'text-background-opacity': 0.8,
                        'text-background-padding': 3
                    }
                },
                {
                    selector: ':selected',
                    style: {
                        'border-width': 3,
                        'border-color': '#93c5fd',
                        'line-color': '#93c5fd',
                        'target-arrow-color': '#93c5fd'
                    }
                },
                {
                    selector: '.connect-source',
                    style: {
                        'border-width': 4,
                        'border-color': '#fbbf24'
                    }
                }
            ],
            layout: { name: 'preset' }
        });
        cy.on('tap', 'node', event => {
            if (connectMode) {
                handleConnectTap(event.target);
                return;
            }
            selectWorkflowElement(event.target);
        });
        cy.on('tap', 'edge', event => {
            selectWorkflowElement(event.target);
        });
        cy.on('tap', event => {
            if (event.target === cy) {
                if (connectMode) clearConnectSource();
                selectWorkflowElement(null);
            }
        });
        cy.on('add remove', updateEmptyState);
        document.addEventListener('keydown', event => {
            const active = document.activeElement;
            const editing = active && ['INPUT', 'TEXTAREA', 'SELECT'].includes(active.tagName);
            if (editing) return;
            if (typeof currentPage !== 'undefined' && currentPage !== 'workflows') return;
            if (event.key === 'Delete' || event.key === 'Backspace') {
                event.preventDefault();
                deleteWorkflowSelection();
            }
        });
        setupWorkflowResizeObserver(container);
    }

    async function loadWorkflows(includeDisabled) {
        const response = await apiFetch(`/api/workflows?includeDisabled=${includeDisabled ? 'true' : 'false'}`);
        if (!response.ok) {
            const err = await response.json().catch(() => ({}));
            throw new Error(err.error || _t('workflows.loadFailed'));
        }
        const data = await response.json();
        workflows = data.workflows || [];
        return workflows;
    }

    async function loadWorkflowTools() {
        if (workflowToolsLoaded) return workflowToolOptions;
        const collected = [];
        const seen = new Set();
        let page = 1;
        let totalPages = 1;
        while (page <= totalPages && page <= 20) {
            const response = await apiFetch(`/api/config/tools?page=${page}&page_size=100`);
            if (!response.ok) break;
            const data = await response.json();
            totalPages = data.total_pages || 1;
            (data.tools || []).forEach(tool => {
                if (!tool || !tool.name) return;
                const key = tool.is_external && tool.external_mcp ? `${tool.external_mcp}::${tool.name}` : tool.name;
                if (seen.has(key)) return;
                seen.add(key);
                collected.push({ key, name: tool.name, enabled: tool.enabled !== false });
            });
            page += 1;
        }
        workflowToolOptions = collected;
        workflowToolsLoaded = true;
        return workflowToolOptions;
    }

    function readWorkflowMetaFromForm() {
        const idEl = document.getElementById('workflow-id');
        const nameEl = document.getElementById('workflow-name');
        const descEl = document.getElementById('workflow-description');
        const enabledEl = document.getElementById('workflow-enabled');
        return {
            id: idEl ? idEl.value.trim() : '',
            name: nameEl ? nameEl.value.trim() : '',
            description: descEl ? descEl.value.trim() : '',
            enabled: enabledEl ? enabledEl.checked : true
        };
    }

    function updateWorkflowCanvasTitle() {
        const titleEl = document.getElementById('workflow-canvas-title');
        const subtitleEl = document.getElementById('workflow-canvas-subtitle');
        if (!titleEl) return;
        const meta = readWorkflowMetaFromForm();
        const wf = workflows.find(item => item.id === currentWorkflowId);
        if (!meta.name && !meta.id) {
            titleEl.textContent = _t('workflows.untitled');
        } else {
            titleEl.textContent = meta.name || meta.id;
        }
        titleEl.classList.toggle('is-disabled', !meta.enabled);
        titleEl.title = meta.description || '';
        if (subtitleEl) {
            const parts = [];
            if (meta.id) parts.push(meta.id);
            if (wf && wf.version) parts.push(`v${wf.version}`);
            parts.push(meta.enabled ? _t('workflows.statusEnabled') : _t('workflows.statusDisabled'));
            subtitleEl.textContent = parts.join(' · ');
            subtitleEl.hidden = !parts.length;
        }
    }

    function syncWorkflowMetaIdField(locked, id) {
        const idEl = document.getElementById('workflow-id');
        const lockedEl = document.getElementById('workflow-id-locked');
        const displayEl = document.getElementById('workflow-id-display');
        const hintEl = document.querySelector('.workflow-meta-id-hint');
        const idGroup = document.getElementById('workflow-meta-id-group');
        if (!idEl) return;
        idEl.value = id || '';
        if (locked) {
            idEl.hidden = true;
            idEl.disabled = true;
            if (lockedEl) lockedEl.hidden = false;
            if (displayEl) displayEl.textContent = id || '';
            if (hintEl) hintEl.hidden = true;
            if (idGroup) idGroup.classList.add('is-locked');
        } else {
            idEl.hidden = false;
            idEl.disabled = false;
            if (lockedEl) lockedEl.hidden = true;
            if (displayEl) displayEl.textContent = '';
            if (hintEl) hintEl.hidden = false;
            if (idGroup) idGroup.classList.remove('is-locked');
        }
    }

    function syncWorkflowMetaForm(wf) {
        const nameEl = document.getElementById('workflow-name');
        const descEl = document.getElementById('workflow-description');
        const enabledEl = document.getElementById('workflow-enabled');
        if (!nameEl || !descEl || !enabledEl) return;
        syncWorkflowMetaIdField(!!wf.id, wf.id || '');
        nameEl.value = wf.name || '';
        descEl.value = wf.description || '';
        enabledEl.checked = wf.enabled !== false;
        updateWorkflowCanvasTitle();
    }

    function renderWorkflowList() {
        const list = document.getElementById('workflow-list');
        if (!list) return;
        if (!workflows.length) {
            list.innerHTML = '<div class="empty-state">' + esc(_t('workflows.emptyList')) + '</div>';
            return;
        }
        list.innerHTML = workflows.map(wf => {
            const encodedId = encodeURIComponent(wf.id);
            const isActive = wf.id === currentWorkflowId;
            const toggleTitle = esc(_t('workflows.toggleEnabled'));
            const editTitle = esc(_t('workflows.editMeta'));
            const enabled = wf.enabled !== false;
            return `
                <div class="workflow-list-item ${isActive ? 'is-active' : ''}">
                    <button type="button" class="workflow-list-main" onclick="selectWorkflow(decodeURIComponent('${encodedId}'))">
                        <span class="workflow-list-title">${esc(wf.name || wf.id)}</span>
                        <span class="workflow-list-meta">${esc(wf.id)} · v${wf.version || 1}</span>
                    </button>
                    <div class="workflow-list-actions">
                        <label class="workflow-switch" title="${toggleTitle}" onclick="event.stopPropagation()">
                            <input type="checkbox" ${enabled ? 'checked' : ''} aria-label="${toggleTitle}" onchange="toggleWorkflowEnabled(decodeURIComponent('${encodedId}'), this.checked)">
                            <span class="workflow-switch-slider" aria-hidden="true"></span>
                        </label>
                        <button type="button" class="btn-icon workflow-list-edit" title="${editTitle}" aria-label="${editTitle}" onclick="event.stopPropagation(); editWorkflowFromList(decodeURIComponent('${encodedId}'))">${WORKFLOW_EDIT_ICON}</button>
                    </div>
                </div>
            `;
        }).join('');
    }

    function nextNodeId(type) {
        while (cy && cy.getElementById(`node-${nodeSeq}`).length) nodeSeq += 1;
        const id = `node-${nodeSeq}`;
        nodeSeq += 1;
        return id;
    }

    function nextEdgeId() {
        while (cy && cy.getElementById(`edge-${edgeSeq}`).length) edgeSeq += 1;
        const id = `edge-${edgeSeq}`;
        edgeSeq += 1;
        return id;
    }

    function resetSequences(graph) {
        nodeSeq = 1;
        edgeSeq = 1;
        (graph.nodes || []).forEach(node => {
            const m = String(node.id || '').match(/^node-(\d+)$/);
            if (m) nodeSeq = Math.max(nodeSeq, Number(m[1]) + 1);
        });
        (graph.edges || []).forEach(edge => {
            const m = String(edge.id || '').match(/^edge-(\d+)$/);
            if (m) edgeSeq = Math.max(edgeSeq, Number(m[1]) + 1);
        });
    }

    function fillWorkflowForm(wf) {
        const data = wf || {};
        syncWorkflowMetaForm(data);
        currentWorkflowId = data.id ? data.id : '';
        initCy();
        if (!cy) return;
        const graph = parseGraph(data.graph_json || data.graph || defaultGraph());
        resetSequences(graph);
        cy.elements().remove();
        cy.add(graphToElements(graph));
        if (cy.nodes().length) {
            layoutWorkflowGraph(false);
        }
        selectWorkflowElement(null);
        updateEmptyState();
        renderWorkflowList();
        setTimeout(() => cy && cy.resize(), 0);
    }

    function selectWorkflowElement(ele) {
        selectedElement = ele && ele.length ? ele : null;
        const empty = document.getElementById('workflow-property-empty');
        const form = document.getElementById('workflow-property-form');
        const title = document.getElementById('workflow-property-title');
        const deleteBtn = document.getElementById('workflow-property-delete-btn');
        if (!empty || !form) return;
        if (!selectedElement) {
            empty.hidden = false;
            form.hidden = true;
            if (title) title.textContent = _t('workflows.properties');
            if (deleteBtn) deleteBtn.hidden = true;
            return;
        }
        empty.hidden = true;
        form.hidden = false;
        if (title) title.textContent = selectedElement.isNode() ? _t('workflows.nodeProperties') : _t('workflows.edgeProperties');
        if (deleteBtn) {
            deleteBtn.hidden = false;
            deleteBtn.textContent = selectedElement.isNode() ? _t('workflows.deleteNode') : _t('workflows.deleteEdge');
        }
        cy.elements().unselect();
        selectedElement.select();
        const typeWrap = document.getElementById('workflow-prop-type-wrap');
        const label = document.getElementById('workflow-prop-label');
        const type = document.getElementById('workflow-prop-type');
        label.value = selectedElement.data('label') || '';
        if (selectedElement.isNode()) {
            typeWrap.style.display = '';
            type.value = selectedElement.data('type') || 'tool';
        } else {
            typeWrap.style.display = 'none';
        }
        renderTypedConfig(selectedElement);
        renderCustomFields(stripTypedConfig(selectedElement));
    }

    function typedKeysForType(type) {
        return new Set(Object.keys(defaultConfigForType(type)));
    }

    function stripTypedConfig(ele) {
        const cfg = Object.assign({}, ele.data('config') || {});
        const typed = ele.isNode() ? typedKeysForType(ele.data('type') || 'tool') : new Set(['condition']);
        typed.forEach(key => delete cfg[key]);
        return cfg;
    }

    function typedField(id, label, value, placeholder) {
        return `
            <div class="form-group">
                <label for="${id}">${label}</label>
                <input type="text" id="${id}" class="form-input" value="${esc(value || '')}" placeholder="${esc(placeholder || '')}" oninput="updateWorkflowTypedConfig()">
            </div>
        `;
    }

    function typedTextarea(id, label, value, placeholder) {
        return `
            <div class="form-group">
                <label for="${id}">${label}</label>
                <textarea id="${id}" class="form-input" rows="4" placeholder="${esc(placeholder || '')}" oninput="updateWorkflowTypedConfig()">${esc(value || '')}</textarea>
            </div>
        `;
    }

    function renderTypedConfig(ele) {
        const wrap = document.getElementById('workflow-typed-config');
        if (!wrap || !ele) return;
        const cfg = configWithDefaults(ele.isNode() ? ele.data('type') : 'edge', ele.data('config') || {});
        if (!ele.isNode()) {
            const sourceType = ele.source().data('type') || '';
            const edgeHint = sourceType === 'condition'
                ? _t('workflows.config.edgeConditionHintCondition')
                : _t('workflows.config.edgeConditionHintExample');
            wrap.innerHTML = `
                ${typedField('workflow-edge-condition', _t('workflows.config.edgeCondition'), cfg.condition || '', edgeHint)}
                ${sourceType === 'condition' ? '<p class="workflow-config-hint">' + esc(_t('workflows.config.edgeBranchHint')) + '</p>' : ''}
            `;
            return;
        }
        const type = ele.data('type') || 'tool';
        switch (type) {
            case 'start':
                wrap.innerHTML = typedField('workflow-start-input-keys', _t('workflows.config.inputKeys'), cfg.input_keys, 'message, projectId');
                break;
            case 'tool':
                wrap.innerHTML = `
                    <div class="form-group">
                        <label for="workflow-tool-name">${esc(_t('workflows.config.mcpTool'))}</label>
                        <select id="workflow-tool-name" onchange="updateWorkflowTypedConfig()">
                            <option value="">${esc(_t('workflows.config.selectTool'))}</option>
                            ${workflowToolOptions.map(tool => `<option value="${esc(tool.key)}" ${tool.key === cfg.tool_name ? 'selected' : ''}>${esc(tool.key)}${tool.enabled ? '' : esc(_t('workflows.config.toolDisabled'))}</option>`).join('')}
                        </select>
                    </div>
                    ${typedTextarea('workflow-tool-arguments', _t('workflows.config.argumentsStatic'), cfg.arguments, '{"target":"example.com"}')}
                    ${typedField('workflow-tool-timeout', _t('workflows.config.timeoutSeconds'), cfg.timeout_seconds, _t('workflows.config.optional'))}
                `;
                if (!workflowToolsLoaded) {
                    loadWorkflowTools().then(() => {
                        if (selectedElement === ele) renderTypedConfig(ele);
                    });
                }
                break;
            case 'agent':
                wrap.innerHTML = `
                    <div class="form-group">
                        <label for="workflow-agent-mode">${esc(_t('workflows.config.agentMode'))}</label>
                        <select id="workflow-agent-mode" onchange="updateWorkflowTypedConfig()">
                            ${AGENT_MODES.map(mode => `<option value="${mode}" ${mode === cfg.agent_mode ? 'selected' : ''}>${mode}</option>`).join('')}
                        </select>
                    </div>
                    ${bindingFieldHtml('workflow-agent-input', 'workflows.config.inputBinding', bindingFromConfig(cfg, 'input_binding', 'previous', 'output'), 'workflows.config.inputBindingHint')}
                    ${typedTextarea('workflow-agent-instruction', _t('workflows.config.nodeInstruction'), cfg.instruction, _t('workflows.config.instructionPlaceholder'))}
                    ${typedField('workflow-agent-output-key', _t('workflows.config.outputKey'), cfg.output_key, 'agent_result')}
                `;
                break;
            case 'condition':
                wrap.innerHTML = `
                    ${typedField('workflow-condition-expression', _t('workflows.config.conditionExpression'), cfg.expression, '{{previous.output}} != ""')}
                    <p class="workflow-config-hint">${_t('workflows.config.conditionHint')}</p>
                `;
                break;
            case 'hitl':
                wrap.innerHTML = `
                    ${typedTextarea('workflow-hitl-prompt', _t('workflows.config.hitlPrompt'), cfg.prompt, _t('workflows.config.hitlPromptPlaceholder'))}
                    ${bindingFieldHtml('workflow-hitl-prompt-binding', 'workflows.config.promptBinding', bindingFromConfig(cfg, 'prompt_binding', 'previous', 'output'), 'workflows.config.promptBindingHint')}
                    <p class="workflow-config-hint">${_t('workflows.config.hitlInteractiveHint')}</p>
                    <div class="form-group">
                        <label for="workflow-hitl-reviewer">${esc(_t('workflows.config.hitlReviewer'))}</label>
                        <select id="workflow-hitl-reviewer" onchange="updateWorkflowTypedConfig()">
                            <option value="human" ${cfg.reviewer === 'human' ? 'selected' : ''}>human</option>
                            <option value="audit_agent" ${cfg.reviewer === 'audit_agent' ? 'selected' : ''}>audit_agent</option>
                        </select>
                    </div>
                `;
                break;
            case 'output':
                wrap.innerHTML = `
                    ${typedField('workflow-output-key', _t('workflows.config.outputKey'), cfg.output_key, 'result')}
                    ${bindingFieldHtml('workflow-output-source', 'workflows.config.sourceBinding', bindingFromConfig(cfg, 'source_binding', 'previous', 'output'), 'workflows.config.sourceBindingHint')}
                    ${typedField('workflow-output-static', _t('workflows.config.staticValue'), cfg.static_value || '', _t('workflows.config.optional'))}
                `;
                break;
            case 'end':
                wrap.innerHTML = bindingFieldHtml('workflow-end-result', 'workflows.config.resultBinding', bindingFromConfig(cfg, 'result_binding', 'outputs', 'result'), 'workflows.config.resultBindingHint');
                break;
            default:
                wrap.innerHTML = '';
        }
    }

    function renderCustomFields(config) {
        const wrap = document.getElementById('workflow-custom-fields');
        if (!wrap) return;
        const entries = Object.entries(config || {});
        if (!entries.length) {
            wrap.innerHTML = '<div class="workflow-property-empty workflow-property-empty--compact">' + esc(_t('workflows.noCustomFields')) + '</div>';
            return;
        }
        wrap.innerHTML = entries.map(([key, value], index) => `
            <div class="workflow-custom-field" data-index="${index}">
                <input type="text" value="${esc(key)}" data-field-key oninput="updateWorkflowCustomFields()">
                <input type="text" value="${esc(String(value == null ? '' : value))}" data-field-value oninput="updateWorkflowCustomFields()">
                <button type="button" onclick="removeWorkflowCustomField(${index})">×</button>
            </div>
        `).join('');
    }

    function readCustomFields() {
        const out = {};
        document.querySelectorAll('#workflow-custom-fields .workflow-custom-field').forEach(row => {
            const key = row.querySelector('[data-field-key]').value.trim();
            const value = row.querySelector('[data-field-value]').value;
            if (key) out[key] = value;
        });
        return out;
    }

    function readTypedConfig(ele) {
        if (!ele) return {};
        if (!ele.isNode()) {
            return { condition: (document.getElementById('workflow-edge-condition') || {}).value || '' };
        }
        const type = ele.data('type') || 'tool';
        switch (type) {
            case 'start':
                return { input_keys: (document.getElementById('workflow-start-input-keys') || {}).value || '' };
            case 'tool':
                return {
                    tool_name: (document.getElementById('workflow-tool-name') || {}).value || '',
                    arguments: (document.getElementById('workflow-tool-arguments') || {}).value || '{}',
                    timeout_seconds: (document.getElementById('workflow-tool-timeout') || {}).value || ''
                };
            case 'agent':
                return {
                    agent_mode: (document.getElementById('workflow-agent-mode') || {}).value || 'eino_single',
                    input_binding: readBinding('workflow-agent-input'),
                    instruction: (document.getElementById('workflow-agent-instruction') || {}).value || '',
                    output_key: (document.getElementById('workflow-agent-output-key') || {}).value || 'agent_result'
                };
            case 'condition':
                return { expression: (document.getElementById('workflow-condition-expression') || {}).value || '' };
            case 'hitl':
                return {
                    prompt: (document.getElementById('workflow-hitl-prompt') || {}).value || '',
                    prompt_binding: readBinding('workflow-hitl-prompt-binding'),
                    reviewer: (document.getElementById('workflow-hitl-reviewer') || {}).value || 'human'
                };
            case 'output':
                return {
                    output_key: (document.getElementById('workflow-output-key') || {}).value || 'result',
                    source_binding: readBinding('workflow-output-source'),
                    static_value: (document.getElementById('workflow-output-static') || {}).value || ''
                };
            case 'end':
                return { result_binding: readBinding('workflow-end-result') };
            default:
                return {};
        }
    }

    function mergeVisibleConfig() {
        if (!selectedElement) return;
        selectedElement.data('config', Object.assign({}, readCustomFields(), readTypedConfig(selectedElement)));
    }

    function handleConnectTap(node) {
        if (!connectSourceId) {
            connectSourceId = node.id();
            node.addClass('connect-source');
            return;
        }
        if (connectSourceId === node.id()) {
            clearConnectSource();
            return;
        }
        const duplicate = cy.edges().some(edge => edge.source().id() === connectSourceId && edge.target().id() === node.id());
        if (duplicate) {
            if (typeof showNotification === 'function') {
                showNotification(_t('workflows.duplicateEdge'), 'warning');
            }
            clearConnectSource();
            return;
        }
        const sourceNode = cy.getElementById(connectSourceId);
        const sourceType = sourceNode.data('type') || '';
        let edgeLabel = '';
        let edgeConfig = {};
        if (sourceType === 'condition') {
            const siblingCount = cy.edges().filter(edge => edge.source().id() === connectSourceId).length;
            if (siblingCount === 0) {
                edgeLabel = _t('workflows.edges.yes');
                edgeConfig = { condition: '{{previous.matched}} == "true"', branch: 'true' };
            } else if (siblingCount === 1) {
                edgeLabel = _t('workflows.edges.no');
                edgeConfig = { condition: '{{previous.matched}} == "false"', branch: 'false' };
            } else {
                edgeConfig = { condition: '' };
            }
        }
        cy.add({
            group: 'edges',
            data: {
                id: nextEdgeId(),
                source: connectSourceId,
                target: node.id(),
                label: edgeLabel,
                config: edgeConfig
            }
        });
        clearConnectSource();
    }

    function clearConnectSource() {
        if (cy) cy.nodes().removeClass('connect-source');
        connectSourceId = '';
    }

    function addNode(type, position) {
        initCy();
        if (!cy) return;
        const node = cy.add({
            group: 'nodes',
            data: {
                id: nextNodeId(type),
                type,
                label: wfNodeLabel(type),
                config: defaultConfigForType(type)
            },
            position: position || { x: 180 + cy.nodes().length * 28, y: 160 + cy.nodes().length * 28 }
        });
        selectWorkflowElement(node);
        updateEmptyState();
    }

    window.refreshWorkflows = async function () {
        initCy();
        const list = document.getElementById('workflow-list');
        if (list) list.innerHTML = '<div class="loading-spinner">' + esc(_t('common.loading')) + '</div>';
        try {
            await loadWorkflows(true);
            if (currentWorkflowId) {
                const wf = workflows.find(item => item.id === currentWorkflowId);
                if (wf) {
                    syncWorkflowMetaForm(wf);
                }
            } else if (workflows.length) {
                fillWorkflowForm(workflows[0]);
            } else {
                newWorkflowDraft({ openMeta: false });
                return;
            }
            renderWorkflowList();
        } catch (error) {
            if (list) list.innerHTML = `<div class="empty-state">${esc(error.message)}</div>`;
            if (typeof showNotification === 'function') showNotification(error.message, 'error');
        }
    };

    window.newWorkflowDraft = function (options) {
        const shouldOpenMeta = !options || options.openMeta !== false;
        currentWorkflowId = '';
        fillWorkflowForm({
            id: '',
            name: '',
            description: '',
            enabled: true,
            graph_json: defaultGraph()
        });
        syncWorkflowMetaIdField(false, '');
        if (shouldOpenMeta) {
            openWorkflowMetaModal();
        }
    };

    window.selectWorkflow = function (id) {
        const wf = workflows.find(item => item.id === id);
        if (wf) fillWorkflowForm(wf);
    };

    window.openWorkflowMetaModal = function () {
        const nameEl = document.getElementById('workflow-name');
        const idEl = document.getElementById('workflow-id');
        if (currentWorkflowId) {
            syncWorkflowMetaIdField(true, currentWorkflowId);
        } else {
            syncWorkflowMetaIdField(false, idEl ? idEl.value.trim() : '');
        }
        if (typeof openAppModal === 'function') {
            openAppModal('workflow-meta-modal', {
                focusEl: currentWorkflowId ? nameEl : (idEl && !idEl.hidden ? idEl : nameEl)
            });
        }
    };

    window.closeWorkflowMetaModal = function () {
        if (typeof closeAppModal === 'function') {
            closeAppModal('workflow-meta-modal');
        }
    };

    window.applyWorkflowMetaModal = function () {
        const meta = readWorkflowMetaFromForm();
        if (!meta.id || !meta.name) {
            if (typeof showNotification === 'function') {
                showNotification(_t('workflows.idNameRequired'), 'error');
            }
            return;
        }
        updateWorkflowCanvasTitle();
        renderWorkflowList();
        closeWorkflowMetaModal();
    };

    window.editWorkflowFromList = function (id) {
        if (id !== currentWorkflowId) {
            selectWorkflow(id);
        }
        openWorkflowMetaModal();
    };

    window.toggleWorkflowEnabled = async function (id, enabled) {
        const wf = workflows.find(item => item.id === id);
        if (!wf) return;
        const previous = wf.enabled !== false;
        wf.enabled = enabled;
        if (id === currentWorkflowId) {
            const enabledEl = document.getElementById('workflow-enabled');
            if (enabledEl) enabledEl.checked = enabled;
            updateWorkflowCanvasTitle();
        }
        renderWorkflowList();
        let graph = defaultGraph();
        if (id === currentWorkflowId && cy) {
            graph = elementsToGraph();
        } else {
            graph = parseGraph(wf.graph_json || wf.graph || defaultGraph());
        }
        try {
            const response = await apiFetch(`/api/workflows/${encodeURIComponent(id)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    id: wf.id,
                    name: wf.name,
                    description: wf.description || '',
                    enabled,
                    graph
                })
            });
            if (!response.ok) {
                const err = await response.json().catch(() => ({}));
                throw new Error(err.error || _t('workflows.enabledUpdateFailed'));
            }
            if (typeof showNotification === 'function') {
                showNotification(_t('workflows.enabledUpdated'), 'success');
            }
            if (typeof loadWorkflowOptionsForRoleModal === 'function') {
                await loadWorkflowOptionsForRoleModal();
            }
        } catch (error) {
            wf.enabled = previous;
            if (id === currentWorkflowId) {
                const enabledEl = document.getElementById('workflow-enabled');
                if (enabledEl) enabledEl.checked = previous;
                updateWorkflowCanvasTitle();
            }
            renderWorkflowList();
            if (typeof showNotification === 'function') {
                showNotification(error.message || _t('workflows.enabledUpdateFailed'), 'error');
            }
        }
    };

    function validateWorkflowGraph(graph) {
        const errors = [];
        const nodes = graph.nodes || [];
        const edges = graph.edges || [];
        const ids = new Set(nodes.map(node => node.id));
        const starts = nodes.filter(node => node.type === 'start');
        const outputs = nodes.filter(node => node.type === 'output');
        if (!starts.length) errors.push(_t('workflows.validation.needStart'));
        if (!outputs.length) errors.push(_t('workflows.validation.needOutput'));
        edges.forEach(edge => {
            if (edge.source === edge.target) errors.push(_t('workflows.validation.edgeSelfLoop', { id: edge.id }));
            if (!ids.has(edge.source)) errors.push(_t('workflows.validation.edgeSourceMissing', { id: edge.id }));
            if (!ids.has(edge.target)) errors.push(_t('workflows.validation.edgeTargetMissing', { id: edge.id }));
        });
        starts.forEach(node => {
            if (edges.some(edge => edge.target === node.id)) errors.push(_t('workflows.validation.startIncoming', { label: node.label || node.id }));
        });
        outputs.forEach(node => {
            if (edges.some(edge => edge.source === node.id)) errors.push(_t('workflows.validation.outputOutgoing', { label: node.label || node.id }));
        });
        nodes.filter(node => node.type === 'tool').forEach(node => {
            if (!String((node.config || {}).tool_name || '').trim()) {
                errors.push(_t('workflows.validation.toolNeedsMcp', { label: node.label || node.id }));
            }
        });
        nodes.filter(node => node.type === 'condition').forEach(node => {
            if (!String((node.config || {}).expression || '').trim()) {
                errors.push(_t('workflows.validation.conditionNeedsExpr', { label: node.label || node.id }));
            }
            const outEdges = edges.filter(edge => edge.source === node.id);
            if (outEdges.length === 0) {
                errors.push(_t('workflows.validation.conditionNeedsOutEdge', { label: node.label || node.id }));
            } else if (outEdges.length > 2) {
                errors.push(_t('workflows.validation.conditionTooManyEdges', { label: node.label || node.id }));
            }
        });
        nodes.filter(node => node.type === 'output').forEach(node => {
            if (!String((node.config || {}).output_key || '').trim()) {
                errors.push(_t('workflows.validation.outputNeedsKey', { label: node.label || node.id }));
            }
        });
        return errors;
    }

    window.saveWorkflowDraft = async function () {
        initCy();
        const meta = readWorkflowMetaFromForm();
        if (!meta.id || !meta.name) {
            if (typeof showNotification === 'function') {
                showNotification(_t('workflows.idNameRequired'), 'error');
            }
            openWorkflowMetaModal();
            return;
        }
        const graph = elementsToGraph();
        const errors = validateWorkflowGraph(graph);
        if (errors.length) {
            showNotification(errors.slice(0, 4).join('；'), 'error');
            return;
        }
        const method = currentWorkflowId ? 'PUT' : 'POST';
        const url = currentWorkflowId ? `/api/workflows/${encodeURIComponent(currentWorkflowId)}` : '/api/workflows';
        const response = await apiFetch(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                id: meta.id,
                name: meta.name,
                description: meta.description,
                enabled: meta.enabled,
                graph
            })
        });
        if (!response.ok) {
            const err = await response.json().catch(() => ({}));
            showNotification(err.error || _t('workflows.saveFailed'), 'error');
            return;
        }
        const data = await response.json();
        currentWorkflowId = data.workflow && data.workflow.id ? data.workflow.id : meta.id;
        syncWorkflowMetaIdField(true, currentWorkflowId);
        closeWorkflowMetaModal();
        showNotification(_t('workflows.saved'), 'success');
        await refreshWorkflows();
        if (typeof loadWorkflowOptionsForRoleModal === 'function') {
            await loadWorkflowOptionsForRoleModal();
        }
    };

    window.deleteCurrentWorkflow = async function () {
        const meta = readWorkflowMetaFromForm();
        const id = currentWorkflowId || meta.id;
        if (!id) {
            showNotification(_t('workflows.selectToDelete'), 'warning');
            return;
        }
        if (!confirm(_t('workflows.confirmDelete', { id: id }))) return;
        const response = await apiFetch(`/api/workflows/${encodeURIComponent(id)}`, { method: 'DELETE' });
        if (!response.ok) {
            const err = await response.json().catch(() => ({}));
            showNotification(err.error || _t('workflows.deleteFailed'), 'error');
            return;
        }
        currentWorkflowId = '';
        showNotification(_t('workflows.deleted'), 'success');
        newWorkflowDraft({ openMeta: false });
        await refreshWorkflows();
    };

    window.workflowPaletteDragStart = function (event) {
        const type = event.currentTarget.dataset.nodeType || 'tool';
        event.dataTransfer.setData('application/x-workflow-node', type);
        event.dataTransfer.setData('text/plain', type);
        event.dataTransfer.effectAllowed = 'copy';
    };

    window.workflowCanvasDragOver = function (event) {
        event.preventDefault();
        event.dataTransfer.dropEffect = 'copy';
    };

    window.workflowCanvasDrop = function (event) {
        event.preventDefault();
        const type = event.dataTransfer.getData('application/x-workflow-node') || event.dataTransfer.getData('text/plain') || 'tool';
        const rect = document.getElementById('workflow-canvas').getBoundingClientRect();
        const pan = cy.pan();
        const zoom = cy.zoom();
        addNode(type, {
            x: (event.clientX - rect.left - pan.x) / zoom,
            y: (event.clientY - rect.top - pan.y) / zoom
        });
    };

    window.addWorkflowNodeFromPalette = function (type) {
        addNode(type || 'tool');
    };

    window.toggleWorkflowConnectMode = function () {
        connectMode = !connectMode;
        clearConnectSource();
        const btn = document.getElementById('workflow-connect-btn');
        if (btn) {
            btn.classList.toggle('active', connectMode);
            btn.textContent = connectMode ? _t('workflows.connecting') : _t('workflows.connect');
        }
        if (typeof showNotification === 'function') {
            showNotification(connectMode ? _t('workflows.connectModeOn') : _t('workflows.connectModeOff'), 'info');
        }
    };

    window.deleteWorkflowSelection = function () {
        if (!cy) return;
        const selected = selectedElement && selectedElement.length ? selectedElement : cy.$(':selected');
        if (!selected.length) return;
        selected.remove();
        selectWorkflowElement(null);
        updateEmptyState();
    };

    window.layoutWorkflowGraph = function (animate) {
        if (!cy || !cy.nodes().length) return;
        cy.layout({
            name: 'breadthfirst',
            directed: true,
            padding: 40,
            spacingFactor: 1.25,
            animate: animate !== false,
            animationDuration: 250
        }).run();
        cy.fit(undefined, 40);
    };

    window.updateWorkflowSelectedProperty = function () {
        if (!selectedElement) return;
        const label = document.getElementById('workflow-prop-label').value.trim();
        selectedElement.data('label', label);
        if (selectedElement.isNode()) {
            const type = document.getElementById('workflow-prop-type').value || 'tool';
            const prevType = selectedElement.data('type') || 'tool';
            selectedElement.data('type', type);
            if (type !== prevType) {
                selectedElement.data('config', defaultConfigForType(type));
                selectedElement.data('label', label || wfNodeLabel(type));
                document.getElementById('workflow-prop-label').value = selectedElement.data('label') || '';
                renderTypedConfig(selectedElement);
                renderCustomFields({});
            }
        }
    };

    window.addWorkflowCustomField = function () {
        if (!selectedElement) return;
        const cfg = Object.assign({}, selectedElement.data('config') || {});
        let i = 1;
        while (Object.prototype.hasOwnProperty.call(cfg, `field_${i}`)) i += 1;
        cfg[`field_${i}`] = '';
        selectedElement.data('config', cfg);
        renderCustomFields(cfg);
    };

    window.updateWorkflowCustomFields = function () {
        if (!selectedElement) return;
        mergeVisibleConfig();
    };

    window.updateWorkflowTypedConfig = function () {
        if (!selectedElement) return;
        mergeVisibleConfig();
    };

    window.removeWorkflowCustomField = function (index) {
        if (!selectedElement) return;
        const entries = Object.entries(stripTypedConfig(selectedElement));
        entries.splice(index, 1);
        const next = {};
        entries.forEach(([key, value]) => {
            if (key) next[key] = value;
        });
        selectedElement.data('config', Object.assign({}, next, readTypedConfig(selectedElement)));
        renderCustomFields(next);
    };

    window.loadWorkflowOptionsForRoleModal = async function (selectedId) {
        try {
            await loadWorkflows(true);
        } catch (_) {
            workflows = [];
        }
        const select = document.getElementById('role-workflow-id');
        if (!select) return;
        const current = selectedId !== undefined ? selectedId : select.value;
        select.innerHTML = '<option value="">' + esc(_t('roleModal.noWorkflowBind')) + '</option>' + workflows.map(wf => (
            `<option value="${esc(wf.id)}">${esc(wf.name || wf.id)}${wf.enabled ? '' : esc(_t('roleModal.workflowDisabledSuffix'))}</option>`
        )).join('');
        select.value = current || '';
    };

    function refreshCanvasLabels() {
        if (!cy) return;
        cy.nodes().forEach(function (node) {
            const type = node.data('type') || 'tool';
            const label = node.data('label') || '';
            const known = KNOWN_NODE_LABELS[type] || [];
            if (known.indexOf(label) !== -1) {
                node.data('label', wfNodeLabel(type));
            }
        });
        cy.edges().forEach(function (edge) {
            const label = edge.data('label') || '';
            if (KNOWN_EDGE_LABELS.yes.indexOf(label) !== -1) {
                edge.data('label', _t('workflows.edges.yes'));
            } else if (KNOWN_EDGE_LABELS.no.indexOf(label) !== -1) {
                edge.data('label', _t('workflows.edges.no'));
            }
        });
    }

    function refreshWorkflowsI18n() {
        const page = document.getElementById('page-workflows');
        if (page && typeof window.applyTranslations === 'function') {
            window.applyTranslations(page);
        }
        const connectBtn = document.getElementById('workflow-connect-btn');
        if (connectBtn) {
            connectBtn.textContent = connectMode ? _t('workflows.connecting') : _t('workflows.connect');
        }
        refreshCanvasLabels();
        updateWorkflowCanvasTitle();
        renderWorkflowList();
        if (selectedElement && selectedElement.length) {
            selectWorkflowElement(selectedElement);
        } else {
            selectWorkflowElement(null);
        }
        if (typeof loadWorkflowOptionsForRoleModal === 'function') {
            loadWorkflowOptionsForRoleModal();
        }
    }

    document.addEventListener('languagechange', function () {
        refreshWorkflowsI18n();
    });
})();
