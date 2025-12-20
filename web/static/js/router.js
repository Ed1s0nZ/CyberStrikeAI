// 页面路由管理
let currentPage = 'chat';

// 初始化路由
function initRouter() {
    // 默认显示对话页面
    switchPage('chat');
    
    // 从URL hash读取页面（如果有）
    const hash = window.location.hash.slice(1);
    if (hash && ['chat', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'settings'].includes(hash)) {
        switchPage(hash);
    }
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
        case 'chat':
            // 对话页面已由chat.js初始化
            break;
        case 'mcp-monitor':
            // 初始化监控面板
            if (typeof refreshMonitorPanel === 'function') {
                refreshMonitorPanel();
            }
            break;
        case 'mcp-management':
            // 初始化MCP管理
            if (typeof loadExternalMCPs === 'function') {
                loadExternalMCPs();
            }
            // 加载工具列表（MCP工具配置已移到MCP管理页面）
            if (typeof loadToolsList === 'function') {
                // 确保工具分页设置已初始化
                if (typeof getToolsPageSize === 'function' && typeof toolsPagination !== 'undefined') {
                    toolsPagination.pageSize = getToolsPageSize();
                }
                loadToolsList(1, '');
            }
            break;
        case 'settings':
            // 初始化设置页面（不需要加载工具列表）
            if (typeof loadConfig === 'function') {
                loadConfig(false);
            }
            break;
    }
}

// 页面加载完成后初始化路由
document.addEventListener('DOMContentLoaded', function() {
    initRouter();
    initSidebarState();
    
    // 监听hash变化
    window.addEventListener('hashchange', function() {
        const hash = window.location.hash.slice(1);
        if (hash && ['chat', 'mcp-monitor', 'mcp-management', 'knowledge-management', 'knowledge-retrieval-logs', 'settings'].includes(hash)) {
            switchPage(hash);
        }
    });
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

