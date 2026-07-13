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
    const WORKFLOW_TOOL_SELECT_ID = 'workflow-tool-name';
    let workflowToolSelectRegistry = null;
    let workflowToolSelectDocBound = false;
    const workflowPackageState = {
        file: null,
        inspection: null,
        importRecord: null,
        resolutionAction: '',
        newWorkflowId: '',
        riskChoicesVisible: false,
        idempotencyKey: '',
        requestSignature: ''
    };
    const WORKFLOW_PACKAGE_INSPECTION_STORAGE_KEY = 'csai.workflow-package.inspection-id';
    const WORKFLOW_PACKAGE_IMPORT_STORAGE_KEY = 'csai.workflow-package.import-id';

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
    const JOIN_STRATEGIES = ['all_merge', 'last_by_canvas', 'first_non_empty', 'fail_fast'];
    const NODE_DEFAULT_SIZE = { w: 150, h: 52 };
    const NODE_TYPE_SIZES = { condition: { w: 118, h: 86 } };
    const NODE_PLACEMENT_GAP = 48;
    const NODE_PLACEMENT_PADDING = 20;

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
                    <select id="${prefix}-from" class="workflow-form-select-native" onchange="updateWorkflowTypedConfig()" style="flex:1;">${options}</select>
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
                return { tool_name: '', arguments: '{}', timeout_seconds: '', join_strategy: 'all_merge' };
            case 'agent':
                return { agent_mode: 'eino_single', input_binding: { from: 'previous', field: 'output' }, instruction: '', output_key: 'agent_result', join_strategy: 'all_merge' };
            case 'condition':
                return { expression: '{{previous.output}} != ""', join_strategy: 'all_merge' };
            case 'hitl':
                return { prompt: _t('workflows.defaultHitlPrompt'), prompt_binding: { from: 'previous', field: 'output' }, reviewer: 'human', join_strategy: 'all_merge' };
            case 'output':
                return { output_key: 'result', source_binding: { from: 'previous', field: 'output' }, join_strategy: 'all_merge' };
            case 'end':
                return { result_binding: { from: 'outputs', field: 'result' }, join_strategy: 'all_merge' };
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
                },
                {
                    selector: 'node.just-added',
                    style: {
                        'border-width': 4,
                        'border-color': '#fbbf24',
                        'border-opacity': 1,
                        'z-index': 999
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

    function workflowToolOptionLabel(tool) {
        return tool.key + (tool.enabled ? '' : _t('workflows.config.toolDisabled'));
    }

    function closeWorkflowToolSelect() {
        const reg = workflowToolSelectRegistry;
        if (!reg || !reg.wrapper) return;
        reg.wrapper.classList.remove('open');
        if (reg.trigger) reg.trigger.setAttribute('aria-expanded', 'false');
        if (reg.searchInput) reg.searchInput.value = '';
    }

    function createWorkflowToolOptionButton(value, label, selectedValue) {
        const item = document.createElement('button');
        item.type = 'button';
        item.className = 'workflow-tool-select-option';
        item.setAttribute('role', 'option');
        item.setAttribute('data-value', value);
        item.title = label;
        if (value === selectedValue) {
            item.classList.add('is-selected');
            item.setAttribute('aria-selected', 'true');
        } else {
            item.setAttribute('aria-selected', 'false');
        }
        const check = document.createElement('span');
        check.className = 'workflow-tool-select-check';
        check.setAttribute('aria-hidden', 'true');
        check.textContent = '✓';
        const labelEl = document.createElement('span');
        labelEl.className = 'workflow-tool-select-label';
        labelEl.textContent = label;
        labelEl.title = label;
        item.appendChild(check);
        item.appendChild(labelEl);
        return item;
    }

    function renderWorkflowToolSelectOptions(reg, query) {
        const { select, optionsList } = reg;
        optionsList.innerHTML = '';
        const q = (query || '').trim().toLowerCase();
        let matchCount = 0;

        Array.prototype.forEach.call(select.options, (opt) => {
            if (opt.value === '') {
                if (!q) {
                    optionsList.appendChild(createWorkflowToolOptionButton(opt.value, opt.textContent || '', select.value));
                }
                return;
            }
            const label = opt.textContent || opt.value;
            if (q && !label.toLowerCase().includes(q) && !opt.value.toLowerCase().includes(q)) return;
            matchCount += 1;
            optionsList.appendChild(createWorkflowToolOptionButton(opt.value, label, select.value));
        });

        if (matchCount === 0) {
            const empty = document.createElement('div');
            empty.className = 'workflow-tool-select-empty';
            empty.textContent = q
                ? _t('workflows.config.noToolsFound')
                : _t('workflows.config.noToolsAvailable');
            optionsList.appendChild(empty);
        }
    }

    function ensureWorkflowToolSearchUi(reg) {
        if (reg.searchInput && reg.optionsList) return;
        const { dropdown } = reg;
        dropdown.innerHTML = '';

        const searchWrap = document.createElement('div');
        searchWrap.className = 'workflow-tool-select-search';
        const searchInput = document.createElement('input');
        searchInput.type = 'search';
        searchInput.className = 'workflow-tool-select-search-input';
        searchInput.setAttribute('autocomplete', 'off');
        searchInput.setAttribute('data-i18n', 'workflows.config.searchTool');
        searchInput.setAttribute('data-i18n-attr', 'placeholder');
        searchInput.placeholder = _t('workflows.config.searchTool');
        searchWrap.appendChild(searchInput);
        dropdown.appendChild(searchWrap);
        reg.searchInput = searchInput;

        const optionsList = document.createElement('div');
        optionsList.className = 'workflow-tool-select-options';
        dropdown.appendChild(optionsList);
        reg.optionsList = optionsList;

        searchInput.addEventListener('input', () => renderWorkflowToolSelectOptions(reg, searchInput.value));
        searchInput.addEventListener('click', (e) => e.stopPropagation());
        searchInput.addEventListener('keydown', (e) => {
            e.stopPropagation();
            if (e.key === 'Escape') closeWorkflowToolSelect();
        });
    }

    function syncWorkflowToolSelect() {
        const select = document.getElementById(WORKFLOW_TOOL_SELECT_ID);
        const reg = workflowToolSelectRegistry;
        if (!select || !reg || reg.select !== select) return;
        const selected = select.options[select.selectedIndex];
        reg.valueSpan.textContent = selected && selected.value
            ? selected.textContent
            : _t('workflows.config.selectTool');
        if (reg.optionsList) {
            renderWorkflowToolSelectOptions(reg, reg.searchInput ? reg.searchInput.value : '');
        }
        reg.trigger.disabled = !!select.disabled;
        reg.wrapper.classList.toggle('is-disabled', !!select.disabled);
    }

    function enhanceWorkflowToolSelect() {
        const select = document.getElementById(WORKFLOW_TOOL_SELECT_ID);
        if (!select) {
            workflowToolSelectRegistry = null;
            return;
        }
        if (select.dataset.workflowToolCustom === '1' && workflowToolSelectRegistry && workflowToolSelectRegistry.select === select) {
            syncWorkflowToolSelect();
            return;
        }
        workflowToolSelectRegistry = null;

        select.dataset.workflowToolCustom = '1';
        select.classList.add('workflow-tool-native-select');
        select.tabIndex = -1;
        select.setAttribute('aria-hidden', 'true');

        const wrapper = document.createElement('div');
        wrapper.className = 'workflow-tool-select';

        const trigger = document.createElement('button');
        trigger.type = 'button';
        trigger.className = 'workflow-tool-select-trigger';
        trigger.setAttribute('aria-haspopup', 'listbox');
        trigger.setAttribute('aria-expanded', 'false');
        const valueSpan = document.createElement('span');
        valueSpan.className = 'workflow-tool-select-value';
        trigger.appendChild(valueSpan);
        const caret = document.createElement('span');
        caret.className = 'workflow-tool-select-caret';
        caret.setAttribute('aria-hidden', 'true');
        caret.textContent = '▾';
        trigger.appendChild(caret);

        const dropdown = document.createElement('div');
        dropdown.className = 'workflow-tool-select-dropdown';
        dropdown.setAttribute('role', 'listbox');

        const parent = select.parentNode;
        parent.insertBefore(wrapper, select);
        wrapper.appendChild(trigger);
        wrapper.appendChild(dropdown);
        wrapper.appendChild(select);

        workflowToolSelectRegistry = {
            wrapper,
            trigger,
            dropdown,
            select,
            valueSpan,
            searchInput: null,
            optionsList: null
        };

        trigger.addEventListener('click', (e) => {
            e.stopPropagation();
            if (select.disabled) return;
            const open = wrapper.classList.contains('open');
            closeWorkflowToolSelect();
            closeAllWorkflowFormSelects();
            if (!open) {
                wrapper.classList.add('open');
                trigger.setAttribute('aria-expanded', 'true');
                ensureWorkflowToolSearchUi(workflowToolSelectRegistry);
                if (workflowToolSelectRegistry.searchInput) {
                    workflowToolSelectRegistry.searchInput.value = '';
                    renderWorkflowToolSelectOptions(workflowToolSelectRegistry, '');
                    requestAnimationFrame(() => workflowToolSelectRegistry.searchInput.focus());
                }
            }
        });

        dropdown.addEventListener('click', (e) => {
            const opt = e.target.closest('.workflow-tool-select-option');
            if (!opt) return;
            e.stopPropagation();
            const val = opt.getAttribute('data-value');
            if (val === null) return;
            if (select.value !== val) {
                select.value = val;
                select.dispatchEvent(new Event('change', { bubbles: true }));
            }
            closeWorkflowToolSelect();
            syncWorkflowToolSelect();
        });

        select.addEventListener('change', () => syncWorkflowToolSelect());

        if (!workflowToolSelectDocBound) {
            workflowToolSelectDocBound = true;
            document.addEventListener('click', closeWorkflowToolSelect);
            document.addEventListener('keydown', (e) => {
                if (e.key === 'Escape') closeWorkflowToolSelect();
            });
        }

        syncWorkflowToolSelect();
    }

    const workflowFormSelectMap = {};
    let workflowFormSelectDocBound = false;
    const WORKFLOW_FORM_SELECT_CARET = '<svg class="workflow-form-select-caret" width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>';

    function closeAllWorkflowFormSelects() {
        Object.keys(workflowFormSelectMap).forEach(function (id) {
            const reg = workflowFormSelectMap[id];
            if (!reg || !reg.wrapper) return;
            reg.wrapper.classList.remove('open');
            if (reg.trigger) reg.trigger.setAttribute('aria-expanded', 'false');
        });
    }

    function pruneWorkflowFormSelectMap(root) {
        Object.keys(workflowFormSelectMap).forEach(function (id) {
            const select = document.getElementById(id);
            if (!select || (root && !root.contains(select))) {
                delete workflowFormSelectMap[id];
            }
        });
    }

    function syncWorkflowFormSelect(select) {
        const reg = workflowFormSelectMap[select.id];
        if (!reg) return;
        const dropdown = reg.dropdown;
        const trigger = reg.trigger;
        const valueSpan = trigger.querySelector('.workflow-form-select-value');

        dropdown.innerHTML = '';
        Array.prototype.forEach.call(select.options, function (opt) {
            const item = document.createElement('button');
            item.type = 'button';
            item.className = 'workflow-form-select-option';
            item.setAttribute('role', 'option');
            item.setAttribute('data-value', opt.value);
            if (opt.value === select.value) {
                item.classList.add('is-selected');
                item.setAttribute('aria-selected', 'true');
            } else {
                item.setAttribute('aria-selected', 'false');
            }
            const check = document.createElement('span');
            check.className = 'workflow-form-select-check';
            check.setAttribute('aria-hidden', 'true');
            check.textContent = '✓';
            const label = document.createElement('span');
            label.className = 'workflow-form-select-label';
            label.textContent = opt.textContent;
            item.appendChild(check);
            item.appendChild(label);
            dropdown.appendChild(item);
        });

        const selectedOpt = select.options[select.selectedIndex];
        if (valueSpan) {
            valueSpan.textContent = selectedOpt ? selectedOpt.textContent : '';
        }
        trigger.disabled = !!select.disabled;
        reg.wrapper.classList.toggle('is-disabled', !!select.disabled);
    }

    function enhanceWorkflowFormSelect(select) {
        if (!select || !select.id) return;
        if (select.id === WORKFLOW_TOOL_SELECT_ID) return;
        const existing = workflowFormSelectMap[select.id];
        if (existing && existing.select !== select) {
            delete workflowFormSelectMap[select.id];
        }
        if (select.dataset.workflowFormCustom === '1') {
            syncWorkflowFormSelect(select);
            return;
        }
        select.dataset.workflowFormCustom = '1';
        select.classList.add('workflow-form-native-select');
        select.tabIndex = -1;
        select.setAttribute('aria-hidden', 'true');

        const wrapper = document.createElement('div');
        wrapper.className = 'workflow-form-select-ui';

        const trigger = document.createElement('button');
        trigger.type = 'button';
        trigger.className = 'workflow-form-select-trigger';
        trigger.setAttribute('aria-haspopup', 'listbox');
        trigger.setAttribute('aria-expanded', 'false');
        const valueSpan = document.createElement('span');
        valueSpan.className = 'workflow-form-select-value';
        trigger.appendChild(valueSpan);
        trigger.insertAdjacentHTML('beforeend', WORKFLOW_FORM_SELECT_CARET);

        const dropdown = document.createElement('div');
        dropdown.className = 'workflow-form-select-dropdown';
        dropdown.setAttribute('role', 'listbox');

        const parent = select.parentNode;
        parent.insertBefore(wrapper, select);
        wrapper.appendChild(trigger);
        wrapper.appendChild(dropdown);
        wrapper.appendChild(select);

        workflowFormSelectMap[select.id] = { wrapper: wrapper, trigger: trigger, dropdown: dropdown, select: select };

        trigger.addEventListener('click', function (e) {
            e.stopPropagation();
            if (select.disabled) return;
            const open = wrapper.classList.contains('open');
            closeAllWorkflowFormSelects();
            closeWorkflowToolSelect();
            if (!open) {
                wrapper.classList.add('open');
                trigger.setAttribute('aria-expanded', 'true');
            }
        });

        dropdown.addEventListener('click', function (e) {
            const opt = e.target.closest('.workflow-form-select-option');
            if (!opt) return;
            e.stopPropagation();
            const val = opt.getAttribute('data-value');
            if (val === null) return;
            if (select.value !== val) {
                select.value = val;
                select.dispatchEvent(new Event('change', { bubbles: true }));
            }
            wrapper.classList.remove('open');
            trigger.setAttribute('aria-expanded', 'false');
            syncWorkflowFormSelect(select);
        });

        select.addEventListener('change', function () {
            syncWorkflowFormSelect(select);
        });

        syncWorkflowFormSelect(select);
    }

    function refreshWorkflowPropertySelects() {
        const form = document.getElementById('workflow-property-form');
        if (!form || form.hidden) return;
        pruneWorkflowFormSelectMap(form);
        form.querySelectorAll('select').forEach(function (select) {
            if (select.id === WORKFLOW_TOOL_SELECT_ID) return;
            enhanceWorkflowFormSelect(select);
        });
        if (!workflowFormSelectDocBound) {
            workflowFormSelectDocBound = true;
            document.addEventListener('click', closeAllWorkflowFormSelects);
            document.addEventListener('keydown', function (e) {
                if (e.key === 'Escape') closeAllWorkflowFormSelects();
            });
        }
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
        closeWorkflowDryRunPanel();
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

    function joinStrategyHtml(cfg) {
        const selected = cfg.join_strategy || 'all_merge';
        return `
            <div class="form-group">
                <label for="workflow-join-strategy">${esc(_t('workflows.config.joinStrategy') || '汇聚策略')}</label>
                <select id="workflow-join-strategy" class="workflow-form-select-native" onchange="updateWorkflowTypedConfig()">
                    ${JOIN_STRATEGIES.map(strategy => `<option value="${strategy}" ${strategy === selected ? 'selected' : ''}>${strategy}</option>`).join('')}
                </select>
                <p class="workflow-config-hint">${esc(_t('workflows.config.joinStrategyHint') || '多个上游进入同一节点时如何生成 previous。')}</p>
            </div>
        `;
    }

    function conditionExpressionGuideHtml() {
        const examples = [
            '{{previous.output}} != ""',
            '{{outputs.risk_score}} >= 8',
            '{{previous.output}} contains "success"',
            '{{previous.output}} matches "^ok"',
            'jsonpath({{previous.output}}, "$.status") == "ok"',
            'jq({{outputs.scan}}, ".severity") == "high"'
        ];
        return `
            <div class="workflow-config-hint workflow-condition-guide">
                <div><strong>${esc(_t('workflows.config.conditionGuideTitle'))}</strong></div>
                <div>${esc(_t('workflows.config.conditionGuideVars'))}</div>
                <div>${esc(_t('workflows.config.conditionGuideOps'))}</div>
                <div>${esc(_t('workflows.config.conditionGuideJson'))}</div>
                <div class="workflow-example-chips">
                    ${examples.map(expr => `<button type="button" class="btn-secondary btn-small" onclick="useWorkflowConditionExample(this.dataset.expression)" data-expression="${esc(expr)}">${esc(expr)}</button>`).join('')}
                </div>
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
                ${sourceType === 'condition' ? `
                    <div class="form-group">
                        <label for="workflow-edge-branch">${esc(_t('workflows.config.edgeBranch') || '条件分支')}</label>
                        <select id="workflow-edge-branch" class="workflow-form-select-native" onchange="updateWorkflowTypedConfig()">
                            <option value="">${esc(_t('workflows.config.selectBranch') || '请选择')}</option>
                            <option value="true" ${cfg.branch === 'true' ? 'selected' : ''}>true / 是</option>
                            <option value="false" ${cfg.branch === 'false' ? 'selected' : ''}>false / 否</option>
                        </select>
                    </div>
                    <p class="workflow-config-hint">${esc(_t('workflows.config.edgeBranchHint'))}</p>
                ` : ''}
            `;
            refreshWorkflowPropertySelects();
            return;
        }
        const type = ele.data('type') || 'tool';
        switch (type) {
            case 'start':
                wrap.innerHTML = typedField('workflow-start-input-keys', _t('workflows.config.inputKeys'), cfg.input_keys, 'message, projectId');
                break;
            case 'tool':
                wrap.innerHTML = `
                    ${joinStrategyHtml(cfg)}
                    <div class="form-group">
                        <label>${esc(_t('workflows.config.mcpTool'))}</label>
                        <select id="workflow-tool-name" onchange="updateWorkflowTypedConfig()">
                            <option value="">${esc(_t('workflows.config.selectTool'))}</option>
                            ${workflowToolOptions.map(tool => `<option value="${esc(tool.key)}" ${tool.key === cfg.tool_name ? 'selected' : ''}>${esc(tool.key)}${tool.enabled ? '' : esc(_t('workflows.config.toolDisabled'))}</option>`).join('')}
                        </select>
                    </div>
                    ${typedTextarea('workflow-tool-arguments', _t('workflows.config.argumentsStatic'), cfg.arguments, '{"target":"example.com"}')}
                    ${typedField('workflow-tool-timeout', _t('workflows.config.timeoutSeconds'), cfg.timeout_seconds, _t('workflows.config.optional'))}
                `;
                enhanceWorkflowToolSelect();
                if (!workflowToolsLoaded) {
                    loadWorkflowTools().then(() => {
                        if (selectedElement === ele) renderTypedConfig(ele);
                    });
                }
                break;
            case 'agent':
                wrap.innerHTML = `
                    ${joinStrategyHtml(cfg)}
                    <div class="form-group">
                        <label for="workflow-agent-mode">${esc(_t('workflows.config.agentMode'))}</label>
                        <select id="workflow-agent-mode" class="workflow-form-select-native" onchange="updateWorkflowTypedConfig()">
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
                    ${joinStrategyHtml(cfg)}
                    ${typedField('workflow-condition-expression', _t('workflows.config.conditionExpression'), cfg.expression, '{{previous.output}} != ""')}
                    <p class="workflow-config-hint">${_t('workflows.config.conditionHint')}</p>
                    ${conditionExpressionGuideHtml()}
                `;
                break;
            case 'hitl':
                wrap.innerHTML = `
                    ${joinStrategyHtml(cfg)}
                    ${typedTextarea('workflow-hitl-prompt', _t('workflows.config.hitlPrompt'), cfg.prompt, _t('workflows.config.hitlPromptPlaceholder'))}
                    ${bindingFieldHtml('workflow-hitl-prompt-binding', 'workflows.config.promptBinding', bindingFromConfig(cfg, 'prompt_binding', 'previous', 'output'), 'workflows.config.promptBindingHint')}
                    <p class="workflow-config-hint">${_t('workflows.config.hitlInteractiveHint')}</p>
                    <div class="form-group">
                        <label for="workflow-hitl-reviewer">${esc(_t('workflows.config.hitlReviewer'))}</label>
                        <select id="workflow-hitl-reviewer" class="workflow-form-select-native" onchange="updateWorkflowTypedConfig()">
                            <option value="human" ${cfg.reviewer === 'human' ? 'selected' : ''}>human</option>
                            <option value="audit_agent" ${cfg.reviewer === 'audit_agent' ? 'selected' : ''}>audit_agent</option>
                        </select>
                    </div>
                `;
                break;
            case 'output':
                wrap.innerHTML = `
                    ${joinStrategyHtml(cfg)}
                    ${typedField('workflow-output-key', _t('workflows.config.outputKey'), cfg.output_key, 'result')}
                    ${bindingFieldHtml('workflow-output-source', 'workflows.config.sourceBinding', bindingFromConfig(cfg, 'source_binding', 'previous', 'output'), 'workflows.config.sourceBindingHint')}
                    ${typedField('workflow-output-static', _t('workflows.config.staticValue'), cfg.static_value || '', _t('workflows.config.optional'))}
                `;
                break;
            case 'end':
                wrap.innerHTML = joinStrategyHtml(cfg) + bindingFieldHtml('workflow-end-result', 'workflows.config.resultBinding', bindingFromConfig(cfg, 'result_binding', 'outputs', 'result'), 'workflows.config.resultBindingHint');
                break;
            default:
                wrap.innerHTML = '';
        }
        refreshWorkflowPropertySelects();
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
            const cfg = { condition: (document.getElementById('workflow-edge-condition') || {}).value || '' };
            const branchEl = document.getElementById('workflow-edge-branch');
            if (branchEl) cfg.branch = branchEl.value || '';
            return cfg;
        }
        const type = ele.data('type') || 'tool';
        const join_strategy = (document.getElementById('workflow-join-strategy') || {}).value || 'all_merge';
        switch (type) {
            case 'start':
                return { input_keys: (document.getElementById('workflow-start-input-keys') || {}).value || '' };
            case 'tool':
                return {
                    tool_name: (document.getElementById('workflow-tool-name') || {}).value || '',
                    arguments: (document.getElementById('workflow-tool-arguments') || {}).value || '{}',
                    timeout_seconds: (document.getElementById('workflow-tool-timeout') || {}).value || '',
                    join_strategy
                };
            case 'agent':
                return {
                    agent_mode: (document.getElementById('workflow-agent-mode') || {}).value || 'eino_single',
                    input_binding: readBinding('workflow-agent-input'),
                    instruction: (document.getElementById('workflow-agent-instruction') || {}).value || '',
                    output_key: (document.getElementById('workflow-agent-output-key') || {}).value || 'agent_result',
                    join_strategy
                };
            case 'condition':
                return { expression: (document.getElementById('workflow-condition-expression') || {}).value || '', join_strategy };
            case 'hitl':
                return {
                    prompt: (document.getElementById('workflow-hitl-prompt') || {}).value || '',
                    prompt_binding: readBinding('workflow-hitl-prompt-binding'),
                    reviewer: (document.getElementById('workflow-hitl-reviewer') || {}).value || 'human',
                    join_strategy
                };
            case 'output':
                return {
                    output_key: (document.getElementById('workflow-output-key') || {}).value || 'result',
                    source_binding: readBinding('workflow-output-source'),
                    static_value: (document.getElementById('workflow-output-static') || {}).value || '',
                    join_strategy
                };
            case 'end':
                return { result_binding: readBinding('workflow-end-result'), join_strategy };
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

    function nodeSizeForType(type) {
        return NODE_TYPE_SIZES[type] || NODE_DEFAULT_SIZE;
    }

    function viewportCenterPosition() {
        const pan = cy.pan();
        const zoom = cy.zoom();
        const container = cy.container();
        return {
            x: (container.clientWidth / 2 - pan.x) / zoom,
            y: (container.clientHeight / 2 - pan.y) / zoom
        };
    }

    function positionOverlaps(x, y, width, height, excludeId) {
        const pad = NODE_PLACEMENT_PADDING;
        const hw = width / 2 + pad;
        const hh = height / 2 + pad;
        return cy.nodes().some(node => {
            if (excludeId && node.id() === excludeId) return false;
            const p = node.position();
            const bb = node.boundingBox();
            return Math.abs(p.x - x) < hw + bb.w / 2 && Math.abs(p.y - y) < hh + bb.h / 2;
        });
    }

    function findOpenPosition(anchor, type) {
        const size = nodeSizeForType(type);
        const step = 36;
        for (let i = 0; i < 20; i++) {
            const x = anchor.x + (i % 5) * step;
            const y = anchor.y + Math.floor(i / 5) * step;
            if (!positionOverlaps(x, y, size.w, size.h)) {
                return { x, y };
            }
        }
        return anchor;
    }

    function anchorFromSelection(type) {
        if (selectedElement && selectedElement.length && selectedElement.isNode()) {
            const p = selectedElement.position();
            const srcBb = selectedElement.boundingBox();
            const size = nodeSizeForType(type);
            const gap = NODE_PLACEMENT_GAP;
            const candidates = [
                { x: p.x + srcBb.w / 2 + gap + size.w / 2, y: p.y },
                { x: p.x, y: p.y + srcBb.h / 2 + gap + size.h / 2 },
                { x: p.x - srcBb.w / 2 - gap - size.w / 2, y: p.y },
                { x: p.x, y: p.y - srcBb.h / 2 - gap - size.h / 2 }
            ];
            for (let i = 0; i < candidates.length; i++) {
                const c = candidates[i];
                if (!positionOverlaps(c.x, c.y, size.w, size.h)) {
                    return c;
                }
            }
            return findOpenPosition(candidates[0], type);
        }
        return viewportCenterPosition();
    }

    function defaultNodePosition(type) {
        return findOpenPosition(anchorFromSelection(type), type);
    }

    function isPositionInViewport(x, y, padding) {
        const extent = cy.extent();
        const pad = padding == null ? 40 : padding;
        return x >= extent.x1 + pad && x <= extent.x2 - pad &&
            y >= extent.y1 + pad && y <= extent.y2 - pad;
    }

    function highlightNewNode(node) {
        if (!node || !node.length) return;
        if (typeof node.flashClass === 'function') {
            node.flashClass('just-added', 650);
        } else {
            node.addClass('just-added');
            setTimeout(function () {
                if (node.nonempty()) node.removeClass('just-added');
            }, 650);
        }
    }

    function revealWorkflowNode(node) {
        if (!node || !node.length) return;
        const p = node.position();
        if (!isPositionInViewport(p.x, p.y)) {
            cy.animate({ center: { eles: node }, duration: 200 });
        }
        highlightNewNode(node);
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
            position: position || defaultNodePosition(type)
        });
        selectWorkflowElement(node);
        updateEmptyState();
        if (position) {
            highlightNewNode(node);
        } else {
            revealWorkflowNode(node);
        }
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
        const terminals = nodes.filter(node => node.type === 'output' || node.type === 'end');
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
        nodes.filter(node => node.type === 'end').forEach(node => {
            if (edges.some(edge => edge.source === node.id)) errors.push(_t('workflows.validation.outputOutgoing', { label: node.label || node.id }));
        });
        nodes.filter(node => node.type !== 'start').forEach(node => {
            if (!edges.some(edge => edge.target === node.id)) errors.push(_t('workflows.validation.nodeNeedsIncoming', { label: node.label || node.id }));
        });
        nodes.filter(node => node.type !== 'output' && node.type !== 'end').forEach(node => {
            if (!edges.some(edge => edge.source === node.id)) errors.push(_t('workflows.validation.nodeNeedsOutgoing', { label: node.label || node.id }));
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
            const branches = outEdges.map(edge => String(((edge.config || {}).branch || edge.label || '')).trim().toLowerCase());
            if (branches.some(branch => !['true', 'false', '是', '否', 'yes', 'no', 'y', 'n'].includes(branch))) {
                errors.push(_t('workflows.validation.conditionBranchLabel', { label: node.label || node.id }));
            }
            if (new Set(branches).size !== branches.length) {
                errors.push(_t('workflows.validation.conditionBranchDuplicate', { label: node.label || node.id }));
            }
        });
        nodes.filter(node => node.type === 'output').forEach(node => {
            if (!String((node.config || {}).output_key || '').trim()) {
                errors.push(_t('workflows.validation.outputNeedsKey', { label: node.label || node.id }));
            }
        });
        if (terminals.length) {
            const outgoing = new Map();
            edges.forEach(edge => {
                if (!outgoing.has(edge.source)) outgoing.set(edge.source, []);
                outgoing.get(edge.source).push(edge.target);
            });
            const reached = new Set();
            const queue = starts.map(node => node.id);
            while (queue.length) {
                const id = queue.shift();
                if (reached.has(id)) continue;
                reached.add(id);
                (outgoing.get(id) || []).forEach(next => queue.push(next));
            }
            nodes.forEach(node => {
                if (!reached.has(node.id)) errors.push(_t('workflows.validation.nodeUnreachable', { label: node.label || node.id }));
            });
            const visiting = new Set();
            const visited = new Set();
            function visit(id) {
                if (visiting.has(id)) return true;
                if (visited.has(id)) return false;
                visiting.add(id);
                for (const next of (outgoing.get(id) || [])) {
                    if (visit(next)) return true;
                }
                visiting.delete(id);
                visited.add(id);
                return false;
            }
            nodes.forEach(node => {
                if (visit(node.id)) errors.push(_t('workflows.validation.graphCycle', { label: node.label || node.id }));
            });
        }
        return Array.from(new Set(errors));
    }

    async function validateWorkflowGraphOnServer(graph) {
        const response = await apiFetch('/api/workflows/validate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ graph })
        });
        if (!response.ok) {
            const err = await response.json().catch(() => ({}));
            throw new Error(err.error || _t('workflows.validation.serverFailed'));
        }
    }

    function closeWorkflowDryRunPanel() {
        const panel = document.getElementById('workflow-dry-run-panel');
        const output = document.getElementById('workflow-dry-run-output');
        if (!panel) return;
        panel.hidden = true;
        if (output) output.innerHTML = '';
    }

    function renderWorkflowDryRunTrace(result) {
        const panel = document.getElementById('workflow-dry-run-panel');
        const output = document.getElementById('workflow-dry-run-output');
        if (!panel || !output) return;
        const trace = (result && result.trace) || [];
        panel.hidden = false;
        if (!trace.length) {
            output.textContent = _t('workflows.dryRunNoTrace') || 'No trace';
            return;
        }
        output.innerHTML = trace.map((item, index) => {
            const status = item.status || '';
            const label = item.label || item.nodeId || ('#' + (index + 1));
            return `<div class="workflow-dry-run-step">
                <strong>${index + 1}. ${esc(label)}</strong>
                <span>${esc(item.type || '')} · ${esc(status)}</span>
            </div>`;
        }).join('');
    }

    window.closeWorkflowDryRunPanel = closeWorkflowDryRunPanel;

    window.saveWorkflowDraft = async function () {
        if (typeof requirePermission === 'function' && !requirePermission('workflow:write')) return;
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
        try {
            await validateWorkflowGraphOnServer(graph);
        } catch (error) {
            showNotification(error.message || _t('workflows.validation.serverFailed'), 'error');
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

    window.dryRunWorkflowDraft = async function () {
        initCy();
        const graph = elementsToGraph();
        const errors = validateWorkflowGraph(graph);
        if (errors.length) {
            showNotification(errors.slice(0, 4).join('；'), 'error');
            return;
        }
        const message = window.prompt(_t('workflows.dryRunPrompt') || 'Input message for dry-run', 'ping');
        if (message === null) return;
        try {
            const response = await apiFetch('/api/workflows/dry-run', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ graph, inputs: { message } })
            });
            const data = await response.json().catch(() => ({}));
            if (!response.ok) {
                throw new Error(data.error || _t('workflows.dryRunFailed'));
            }
            const result = data.result || {};
            const trace = result.trace || [];
            console.groupCollapsed('[Workflow dry-run]');
            console.table(trace.map(item => ({
                nodeId: item.nodeId,
                label: item.label,
                type: item.type,
                status: item.status
            })));
            console.log(result);
            console.groupEnd();
            renderWorkflowDryRunTrace(result);
            if (typeof showNotification === 'function') {
                showNotification(_t('workflows.dryRunDone') || 'Dry-run completed', 'success');
            }
        } catch (error) {
            showNotification(error.message || _t('workflows.dryRunFailed'), 'error');
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

    window.useWorkflowConditionExample = function (expr) {
        const input = document.getElementById('workflow-condition-expression');
        if (!input) return;
        input.value = expr || '';
        updateWorkflowTypedConfig();
        input.focus();
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
        if (typeof window.refreshRoleModalSelects === 'function') {
            window.refreshRoleModalSelects();
        }
    };

    function workflowPackageClient() {
        return window.WorkflowPackageClient || null;
    }

    function workflowPackageText(key, fallback) {
        const translated = _t(key);
        return translated === key ? fallback : translated;
    }

    function workflowPackageStorageGet(key) {
        try { return window.sessionStorage.getItem(key) || ''; } catch (_) { return ''; }
    }

    function workflowPackageStorageSet(key, value) {
        try {
            if (value) window.sessionStorage.setItem(key, value);
            else window.sessionStorage.removeItem(key);
        } catch (_) { /* session restore is best effort */ }
    }

    function workflowPackageInspectionEl() {
        return document.getElementById('workflow-package-inspection');
    }

    function workflowPackageResolutionEl() {
        return document.getElementById('workflow-package-resolution');
    }

    function workflowPackageSubmitBtn() {
        return document.getElementById('workflow-package-submit-btn');
    }

    function workflowPackageSetStep(step) {
        const inspectionStep = document.getElementById('workflow-package-step-inspection');
        const importStep = document.getElementById('workflow-package-step-import');
        if (inspectionStep) inspectionStep.classList.toggle('is-active', step === 'inspection');
        if (importStep) importStep.classList.toggle('is-active', step === 'import');
    }

    function workflowPackageSetStatus(target, message, type) {
        if (!target) return;
        target.innerHTML = `<div class="workflow-package-status${type ? ' is-' + esc(type) : ''}">${esc(message)}</div>`;
    }

    function workflowPackageErrorMessage(error) {
        const code = error && error.code ? error.code : '';
        const messages = {
            WFPKG_FILE_REQUIRED: '请选择本地图编排包后再预检。',
            WFPKG_FILE_TOO_LARGE: '文件超过 10 MiB 限制，请选择更小的本地包。',
            WFPKG_FILE_INVALID: '文件格式无效，请选择 .csapkg.zip 本地包。',
            WFPKG_MANIFEST_INVALID: '本地包清单无效，无法完成预检。',
            WFPKG_PACKAGE_UNSUPPORTED: '本地包格式或版本不受支持。',
            WFPKG_INSPECTION_NOT_FOUND: '预检记录不存在，请重新上传并预检。',
            WFPKG_INSPECTION_EXPIRED: '预检已过期，请重新上传并预检。',
            WFPKG_INSPECTION_CONSUMED: '该预检已完成导入，不能重复使用。',
            WFPKG_CONFLICT_CHANGED: '本地工作流在预检后发生变化，请重新预检后再导入。',
            WFPKG_ID_CONFLICT: '当前处理方式与最新冲突状态不匹配，请重新预检。',
            WFPKG_INVALID_ACTION: '导入处理方式无效，请重新选择。',
            WFPKG_INVALID_RENAME_ID: '请输入可用的新工作流 ID。',
            WFPKG_OVERWRITE_CONFIRMATION_REQUIRED: '请勾选确认后再覆盖本地工作流。',
            WFPKG_IDEMPOTENCY_KEY_REQUIRED: '浏览器无法生成导入请求标识，请刷新页面后重试。',
            WFPKG_IDEMPOTENCY_KEY_REUSED: '导入请求状态异常，请重新预检后再试。',
            WFPKG_IMPORT_FAILED: '导入失败，本地工作流未被修改。',
            WFPKG_EXPORT_FAILED: '导出失败，请稍后重试。',
            WFPKG_WORKFLOW_NOT_FOUND: '未找到该工作流，无法导出。'
        };
        return messages[code] || (error && error.message) || '操作未完成，请稍后重试。';
    }

    function workflowPackageConflictCopy(conflict) {
        const state = (conflict && conflict.state) || 'none';
        if (state === 'identical') return { message: '检测到同 ID 且内容相同的本地工作流；本次将跳过导入。', type: 'success' };
        if (state === 'id_conflict') return { message: '检测到同 ID 但内容不同的本地工作流。默认保留本地版本。', type: 'warning' };
        return { message: '未发现同 ID 的本地工作流，可创建导入。', type: 'success' };
    }

    function renderWorkflowPackageInspection() {
        const target = workflowPackageInspectionEl();
        const inspection = workflowPackageState.inspection;
        if (!inspection) {
            if (target) target.innerHTML = '';
            return;
        }
        const workflow = inspection.workflow || {};
        const conflict = inspection.conflict || {};
        const conflictCopy = workflowPackageConflictCopy(conflict);
        const rows = [
            ['工作流名称', workflow.name || '未命名工作流'],
            ['源工作流 ID', workflow.source_id || '—'],
            ['源版本', workflow.source_revision || '—'],
            ['图规模', `${workflow.node_count || 0} 个节点 · ${workflow.edge_count || 0} 条连线`],
            ['内容摘要', workflow.content_hash || '—'],
            ['预检有效期', inspection.expires_at || '—']
        ];
        if (!target) return;
        target.innerHTML = `<div class="workflow-package-status is-${conflictCopy.type}">${esc(conflictCopy.message)}</div>` + rows.map(function (row) {
            return `<div class="workflow-package-summary-row"><span>${esc(row[0])}</span><code title="${esc(row[1])}">${esc(row[1])}</code></div>`;
        }).join('');
    }

    function workflowPackageResolutionCard(action, title, description, selected) {
        return `<label class="workflow-package-choice${selected ? ' is-selected' : ''}">
            <input type="radio" name="workflow-package-resolution-action" value="${esc(action)}" ${selected ? 'checked' : ''} onchange="selectWorkflowPackageResolution('${esc(action)}')">
            <span><strong>${esc(title)}</strong><small>${esc(description)}</small></span>
        </label>`;
    }

    function renderWorkflowPackageResolution(message, type) {
        const target = workflowPackageResolutionEl();
        const submit = workflowPackageSubmitBtn();
        const inspection = workflowPackageState.inspection;
        if (!inspection) {
            if (target) workflowPackageSetStatus(target, message || '请先选择本地包并完成预检。', type || '');
            if (submit) submit.disabled = true;
            return;
        }
        const client = workflowPackageClient();
        const conflictState = (inspection.conflict && inspection.conflict.state) || 'none';
        const allowed = client ? client.allowedActions(conflictState) : [];
        if (allowed.indexOf(workflowPackageState.resolutionAction) === -1) {
            workflowPackageState.resolutionAction = conflictState === 'none' ? 'create' : 'keep_existing';
        }
        const action = workflowPackageState.resolutionAction;
        let html = '';
        if (message) html += `<div class="workflow-package-status${type ? ' is-' + esc(type) : ''}">${esc(message)}</div>`;
        if (conflictState === 'none') {
            html += '<div class="workflow-package-status is-success">导入后会创建一个新的本地工作流。</div>';
        } else if (conflictState === 'identical') {
            html += '<div class="workflow-package-status is-success">内容已经存在，无需重复导入。</div>';
        } else {
            html += workflowPackageResolutionCard('keep_existing', '保留本地版本', '不修改当前本地工作流；导入记录会保留。', action === 'keep_existing');
            if (!workflowPackageState.riskChoicesVisible) {
                html += '<button type="button" class="workflow-package-reveal" onclick="revealWorkflowPackageRiskChoices()">更改处理方式</button>';
            } else {
                html += workflowPackageResolutionCard('overwrite', '覆盖本地版本', '替换名称、描述、图定义和启用状态；需要再次确认。', action === 'overwrite');
                html += workflowPackageResolutionCard('rename', '另存为新 ID', '保留当前本地工作流，并以新的 ID 创建副本。', action === 'rename');
                if (action === 'rename') {
                    html += `<label class="workflow-package-rename-field">新的工作流 ID<input id="workflow-package-new-id" class="form-input" type="text" value="${esc(workflowPackageState.newWorkflowId)}" placeholder="例如：web-scan-basic-copy" oninput="updateWorkflowPackageRenameId()" autocomplete="off"></label>`;
                }
            }
        }
        if (target) target.innerHTML = html;
        if (submit) {
            const renameIncomplete = action === 'rename' && !workflowPackageState.newWorkflowId.trim();
            submit.disabled = !action || renameIncomplete;
            if (action === 'overwrite') submit.textContent = '继续确认覆盖';
            else if (action === 'rename') submit.textContent = '确认另存';
            else submit.textContent = action === 'create' ? '确认创建' : '确认完成';
        }
        workflowPackageSetStep('import');
    }

    function resetWorkflowPackageImport(options) {
        const keepInspection = options && options.keepInspection;
        workflowPackageState.file = null;
        workflowPackageState.importRecord = null;
        workflowPackageState.newWorkflowId = '';
        workflowPackageState.riskChoicesVisible = false;
        workflowPackageState.idempotencyKey = '';
        workflowPackageState.requestSignature = '';
        if (!keepInspection) {
            workflowPackageState.inspection = null;
            workflowPackageState.resolutionAction = '';
            workflowPackageStorageSet(WORKFLOW_PACKAGE_INSPECTION_STORAGE_KEY, '');
        }
        workflowPackageStorageSet(WORKFLOW_PACKAGE_IMPORT_STORAGE_KEY, '');
        const input = document.getElementById('workflow-package-file-input');
        const name = document.getElementById('workflow-package-file-name');
        if (input) input.value = '';
        if (name) name.textContent = workflowPackageText('workflows.package.noFile', '未选择文件');
    }

    function displayWorkflowPackageError(error) {
        const message = workflowPackageErrorMessage(error);
        if (error && (error.code === 'WFPKG_INSPECTION_EXPIRED' || error.code === 'WFPKG_CONFLICT_CHANGED' || error.code === 'WFPKG_INSPECTION_CONSUMED')) {
            workflowPackageState.inspection = null;
            workflowPackageState.resolutionAction = '';
            workflowPackageStorageSet(WORKFLOW_PACKAGE_INSPECTION_STORAGE_KEY, '');
            workflowPackageSetStep('inspection');
            renderWorkflowPackageInspection();
        }
        renderWorkflowPackageResolution(message, 'warning');
        if (typeof showNotification === 'function') showNotification(message, 'error');
    }

    function renderWorkflowPackageImportResult(importRecord) {
        const result = importRecord && importRecord.result;
        const messages = {
            created: '导入成功，已创建新的本地工作流。',
            overwritten: '导入成功，已覆盖本地工作流。',
            renamed: '导入成功，已按新 ID 创建本地工作流。',
            kept_existing: '已保留本地工作流，未写入任何更改。',
            skipped_identical: '本地已存在相同内容，已跳过导入。'
        };
        const message = messages[result] || '导入已完成。';
        renderWorkflowPackageResolution(message, result === 'kept_existing' || result === 'skipped_identical' ? '' : 'success');
        const submit = workflowPackageSubmitBtn();
        if (submit) {
            submit.disabled = true;
            submit.textContent = '导入已完成';
        }
    }

    async function restoreWorkflowPackageState() {
        const client = workflowPackageClient();
        if (!client || typeof apiFetch !== 'function') return;
        const importId = workflowPackageStorageGet(WORKFLOW_PACKAGE_IMPORT_STORAGE_KEY);
        if (importId) {
            try {
                const data = await client.getImport(apiFetch, importId);
                workflowPackageState.importRecord = data.import || data;
                renderWorkflowPackageImportResult(workflowPackageState.importRecord);
                return;
            } catch (_) {
                workflowPackageStorageSet(WORKFLOW_PACKAGE_IMPORT_STORAGE_KEY, '');
            }
        }
        const inspectionId = workflowPackageStorageGet(WORKFLOW_PACKAGE_INSPECTION_STORAGE_KEY);
        if (!inspectionId) return;
        try {
            const data = await client.getInspection(apiFetch, inspectionId);
            workflowPackageState.inspection = data.inspection || data;
            renderWorkflowPackageInspection();
            renderWorkflowPackageResolution();
        } catch (error) {
            workflowPackageStorageSet(WORKFLOW_PACKAGE_INSPECTION_STORAGE_KEY, '');
            displayWorkflowPackageError(error);
        }
    }

    window.openWorkflowPackageImportModal = async function () {
        if (typeof requirePermission === 'function' && !requirePermission('workflow:write')) return;
        if (typeof openAppModal === 'function') openAppModal('workflow-package-import-modal', { focusEl: document.getElementById('workflow-package-inspect-btn') });
        if (typeof window.applyTranslations === 'function') window.applyTranslations(document.getElementById('workflow-package-import-modal'));
        if (!workflowPackageState.inspection && !workflowPackageState.importRecord) {
            renderWorkflowPackageResolution();
            await restoreWorkflowPackageState();
        } else if (workflowPackageState.importRecord) {
            renderWorkflowPackageImportResult(workflowPackageState.importRecord);
        } else {
            renderWorkflowPackageInspection();
            renderWorkflowPackageResolution();
        }
    };

    window.closeWorkflowPackageImportModal = function () {
        if (typeof closeAppModal === 'function') closeAppModal('workflow-package-import-modal');
    };

    window.onWorkflowPackageFileSelected = function () {
        const input = document.getElementById('workflow-package-file-input');
        const name = document.getElementById('workflow-package-file-name');
        const file = input && input.files ? input.files[0] : null;
        resetWorkflowPackageImport();
        workflowPackageState.file = file || null;
        if (name) name.textContent = file ? file.name : workflowPackageText('workflows.package.noFile', '未选择文件');
        renderWorkflowPackageResolution();
        workflowPackageSetStep('inspection');
    };

    window.inspectWorkflowPackage = async function () {
        if (typeof requirePermission === 'function' && !requirePermission('workflow:write')) return;
        const client = workflowPackageClient();
        const button = document.getElementById('workflow-package-inspect-btn');
        if (!client || typeof apiFetch !== 'function') {
            displayWorkflowPackageError({ code: 'WFPKG_REQUEST_FAILED', message: '导入服务尚未准备好，请刷新页面后重试。' });
            return;
        }
        if (button) button.disabled = true;
        try {
            const data = await client.createInspection(apiFetch, workflowPackageState.file);
            workflowPackageState.inspection = data.inspection || data;
            workflowPackageState.importRecord = null;
            workflowPackageState.riskChoicesVisible = false;
            workflowPackageState.newWorkflowId = '';
            workflowPackageState.resolutionAction = '';
            workflowPackageStorageSet(WORKFLOW_PACKAGE_INSPECTION_STORAGE_KEY, workflowPackageState.inspection.id || '');
            renderWorkflowPackageInspection();
            renderWorkflowPackageResolution();
        } catch (error) {
            displayWorkflowPackageError(error);
        } finally {
            if (button) button.disabled = false;
        }
    };

    window.revealWorkflowPackageRiskChoices = function () {
        workflowPackageState.riskChoicesVisible = true;
        renderWorkflowPackageResolution();
    };

    window.selectWorkflowPackageResolution = function (action) {
        const client = workflowPackageClient();
        const conflict = workflowPackageState.inspection && workflowPackageState.inspection.conflict;
        const allowed = client ? client.allowedActions((conflict && conflict.state) || 'none') : [];
        if (allowed.indexOf(action) === -1) return;
        workflowPackageState.resolutionAction = action;
        if (action !== 'rename') workflowPackageState.newWorkflowId = '';
        renderWorkflowPackageResolution();
    };

    window.updateWorkflowPackageRenameId = function () {
        const input = document.getElementById('workflow-package-new-id');
        workflowPackageState.newWorkflowId = input ? input.value : '';
        const submit = workflowPackageSubmitBtn();
        if (submit) submit.disabled = !workflowPackageState.newWorkflowId.trim();
    };

    async function performWorkflowPackageImport(request) {
        const client = workflowPackageClient();
        const signature = JSON.stringify(request);
        if (!workflowPackageState.idempotencyKey || workflowPackageState.requestSignature !== signature) {
            workflowPackageState.idempotencyKey = client.createIdempotencyKey();
            workflowPackageState.requestSignature = signature;
        }
        const submit = workflowPackageSubmitBtn();
        if (submit) submit.disabled = true;
        try {
            const data = await client.applyImport(apiFetch, request, workflowPackageState.idempotencyKey);
            workflowPackageState.importRecord = data.import || data;
            workflowPackageStorageSet(WORKFLOW_PACKAGE_IMPORT_STORAGE_KEY, workflowPackageState.importRecord.id || '');
            workflowPackageStorageSet(WORKFLOW_PACKAGE_INSPECTION_STORAGE_KEY, '');
            workflowPackageState.inspection = null;
            renderWorkflowPackageImportResult(workflowPackageState.importRecord);
            if (typeof window.refreshWorkflows === 'function') await window.refreshWorkflows();
        } catch (error) {
            displayWorkflowPackageError(error);
        } finally {
            if (submit && !workflowPackageState.importRecord) submit.disabled = false;
        }
    }

    window.submitWorkflowPackageImport = async function () {
        const client = workflowPackageClient();
        if (!client || !workflowPackageState.inspection) return;
        if (workflowPackageState.resolutionAction === 'overwrite') {
            const checkbox = document.getElementById('workflow-package-overwrite-confirm');
            const submit = document.getElementById('workflow-package-overwrite-submit-btn');
            if (checkbox) checkbox.checked = false;
            if (submit) submit.disabled = true;
            if (typeof openAppModal === 'function') openAppModal('workflow-package-overwrite-modal', { focusEl: checkbox });
            return;
        }
        let request;
        try {
            request = client.buildImportRequest({
                inspectionId: workflowPackageState.inspection.id,
                action: workflowPackageState.resolutionAction,
                newWorkflowId: workflowPackageState.newWorkflowId,
                confirmOverwrite: false
            });
        } catch (error) {
            displayWorkflowPackageError(error);
            return;
        }
        await performWorkflowPackageImport(request);
    };

    window.closeWorkflowPackageOverwriteModal = function () {
        if (typeof closeAppModal === 'function') closeAppModal('workflow-package-overwrite-modal');
    };

    window.updateWorkflowPackageOverwriteConfirmation = function () {
        const checkbox = document.getElementById('workflow-package-overwrite-confirm');
        const submit = document.getElementById('workflow-package-overwrite-submit-btn');
        if (submit) submit.disabled = !checkbox || !checkbox.checked;
    };

    window.confirmWorkflowPackageOverwrite = async function () {
        const checkbox = document.getElementById('workflow-package-overwrite-confirm');
        const client = workflowPackageClient();
        if (!client || !checkbox || !checkbox.checked || !workflowPackageState.inspection) return;
        try {
            const request = client.buildImportRequest({
                inspectionId: workflowPackageState.inspection.id,
                action: 'overwrite',
                newWorkflowId: '',
                confirmOverwrite: true
            });
            window.closeWorkflowPackageOverwriteModal();
            await performWorkflowPackageImport(request);
        } catch (error) {
            displayWorkflowPackageError(error);
        }
    };

    window.exportCurrentWorkflowPackage = async function () {
        if (typeof requirePermission === 'function' && !requirePermission('workflow:read')) return;
        const id = currentWorkflowId || readWorkflowMetaFromForm().id;
        if (!id) {
            if (typeof showNotification === 'function') showNotification('请先选择已保存的工作流后再导出。', 'warning');
            return;
        }
        try {
            const response = await apiFetch(`/api/workflows/${encodeURIComponent(id)}/package`, { method: 'GET' });
            if (!response.ok) {
                const error = workflowPackageClient() ? await workflowPackageClient().readApiError(response) : {};
                throw error;
            }
            const blob = await response.blob();
            const disposition = response.headers.get('Content-Disposition') || '';
            const match = disposition.match(/filename\*?=(?:UTF-8''|\")?([^;\"]+)/i);
            const filename = match && match[1] ? decodeURIComponent(match[1].trim()) : `${id}.csapkg.zip`;
            const link = document.createElement('a');
            const url = URL.createObjectURL(blob);
            link.href = url;
            link.download = filename;
            document.body.appendChild(link);
            link.click();
            link.remove();
            window.setTimeout(function () { URL.revokeObjectURL(url); }, 0);
        } catch (error) {
            const message = workflowPackageErrorMessage(error);
            if (typeof showNotification === 'function') showNotification(message, 'error');
        }
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
