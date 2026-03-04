// Page routing management
let currentPage = 'dashboard';

// Initialize routes
function initRouter() {
    // Read page from URL hash (if present)
    const hash = window.location.hash.slice(1);
    if (hash) {
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        if (pageId && ['dashboard', 'chat', 'info-collect', 'vulnerabilities', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'roles-management', 'skills-monitor', 'skills-management', 'settings', 'tasks', 'memory'].includes(pageId)) {
            switchPage(pageId);
            
            // If this is the chat page with a conversation param, load that conversation
            if (pageId === 'chat' && hashParts.length > 1) {
                const params = new URLSearchParams(hashParts[1]);
                const conversationId = params.get('conversation');
                if (conversationId) {
                    setTimeout(() => {
                        // Try multiple ways to call loadConversation
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
    
    // Default: show dashboard
    switchPage('dashboard');
}

// Toggle page
function switchPage(pageId) {
    // Hide all pages
    document.querySelectorAll('.page').forEach(page => {
        page.classList.remove('active');
    });
    
    // Show target page
    const targetPage = document.getElementById(`page-${pageId}`);
    if (targetPage) {
        targetPage.classList.add('active');
        currentPage = pageId;
        
        // UpdateURL hash
        window.location.hash = pageId;
        
        // Update navigation status
        updateNavState(pageId);
        
        // Page-specific initialization
        initPage(pageId);
    }
}

// Update navigation status
function updateNavState(pageId) {
    // Remove all active states
    document.querySelectorAll('.nav-item').forEach(item => {
        item.classList.remove('active');
    });
    
    document.querySelectorAll('.nav-submenu-item').forEach(item => {
        item.classList.remove('active');
    });
    
    // Set active state
    if (pageId === 'mcp-monitor' || pageId === 'mcp-management') {
        // MCP submenu item
        const mcpItem = document.querySelector('.nav-item[data-page="mcp"]');
        if (mcpItem) {
            mcpItem.classList.add('active');
            // Expand MCP submenu
            mcpItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'knowledge-management' || pageId === 'knowledge-retrieval-logs') {
        // Knowledge submenu item
        const knowledgeItem = document.querySelector('.nav-item[data-page="knowledge"]');
        if (knowledgeItem) {
            knowledgeItem.classList.add('active');
            // Expand Knowledge submenu
            knowledgeItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'skills-monitor' || pageId === 'skills-management') {
        // Skills submenu item
        const skillsItem = document.querySelector('.nav-item[data-page="skills"]');
        if (skillsItem) {
            skillsItem.classList.add('active');
            // Expand Skills submenu
            skillsItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'roles-management') {
        // Roles submenu item
        const rolesItem = document.querySelector('.nav-item[data-page="roles"]');
        if (rolesItem) {
            rolesItem.classList.add('active');
            // Expand Roles submenu
            rolesItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'skills-monitor' || pageId === 'skills-management') {
        // Skills submenu item
        const skillsItem = document.querySelector('.nav-item[data-page="skills"]');
        if (skillsItem) {
            skillsItem.classList.add('active');
            // Expand Skills submenu
            skillsItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else {
        // Main menu item
        const navItem = document.querySelector(`.nav-item[data-page="${pageId}"]`);
        if (navItem) {
            navItem.classList.add('active');
        }
    }
}

// Toggle submenu
function toggleSubmenu(menuId) {
    const sidebar = document.getElementById('main-sidebar');
    const navItem = document.querySelector(`.nav-item[data-page="${menuId}"]`);
    
    if (!navItem) return;
    
    // Check if sidebar is collapsed
    if (sidebar && sidebar.classList.contains('collapsed')) {
        // Show popup menu when collapsed
        showSubmenuPopup(navItem, menuId);
    } else {
        // Normal toggle when expanded
        navItem.classList.toggle('expanded');
    }
}

// Show submenu popup
function showSubmenuPopup(navItem, menuId) {
    // Remove other open popup menus
    const existingPopup = document.querySelector('.submenu-popup');
    if (existingPopup) {
        existingPopup.remove();
        return; // already open, close on click
    }
    
    const navItemContent = navItem.querySelector('.nav-item-content');
    const submenu = navItem.querySelector('.nav-submenu');
    
    if (!submenu) return;
    
    // Get menu position
    const rect = navItemContent.getBoundingClientRect();
    
    // Create popup menu
    const popup = document.createElement('div');
    popup.className = 'submenu-popup';
    popup.style.position = 'fixed';
    popup.style.left = (rect.right + 8) + 'px';
    popup.style.top = rect.top + 'px';
    popup.style.zIndex = '1000';
    
    // Copy submenu items into popup
    const submenuItems = submenu.querySelectorAll('.nav-submenu-item');
    submenuItems.forEach(item => {
        const popupItem = document.createElement('div');
        popupItem.className = 'submenu-popup-item';
        popupItem.textContent = item.textContent.trim();
        
        // Check if current page is active
        const pageId = item.getAttribute('data-page');
        if (pageId && document.querySelector(`.nav-submenu-item[data-page="${pageId}"].active`)) {
            popupItem.classList.add('active');
        }
        
        popupItem.onclick = function(e) {
            e.stopPropagation();
            e.preventDefault();
            
            // Get page ID and switch
            const pageId = item.getAttribute('data-page');
            if (pageId) {
                switchPage(pageId);
            }
            
            // Close popup menu
            popup.remove();
            document.removeEventListener('click', closePopup);
        };
        popup.appendChild(popupItem);
    });
    
    document.body.appendChild(popup);
    
    // Click outside to close popup menu
    const closePopup = function(e) {
        if (!popup.contains(e.target) && !navItem.contains(e.target)) {
            popup.remove();
            document.removeEventListener('click', closePopup);
        }
    };
    
    // Delay adding event listener to avoid immediate trigger
    setTimeout(() => {
        document.addEventListener('click', closePopup);
    }, 0);
}

// Initialize page
function initPage(pageId) {
    switch(pageId) {
        case 'dashboard':
            if (typeof refreshDashboard === 'function') {
                refreshDashboard();
            }
            break;
        case 'chat':
            // Conversation page already initialized by chat.js
            break;
        case 'info-collect':
            // Information gathering page
            if (typeof initInfoCollectPage === 'function') {
                initInfoCollectPage();
            }
            break;
        case 'tasks':
            // Initialize task management page
            if (typeof initTasksPage === 'function') {
                initTasksPage();
            }
            break;
        case 'mcp-monitor':
            // Initialize monitoring panel
            if (typeof refreshMonitorPanel === 'function') {
                refreshMonitorPanel();
            }
            break;
        case 'mcp-management':
            // Initialize MCP management
            // Load external MCP list first (fast), then load the tool list
            if (typeof loadExternalMCPs === 'function') {
                loadExternalMCPs().catch(err => {
                    console.warn('Failed to load external MCP list:', err);
                });
            }
            // Load tool list (MCP tool config has moved to the MCP management page)
            // Use async loading to avoid blocking page rendering
            if (typeof loadToolsList === 'function') {
                // Ensure tool pagination settings are initialized
                if (typeof getToolsPageSize === 'function' && typeof toolsPagination !== 'undefined') {
                    toolsPagination.pageSize = getToolsPageSize();
                }
                // Delay loading to let the page render first
                setTimeout(() => {
                    loadToolsList(1, '').catch(err => {
                        console.error('Failed to load tool list:', err);
                    });
                }, 100);
            }
            break;
        case 'vulnerabilities':
            // Initialize vulnerability management page
            if (typeof initVulnerabilityPage === 'function') {
                initVulnerabilityPage();
            }
            break;
        case 'settings':
            // Initialize settings page (no tool list needed)
            if (typeof loadConfig === 'function') {
                loadConfig(false);
            }
            break;
        case 'roles-management':
            // Initialize role management page
            // Reset search UI (variables will auto-update on next search)
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
            // Initialize Skills Status monitor page
            if (typeof loadSkillsMonitor === 'function') {
                loadSkillsMonitor();
            }
            break;
        case 'skills-management':
            // Initialize Skills management page
            // Reset search UI (variables will auto-update on next search)
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
        case 'memory':
            // Initialize persistent memory panel
            if (typeof initMemoryPage === 'function') {
                initMemoryPage();
            }
            break;
    }

    // Clean up timers for other pages
    if (pageId !== 'tasks' && typeof cleanupTasksPage === 'function') {
        cleanupTasksPage();
    }
}

// Initialize routing after page load
document.addEventListener('DOMContentLoaded', function() {
    initRouter();
    initSidebarState();
    
    // Listen for hash changes
    window.addEventListener('hashchange', function() {
        const hash = window.location.hash.slice(1);
        // Handle hash with parameters (e.g., chat?conversation=xxx)
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        
        if (pageId && ['chat', 'info-collect', 'tasks', 'vulnerabilities', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'roles-management', 'skills-monitor', 'skills-management', 'settings', 'memory'].includes(pageId)) {
            switchPage(pageId);
            
            // If this is the chat page with a conversation param, load that conversation
            if (pageId === 'chat' && hashParts.length > 1) {
                const params = new URLSearchParams(hashParts[1]);
                const conversationId = params.get('conversation');
                if (conversationId) {
                    setTimeout(() => {
                        // Try multiple ways to call loadConversation
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
    
    // Also check hash parameters on page load
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

// Toggle sidebar collapse/expand
function toggleSidebar() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        sidebar.classList.toggle('collapsed');
        // Save collapsed state to localStorage
        const isCollapsed = sidebar.classList.contains('collapsed');
        localStorage.setItem('sidebarCollapsed', isCollapsed ? 'true' : 'false');
    }
}

// Initialize sidebar state
function initSidebarState() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        const savedState = localStorage.getItem('sidebarCollapsed');
        if (savedState === 'true') {
            sidebar.classList.add('collapsed');
        }
    }
}

// Export functions for use by other scripts
window.switchPage = switchPage;
window.toggleSubmenu = toggleSubmenu;
window.toggleSidebar = toggleSidebar;
window.currentPage = function() { return currentPage; };

