// pageroutemanagement
let currentPage = 'dashboard';

// initializeroute
function initRouter() {
    // fromURL hashreadpage（ifhas）
    const hash = window.location.hash.slice(1);
    if (hash) {
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        if (pageId && ['dashboard', 'chat', 'info-collect', 'vulnerabilities', 'webshell', 'chat-files', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'roles-management', 'skills-monitor', 'skills-management', 'agents-management', 'settings', 'tasks'].includes(pageId)) {
            switchPage(pageId);
            
 // if it ischatpageandhasconversationparameter，loadforshouldconversation
            if (pageId === 'chat' && hashParts.length > 1) {
                const params = new URLSearchParams(hashParts[1]);
                const conversationId = params.get('conversation');
                if (conversationId) {
                    setTimeout(() => {
                        // trymultiplemethodcallloadConversation
                        if (typeof loadConversation === 'function') {
                            loadConversation(conversationId);
                        } else if (typeof window.loadConversation === 'function') {
                            window.loadConversation(conversationId);
                        } else {
                            console.warn('loadConversation function not found');
                        }
                    }, 500);
                }
            }
            return;
        }
    }
    
    // defaultShow dashboard
    switchPage('dashboard');
}

// switchpage
function switchPage(pageId) {
    // hideallpage
    document.querySelectorAll('.page').forEach(page => {
        page.classList.remove('active');
    });
    
    // Show Targetpage
    const targetPage = document.getElementById(`page-${pageId}`);
    if (targetPage) {
        targetPage.classList.add('active');
        currentPage = pageId;
        
        // updateURL hash
        window.location.hash = pageId;
        
        // updatenavigationstatus
        updateNavState(pageId);
        
 // page's initialize
        initPage(pageId);
    }
}

