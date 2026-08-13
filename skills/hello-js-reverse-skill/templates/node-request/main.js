/**
 * 主脚本模板 - 纯 Node.js 算法还原
 * 
 * 适用场景：加密逻辑可完整提取，无浏览器环境依赖
 * 
 * TODO: 根据逆向分析结果修改以下配置和逻辑
 */

const { generateM, generateSign } = require('./utils/encrypt');
const { RequestClient } = require('./utils/request');

// ============ 配置区域 ============

const CONFIG = {
    // 目标信息
    name: '示例项目',
    description: '采集全部5页数据，计算数值总和',
    
    // 接口配置
    baseURL: 'https://target.com',
    apiPath: '/api/data',
    referer: 'https://target.com/page',
    
    // 采集配置
    totalPages: 5,
    delay: 1500,
    
    // 认证信息
    cookies: {
        // sessionid: 'your_session_id',
    },
    
    // 自定义请求头
    headers: {
        // 'X-Custom-Header': 'value',
    },
};

// ============ 主逻辑 ============

async function fetchPage(client, page) {
    // TODO: 根据实际接口修改参数构造逻辑
    const m = generateM(page);
    const timestamp = Math.floor(Date.now() / 1000);
    
    const params = {
        page,
        m,
        t: timestamp,
    };
    
    const response = await client.requestWithDelay({
        method: 'GET',
        url: CONFIG.apiPath,
        params,
    });
    
    return response.data;
}

function extractData(responseData) {
    // TODO: 根据实际响应格式修改数据提取逻辑
    if (responseData && responseData.data) {
        return responseData.data;
    }
    return [];
}

function calculateResult(allData) {
    // TODO: 根据题目要求修改计算逻辑
    // 示例：求所有数值的总和
    const sum = allData.reduce((acc, item) => {
        const value = typeof item === 'number' ? item : (item.value || 0);
        return acc + value;
    }, 0);
    return sum;
}

async function main() {
    console.log(`[*] 项目：${CONFIG.name}`);
    console.log(`[*] 目标：${CONFIG.description}`);
    console.log('');
    
    const client = new RequestClient({
        baseURL: CONFIG.baseURL,
        cookies: CONFIG.cookies,
        headers: CONFIG.headers,
        referer: CONFIG.referer,
        delay: CONFIG.delay,
    });
    
    const allData = [];
    
    for (let page = 1; page <= CONFIG.totalPages; page++) {
        try {
            const responseData = await fetchPage(client, page);
            const pageData = extractData(responseData);
            allData.push(...pageData);
            console.log(`[+] 正在采集第 ${page}/${CONFIG.totalPages} 页... ✓ 获取 ${pageData.length} 条数据`);
        } catch (error) {
            console.error(`[-] 第 ${page} 页采集失败: ${error.message}`);
            if (error.response) {
                console.error(`    状态码: ${error.response.status}`);
                console.error(`    响应: ${JSON.stringify(error.response.data).substring(0, 200)}`);
            }
        }
    }
    
    console.log(`\n[+] 采集完成，共 ${allData.length} 条数据`);
    
    const result = calculateResult(allData);
    
    console.log('\n' + '='.repeat(30) + ' 计算结果 ' + '='.repeat(30));
    console.log(`答案：${result}`);
    console.log('='.repeat(70));
}

main().catch(error => {
    console.error('\n[!] 运行错误:', error.message);
    process.exit(1);
});
