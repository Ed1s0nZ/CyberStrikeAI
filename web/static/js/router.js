// 页面路由管理
let currentPage = 'dashboard';

// 初始化路由
function initRouter() {
    // 从URL hash读取页面（如果有）
    const hash = window.location.hash.slice(1);
    if (hash) {
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        if (pageId && ['dashboard', 'chat', 'info-collect', 'vulnerabilities', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'roles-management', 'skills-monitor', 'skills-management', 'settings', 'tasks'].includes(pageId)) {
            switchPage(pageId);
            
            // 如果是chat页面且带有conversation参数，加载对应对话
            if (pageId === 'chat' && hashParts.length > 1) {
                const params = new URLSearchParams(hashParts[1]);
                const conversationId = params.get('conversation');
                if (conversationId) {
                    setTimeout(() => {
                        // 尝试多种方式调用loadConversation
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
    
    // 默认显示仪表盘
    switchPage('dashboard');
}

// 切换页面
function switchPage(pageId) {
    // 隐藏所有页面
    document.querySelectorAll('.page').forEach(page => {
        page.classList.remove('active');
    });
    
    // 显示目标页面
    const targetPage = document.getElementById(`page-${pageId}`);
    if (targetPage) {
        targetPage.classList.add('active');
        currentPage = pageId;
        
        // 更新URL hash
        window.location.hash = pageId;
        
        // 更新导航状态
        updateNavState(pageId);
        
        // 页面特定的初始化
        initPage(pageId);
    }
}

// 更新导航状态
function updateNavState(pageId) {
    // 移除所有活动状态
    document.querySelectorAll('.nav-item').forEach(item => {
        item.classList.remove('active');
    });
    
    document.querySelectorAll('.nav-submenu-item').forEach(item => {
        item.classList.remove('active');
    });
    
    // 设置活动状态
    if (pageId === 'mcp-monitor' || pageId === 'mcp-management') {
        // MCP子菜单项
        const mcpItem = document.querySelector('.nav-item[data-page="mcp"]');
        if (mcpItem) {
            mcpItem.classList.add('active');
            // 展开MCP子菜单
            mcpItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'knowledge-management' || pageId === 'knowledge-retrieval-logs') {
        // 知识子菜单项
        const knowledgeItem = document.querySelector('.nav-item[data-page="knowledge"]');
        if (knowledgeItem) {
            knowledgeItem.classList.add('active');
            // 展开知识子菜单
            knowledgeItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'skills-monitor' || pageId === 'skills-management') {
        // Skills子菜单项
        const skillsItem = document.querySelector('.nav-item[data-page="skills"]');
        if (skillsItem) {
            skillsItem.classList.add('active');
            // 展开Skills子菜单
            skillsItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'roles-management') {
        // 角色子菜单项
        const rolesItem = document.querySelector('.nav-item[data-page="roles"]');
        if (rolesItem) {
            rolesItem.classList.add('active');
            // 展开角色子菜单
            rolesItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else if (pageId === 'skills-monitor' || pageId === 'skills-management') {
        // Skills子菜单项
        const skillsItem = document.querySelector('.nav-item[data-page="skills"]');
        if (skillsItem) {
            skillsItem.classList.add('active');
            // 展开Skills子菜单
            skillsItem.classList.add('expanded');
        }
        
        const submenuItem = document.querySelector(`.nav-submenu-item[data-page="${pageId}"]`);
        if (submenuItem) {
            submenuItem.classList.add('active');
        }
    } else {
        // 主菜单项
        const navItem = document.querySelector(`.nav-item[data-page="${pageId}"]`);
        if (navItem) {
            navItem.classList.add('active');
        }
    }
}

// 切换子菜单
function toggleSubmenu(menuId) {
    const sidebar = document.getElementById('main-sidebar');
    const navItem = document.querySelector(`.nav-item[data-page="${menuId}"]`);
    
    if (!navItem) return;
    
    // 检查侧边栏是否折叠
    if (sidebar && sidebar.classList.contains('collapsed')) {
        // 折叠状态下显示弹出菜单
        showSubmenuPopup(navItem, menuId);
    } else {
        // 展开状态下正常切换子菜单
        navItem.classList.toggle('expanded');
    }
}

// 显示子菜单弹出框
function showSubmenuPopup(navItem, menuId) {
    // 移除其他已打开的弹出菜单
    const existingPopup = document.querySelector('.submenu-popup');
    if (existingPopup) {
        existingPopup.remove();
        return; // 如果已经打开，点击时关闭
    }
    
    const navItemContent = navItem.querySelector('.nav-item-content');
    const submenu = navItem.querySelector('.nav-submenu');
    
    if (!submenu) return;
    
    // 获取菜单位置
    const rect = navItemContent.getBoundingClientRect();
    
    // 创建弹出菜单
    const popup = document.createElement('div');
    popup.className = 'submenu-popup';
    popup.style.position = 'fixed';
    popup.style.left = (rect.right + 8) + 'px';
    popup.style.top = rect.top + 'px';
    popup.style.zIndex = '1000';
    
    // 复制子菜单项到弹出菜单
    const submenuItems = submenu.querySelectorAll('.nav-submenu-item');
    submenuItems.forEach(item => {
        const popupItem = document.createElement('div');
        popupItem.className = 'submenu-popup-item';
        popupItem.textContent = item.textContent.trim();
        
        // 检查是否是当前激活的页面
        const pageId = item.getAttribute('data-page');
        if (pageId && document.querySelector(`.nav-submenu-item[data-page="${pageId}"].active`)) {
            popupItem.classList.add('active');
        }
        
        popupItem.onclick = function(e) {
            e.stopPropagation();
            e.preventDefault();
            
            // 获取页面ID并切换
            const pageId = item.getAttribute('data-page');
            if (pageId) {
                switchPage(pageId);
            }
            
            // 关闭弹出菜单
            popup.remove();
            document.removeEventListener('click', closePopup);
        };
        popup.appendChild(popupItem);
    });
    
    document.body.appendChild(popup);
    
    // 点击外部关闭弹出菜单
    const closePopup = function(e) {
        if (!popup.contains(e.target) && !navItem.contains(e.target)) {
            popup.remove();
            document.removeEventListener('click', closePopup);
        }
    };
    
    // 延迟添加事件监听，避免立即触发
    setTimeout(() => {
        document.addEventListener('click', closePopup);
    }, 0);
}

// 初始化页面
function initPage(pageId) {
    switch(pageId) {
        case 'dashboard':
            if (typeof refreshDashboard === 'function') {
                refreshDashboard();
            }
            break;
        case 'chat':
            // 对话页面已由chat.js初始化
            break;
        case 'info-collect':
            // 信息收集页面
            if (typeof initInfoCollectPage === 'function') {
                initInfoCollectPage();
            }
            break;
        case 'tasks':
            // 初始化任务管理页面
            if (typeof initTasksPage === 'function') {
                initTasksPage();
            }
            break;
        case 'mcp-monitor':
            // 初始化监控面板
            if (typeof refreshMonitorPanel === 'function') {
                refreshMonitorPanel();
            }
            break;
        case 'mcp-management':
            // 初始化MCP管理
            // 先加载外部MCP列表（快速），然后加载工具列表
            if (typeof loadExternalMCPs === 'function') {
                loadExternalMCPs().catch(err => {
                    console.warn('加载外部MCP列表失败:', err);
                });
            }
            // 加载工具列表（MCP工具配置已移到MCP管理页面）
            // 使用异步加载，避免阻塞页面渲染
            if (typeof loadToolsList === 'function') {
                // 确保工具分页设置已初始化
                if (typeof getToolsPageSize === 'function' && typeof toolsPagination !== 'undefined') {
                    toolsPagination.pageSize = getToolsPageSize();
                }
                // 延迟加载，让页面先渲染
                setTimeout(() => {
                    loadToolsList(1, '').catch(err => {
                        console.error('加载工具列表失败:', err);
                    });
                }, 100);
            }
            break;
        case 'vulnerabilities':
            // 初始化漏洞管理页面
            if (typeof initVulnerabilityPage === 'function') {
                initVulnerabilityPage();
            }
            break;
        case 'settings':
            // 初始化设置页面（不需要加载工具列表）
            if (typeof loadConfig === 'function') {
                loadConfig(false);
            }
            break;
        case 'roles-management':
            // 初始化角色管理页面
            // 重置搜索UI（变量会在下次搜索时自动更新）
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
            // 初始化Skills状态监控页面
            if (typeof loadSkillsMonitor === 'function') {
                loadSkillsMonitor();
            }
            break;
        case 'skills-management':
            // 初始化Skills管理页面
            // 重置搜索UI（变量会在下次搜索时自动更新）
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
    }
    
    // 清理其他页面的定时器
    if (pageId !== 'tasks' && typeof cleanupTasksPage === 'function') {
        cleanupTasksPage();
    }
}

// 页面加载完成后初始化路由
document.addEventListener('DOMContentLoaded', function() {
    initRouter();
    initSidebarState();
    
    // 监听hash变化
    window.addEventListener('hashchange', function() {
        const hash = window.location.hash.slice(1);
        // 处理带参数的hash（如 chat?conversation=xxx）
        const hashParts = hash.split('?');
        const pageId = hashParts[0];
        
        if (pageId && ['chat', 'info-collect', 'tasks', 'vulnerabilities', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'roles-management', 'skills-monitor', 'skills-management', 'settings'].includes(pageId)) {
            switchPage(pageId);
            
            // 如果是chat页面且带有conversation参数，加载对应对话
            if (pageId === 'chat' && hashParts.length > 1) {
                const params = new URLSearchParams(hashParts[1]);
                const conversationId = params.get('conversation');
                if (conversationId) {
                    setTimeout(() => {
                        // 尝试多种方式调用loadConversation
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
    
    // 页面加载时也检查hash参数
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

// 切换侧边栏折叠/展开
function toggleSidebar() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        sidebar.classList.toggle('collapsed');
        // 保存折叠状态到localStorage
        const isCollapsed = sidebar.classList.contains('collapsed');
        localStorage.setItem('sidebarCollapsed', isCollapsed ? 'true' : 'false');
    }
}

// 初始化侧边栏状态
function initSidebarState() {
    const sidebar = document.getElementById('main-sidebar');
    if (sidebar) {
        const savedState = localStorage.getItem('sidebarCollapsed');
        if (savedState === 'true') {
            sidebar.classList.add('collapsed');
        }
    }
}

// 导出函数供其他脚本使用
window.switchPage = switchPage;
window.toggleSubmenu = toggleSubmenu;
window.toggleSidebar = toggleSidebar;
window.currentPage = function() { return currentPage; };