// updatenavigationstatus
function updateNavState(pageId) {
    // removeallactivestatus
    document.querySelectorAll('.nav-item').forEach(item => {
        item.classList.remove('active');
    });
    
    document.querySelectorAll('.nav-submenu-item').forEach(item => {
        item.classList.remove('active');
    });
    
    // settingsactivestatus
    if (pageId === 'mcp-monitor' || pageId === 'mcp-management') {
 // MCPmenuitem
        const mcpItem = document.querySelector('.nav-item[data-page="mcp"]');
        if (mcpItem) {
            mcpItem.classList.add('active');
 // expandMCPmenu
            mcpItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'knowledge-management' || pageId === 'knowledge-retrieval-logs') {
 // knowledgemenuitem
        const knowledgeItem = document.querySelector('.nav-item[data-page="knowledge"]');
        if (knowledgeItem) {
            knowledgeItem.classList.add('active');
 // expandknowledgemenu
            knowledgeItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'skills-monitor' || pageId === 'skills-management') {
 // Skillsmenuitem
        const skillsItem = document.querySelector('.nav-item[data-page="skills"]');
        if (skillsItem) {
            skillsItem.classList.add('active');
 // expandSkillsmenu
            skillsItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'agents-management') {
        const agentsItem = document.querySelector('.nav-item[data-page="agents"]');
        if (agentsItem) {
            agentsItem.classList.add('active');
            agentsItem.classList.add('expanded');
        }
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'roles-management') {
 // rolemenuitem
        const rolesItem = document.querySelector('.nav-item[data-page="roles"]');
        if (rolesItem) {
            rolesItem.classList.add('active');
 // expandrolemenu
            rolesItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else {
 // menuitem
        const navItem = document.querySelector(`.nav-item[data-page="${pageId}"]`);
        if (navItem) {
            navItem.classList.add('active');
        }
    }
}

// switchmenu
function toggleSubmenu(menuId) {
    const sidebar = document.getElementById('main-sidebar');
    const navItem = document.querySelector(`.nav-item[data-page="${menuId}"]`);
    
    if (!navItem) return;
    
 // checksidebariscollapse
    if (sidebar && sidebar.classList.contains('collapsed')) {
        // collapsestatusunderShow popup menu
        showSubmenuPopup(navItem, menuId);
    } else {
 // expandstatusunderNormalswitchmenu
        navItem.classList.toggle('expanded');
    }
}

// Show menuout
function showSubmenuPopup(navItem, menuId) {
    // removeotheralreadyopen's popup menu
    const existingPopup = document.querySelector('.submenu-popup');
    if (existingPopup) {
        existingPopup.remove();
        return; // ifalreadyopen，clickwhenClose
    }
    
    const navItemContent = navItem.querySelector('.nav-item-content');
    const submenu = navItem.querySelector('.nav-submenu');
    
    if (!submenu) return;
    
    // getmenuposition
    const rect = navItemContent.getBoundingClientRect();
    
    // createpopup menu
    const popup = document.createElement('div');
    popup.className = 'submenu-popup';
    popup.style.position = 'fixed';
    popup.style.left = (rect.right + 8) + 'px';
    popup.style.top = rect.top + 'px';
    popup.style.zIndex = '1000';
    
 // Copymenuitemtopopup menu
    const submenuItems = submenu.querySelectorAll('.nav-submenu-item');
    submenuItems.forEach(item => {
        const popupItem = document.createElement('div');
        popupItem.className = 'submenu-popup-item';
        popupItem.textContent = item.textContent.trim();
        
 // check if it iscurrent's page
        const pageId = item.getAttribute('data-page');
        if (pageId && document.querySelector(`.nav-submenu-item[data-page="${pageId}"].active`)) {
            popupItem.classList.add('active');
        }
        
        popupItem.onclick = function(e) {
            e.stopPropagation();
            e.preventDefault();
            
 // getpageIDswitch
            const pageId = item.getAttribute('data-page');
            if (pageId) {
                switchPage(pageId);
            }
            
            // Closepopup menu
            popup.remove();
            document.removeEventListener('click', closePopup);
        };
        popup.appendChild(popupItem);
    });
    
    document.body.appendChild(popup);
    
    // clickExternalClosepopup menu
    const closePopup = function(e) {
        if (!popup.contains(e.target) && !navItem.contains(e.target)) {
            popup.remove();
            document.removeEventListener('click', closePopup);
        }
    };
    
    // delayaddeventlisten，avoidimmediatelytrigger
    setTimeout(() => {
        document.addEventListener('click', closePopup);
    }, 0);
}

// initializepage
function initPage(pageId) {
    switch(pageId) {
        case 'dashboard':
            if (typeof refreshDashboard === 'function') {
                refreshDashboard();
            }
            break;
        case 'chat':
 // restoreconversationlistcollapsestatus（fromotherpagereturnwhenmaintainuseselect）
            initConversationSidebarState();
            break;
        case 'info-collect':
            // Infocollectpage
            if (typeof initInfoCollectPage === 'function') {
                initInfoCollectPage();
            }
            break;
        case 'tasks':
            // initializetaskmanagement page
            if (typeof initTasksPage === 'function') {
                initTasksPage();
            }
            break;
        case 'mcp-monitor':
            // initializeMonitorpanel
            if (typeof refreshMonitorPanel === 'function') {
                refreshMonitorPanel();
            }
            break;
        case 'mcp-management':
            // initializeMCPmanagement
 // firstloadExternalMCPlist（fast），afterloadtoollist
            if (typeof loadExternalMCPs === 'function') {
                loadExternalMCPs().catch(err => {
                    console.warn('loadExternalMCPlist failed:', err);
                });
            }
 // loadtoollist（MCPtoolconfigurationalreadytoMCPmanagement page）
            // useasyncload，avoidblockpagerender
            if (typeof loadToolsList === 'function') {
                // ensuretoolpaginationsettingsalreadyinitialize
                if (typeof getToolsPageSize === 'function' && typeof toolsPagination !== 'undefined') {
                    toolsPagination.pageSize = getToolsPageSize();
                }
                // delayload，letpagefirstrender
                setTimeout(() => {
                    loadToolsList(1, '').catch(err => {
                        console.error('Failed to load tool list:', err);
                    });
                }, 100);
            }
            break;
        case 'vulnerabilities':
            // initializevulnerabilitymanagement page
            if (typeof initVulnerabilityPage === 'function') {
                initVulnerabilityPage();
            }
            break;
        case 'webshell':
            // initialize WebShell management page
            if (typeof initWebshellPage === 'function') {
                initWebshellPage();
            }
            break;
        case 'chat-files':
            if (typeof initChatFilesPage === 'function') {
                initChatFilesPage();
            }
            break;
        case 'settings':
            // initializesettingspage（notneedloadtoollist）
            if (typeof loadConfig === 'function') {
                loadConfig(false);
            }
            break;
        case 'roles-management':
            // initializerolemanagement page
            // resetsearchUI（variablewillatundertimesearchwhenautoupdate）
            const rolesSearchInput = document.getElementById('roles-search');
            if (rolesSearchInput) {
                rolesSearchInput.value = '';
            }
            const rolesSearchClear = document.getElementById('roles-search-clear');
            if (rolesSearchClear) {
                rolesSearchClear.style.display = 'none';
            }
            if (typeof loadRoles === 'function') {
                loadRoles().then(() => {
                    if (typeof renderRolesList === 'function') {
                        renderRolesList();
                    }
                });
            }
            break;
        case 'skills-monitor':
            // initializeSkillsstatusMonitorpage
            if (typeof loadSkillsMonitor === 'function') {
                loadSkillsMonitor();
            }
            break;
        case 'skills-management':
            // initializeSkillsmanagement page
            // resetsearchUI（variablewillatundertimesearchwhenautoupdate）
            const skillsSearchInput = document.getElementById('skills-search');
            if (skillsSearchInput) {
                skillsSearchInput.value = '';
            }
            const skillsSearchClear = document.getElementById('skills-search-clear');
            if (skillsSearchClear) {
                skillsSearchClear.style.display = 'none';
            }
            if (typeof initSkillsPagination === 'function') {
                initSkillsPagination();
            }
            if (typeof loadSkills === 'function') {
                loadSkills();
            }
            break;
        case 'agents-management':
            if (typeof loadMarkdownAgents === 'function') {
                loadMarkdownAgents();
            }
            break;
    }
    
    // cleanupotherpage's timer
    if (pageId !== 'tasks' && typeof cleanupTasksPage === 'function') {
        cleanupTasksPage();
    }
}

// pageloadcompleteafterinitializeroute
document.addEventListener('DOMContentLoaded', function() {
    initRouter();
    initSidebarState();
    
    // listenhashchange
    window.addEventListener('hashchange', function() {
        const hash = window.location.hash.slice(1);
 // processparameter's hash（like chat?conversation=xxx）
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        
        if (pageId && ['chat', 'info-collect', 'tasks', 'vulnerabilities', 'webshell', 'chat-files', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'roles-management', 'skills-monitor', 'skills-management', 'agents-management', 'settings'].includes(pageId)) {
            switchPage(pageId);
            
 // if it ischatpageandhasconversationparameter，loadforshouldconversation
            if (pageId === 'chat' && hashParts.length > 1) {
                const params = new URLSearchParams(hashParts[1]);
                const conversationId = params.get('conversation');
                if (conversationId) {
                    setTimeout(() => {
                        // trymultiplemethodcallloadConversation
                        if (typeof loadConversation === 'function') {
                            loadConversation(conversationId);
                        } else if (typeof window.loadConversation === 'function') {
                            window.loadConversation(conversationId);
                        } else {
                            console.warn('loadConversation function not found');
                        }
                    }, 200);
                }
            }
        }
    });
    
    // pageloadwhenalsocheckhashparameter
    const hash = window.location.hash.slice(1);
    if (hash) {
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        if (pageId === 'chat' && hashParts.length > 1) {
            const params = new URLSearchParams(hashParts[1]);
            const conversationId = params.get('conversation');
            if (conversationId && typeof loadConversation === 'function') {
                setTimeout(() => {
                    loadConversation(conversationId);
                }, 500);
            }
        }
    }
});

// switchsidebarcollapse/expand
function toggleSidebar() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        sidebar.classList.toggle('collapsed');
        // SavecollapsestatustolocalStorage
        const isCollapsed = sidebar.classList.contains('collapsed');
        localStorage.setItem('sidebarCollapsed', isCollapsed ? 'true' : 'false');
    }
}

// initializesidebarstatus
function initSidebarState() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        const savedState = localStorage.getItem('sidebarCollapsed');
        if (savedState === 'true') {
            sidebar.classList.add('collapsed');
        }
    }
    initConversationSidebarState();
}

// switchconversationpageleft sidelistcollapse/expand
function toggleConversationSidebar() {
    const sidebar = document.getElementById('conversation-sidebar');
    if (sidebar) {
        sidebar.classList.toggle('collapsed');
        const isCollapsed = sidebar.classList.contains('collapsed');
        localStorage.setItem('conversationSidebarCollapsed', isCollapsed ? 'true' : 'false');
    }
}

// restoreconversationlistcollapsestatus（Enterconversationpagewhentake effect）
function initConversationSidebarState() {
    const sidebar = document.getElementById('conversation-sidebar');
    if (sidebar) {
        const savedState = localStorage.getItem('conversationSidebarCollapsed');
        if (savedState === 'true') {
            sidebar.classList.add('collapsed');
        } else {
            sidebar.classList.remove('collapsed');
        }
    }
}

// exportfunctionforotheruse
window.switchPage = switchPage;
window.toggleSubmenu = toggleSubmenu;
window.toggleSidebar = toggleSidebar;
window.toggleConversationSidebar = toggleConversationSidebar;
window.currentPage = function() { return currentPage; };

