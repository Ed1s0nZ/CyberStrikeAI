/**
 * 主脚本模板 - 浏览器自动化
 * 
 * 适用场景：TLS 指纹检测、复杂环境依赖、window 蜜罐
 * 使用 Playwright 控制真实 Chrome 浏览器执行请求
 * 
 * TODO: 根据逆向分析结果修改配置和逻辑
 */

const { chromium } = require('playwright-core');

// ============ 配置区域 ============

const CONFIG = {
    name: '示例项目 - 浏览器自动化',
    description: '采集全部5页数据',
    
    // 目标URL
    pageURL: 'https://target.com/page',
    
    // Chrome 路径（macOS 默认路径）
    chromePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    // Linux: '/usr/bin/google-chrome'
    // Windows: 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'
    
    // 是否显示浏览器
    headless: false,
    
    totalPages: 5,
    delay: 2000,
    
    // 登录 Cookie
    cookies: [
        // { name: 'sessionid', value: 'xxx', domain: 'target.com', path: '/' },
    ],
    
    // 注入脚本（可选）
    injectScript: null,
};

// ============ 主逻辑 ============

async function main() {
    console.log(`[*] 项目：${CONFIG.name}`);
    console.log(`[*] 目标：${CONFIG.description}`);
    console.log('');
    
    // 启动浏览器
    console.log('[*] 启动浏览器...');
    const browser = await chromium.launch({
        executablePath: CONFIG.chromePath,
        headless: CONFIG.headless,
        args: [
            '--disable-blink-features=AutomationControlled',
            '--no-sandbox',
        ],
    });
    
    const context = await browser.newContext({
        userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        viewport: { width: 1920, height: 1080 },
        locale: 'zh-CN',
    });
    
    // 设置 Cookie
    if (CONFIG.cookies.length > 0) {
        await context.addCookies(CONFIG.cookies);
    }
    
    // 隐藏自动化特征
    await context.addInitScript(() => {
        Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
        window.chrome = { runtime: {} };
    });
    
    // 注入自定义脚本
    if (CONFIG.injectScript) {
        await context.addInitScript(CONFIG.injectScript);
    }
    
    const page = await context.newPage();
    
    try {
        // 导航到目标页面
        console.log('[*] 打开目标页面...');
        await page.goto(CONFIG.pageURL, { waitUntil: 'networkidle', timeout: 30000 });
        console.log('[+] 页面加载完成');
        
        // 采集数据
        const allData = [];
        
        for (let pageNum = 1; pageNum <= CONFIG.totalPages; pageNum++) {
            // TODO: 根据实际页面结构修改数据提取逻辑
            const pageData = await extractPageData(page, pageNum);
            allData.push(...pageData);
            console.log(`[+] 正在采集第 ${pageNum}/${CONFIG.totalPages} 页... ✓ 获取 ${pageData.length} 条数据`);
            
            if (pageNum < CONFIG.totalPages) {
                // 翻页
                await goToNextPage(page, pageNum + 1);
                await sleep(CONFIG.delay);
            }
        }
        
        console.log(`\n[+] 采集完成，共 ${allData.length} 条数据`);
        
        // 计算结果
        const result = allData.reduce((sum, val) => sum + val, 0);
        
        console.log('\n' + '='.repeat(30) + ' 计算结果 ' + '='.repeat(30));
        console.log(`答案：${result}`);
        console.log('='.repeat(70));
        
    } catch (error) {
        console.error('[!] 错误:', error.message);
        await page.screenshot({ path: 'error_screenshot.png' });
        console.log('[*] 已保存错误截图: error_screenshot.png');
    } finally {
        await browser.close();
    }
}

/**
 * 从页面提取数据
 * TODO: 根据实际页面结构修改
 */
async function extractPageData(page, pageNum) {
    // 方式1：从页面DOM提取
    const data = await page.evaluate(() => {
        const rows = document.querySelectorAll('.data-row');
        return Array.from(rows).map(row => {
            const value = row.querySelector('.value')?.textContent?.trim();
            return parseFloat(value) || 0;
        });
    });
    
    return data;
    
    // 方式2：拦截网络响应
    // const responsePromise = page.waitForResponse(resp => resp.url().includes('/api/data'));
    // // 触发数据加载...
    // const response = await responsePromise;
    // const json = await response.json();
    // return json.data;
}

/**
 * 翻到下一页
 * TODO: 根据实际页面结构修改翻页逻辑
 */
async function goToNextPage(page, targetPage) {
    // 方式1：点击翻页按钮
    await page.click(`.pagination a:has-text("${targetPage}")`);
    await page.waitForLoadState('networkidle');
    
    // 方式2：等待特定元素出现
    // await page.waitForSelector('.data-loaded');
    
    // 方式3：直接导航到带页码的URL
    // await page.goto(`${CONFIG.pageURL}?page=${targetPage}`, { waitUntil: 'networkidle' });
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

main().catch(error => {
    console.error('\n[!] 运行错误:', error.message);
    process.exit(1);
});
