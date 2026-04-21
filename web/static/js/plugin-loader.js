// plugin-loader.js - Loads plugin frontend assets (pages, JS, CSS, i18n)

(function() {
    'use strict';

    window.CyberStrikePlugins = {
        loaded: {},

        // Load all enabled plugins' frontend assets
        async loadAll() {
            try {
                var resp = await apiFetch('/api/plugins');
                var data = await resp.json();
                if (!data.plugins) return;

                for (var i = 0; i < data.plugins.length; i++) {
                    var plugin = data.plugins[i];
                    if (plugin.enabled && plugin.manifest.frontend) {
                        await this.loadPlugin(plugin);
                    }
                }
            } catch (err) {
                console.warn('Plugin loader:', err.message);
            }
        },

        async loadPlugin(plugin) {
            var name = plugin.manifest.name;
            var fe = plugin.manifest.frontend;
            if (!fe || this.loaded[name]) return;

            // Load CSS
            if (fe.styles) {
                for (var i = 0; i < fe.styles.length; i++) {
                    var link = document.createElement('link');
                    link.rel = 'stylesheet';
                    link.href = '/api/plugins/' + name + '/web/css/' + fe.styles[i];
                    document.head.appendChild(link);
                }
            }

            // Load i18n
            await this.loadI18n(name);

            // Add nav items
            if (fe.nav_items) {
                for (var j = 0; j < fe.nav_items.length; j++) {
                    this.addNavItem(name, fe.nav_items[j]);
                }
            }

            // Load page HTML
            if (fe.pages) {
                for (var k = 0; k < fe.pages.length; k++) {
                    await this.loadPage(name, fe.pages[k]);
                }
            }

            // Load JS (after pages are in DOM)
            if (fe.scripts) {
                for (var l = 0; l < fe.scripts.length; l++) {
                    await this.loadScript(name, fe.scripts[l]);
                }
            }

            this.loaded[name] = true;
        },

        async loadI18n(name) {
            // Get current language
            var lang = (typeof i18next !== 'undefined' && i18next.language) || 'en-US';
            try {
                var resp = await fetch('/api/plugins/' + name + '/i18n/' + lang);
                if (resp.ok) {
                    var translations = await resp.json();
                    if (typeof i18next !== 'undefined' && i18next.addResourceBundle) {
                        i18next.addResourceBundle(lang, 'translation', translations, true, true);
                    }
                }
            } catch (e) {
                // i18n optional
            }
        },

        addNavItem(pluginName, nav) {
            var sidebar = document.querySelector('.sidebar-nav');
            if (!sidebar) return;

            // Insert before settings
            var settingsItem = sidebar.querySelector('[data-page="settings"]');
            if (!settingsItem) return;

            var item = document.createElement('div');
            item.className = 'nav-item';
            item.setAttribute('data-page', nav.id);
            item.onclick = function() { switchPage(nav.id); };
            item.innerHTML =
                '<div class="nav-item-content">' +
                    '<span class="nav-icon">' + (nav.icon || '') + '</span>' +
                    '<span class="nav-text" ' + (nav.i18n ? 'data-i18n="' + nav.i18n + '"' : '') + '>' + nav.label + '</span>' +
                '</div>';
            settingsItem.parentNode.insertBefore(item, settingsItem);
        },

        async loadPage(pluginName, pageFile) {
            try {
                var resp = await fetch('/api/plugins/' + pluginName + '/web/pages/' + pageFile);
                if (!resp.ok) return;
                var html = await resp.text();

                // Extract page ID from filename (my-page.html -> page-my-page)
                var pageId = 'page-' + pageFile.replace('.html', '');

                // Create page container
                var pageDiv = document.createElement('div');
                pageDiv.id = pageId;
                pageDiv.className = 'page';
                pageDiv.innerHTML = html;

                // Append to main content area
                var mainContent = document.querySelector('.main-content');
                if (mainContent) {
                    mainContent.appendChild(pageDiv);
                }
            } catch (e) {
                console.warn('Plugin ' + pluginName + ': failed to load page ' + pageFile, e);
            }
        },

        async loadScript(pluginName, jsFile) {
            return new Promise(function(resolve, reject) {
                var script = document.createElement('script');
                script.src = '/api/plugins/' + pluginName + '/web/js/' + jsFile;
                script.onload = resolve;
                script.onerror = reject;
                document.body.appendChild(script);
            });
        }
    };

    // Auto-load on page ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() { CyberStrikePlugins.loadAll(); });
    } else {
        // Small delay to let auth happen first
        setTimeout(function() { CyberStrikePlugins.loadAll(); }, 1000);
    }
})();
