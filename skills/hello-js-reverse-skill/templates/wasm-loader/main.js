/**
 * 主脚本模板 - WASM 加载还原
 * 
 * 适用场景：加密函数通过 WebAssembly 实现
 * 流程：
 *   1. 下载/加载 .wasm 文件
 *   2. 补全运行环境
 *   3. 调用 WASM 导出的加密函数
 *   4. 使用加密结果构造请求
 * 
 * TODO: 根据逆向分析结果修改配置和逻辑
 */

const { loadWasmFromFile, loadWasmFromURL, analyzeWasm } = require('./utils/wasm-loader');
const { patchWasmBindgen } = require('./utils/env-patch');
const axios = require('axios');
const path = require('path');

// ============ 配置区域 ============

const CONFIG = {
    name: '示例项目 - WASM加密',
    description: '采集全部5页数据',
    
    baseURL: 'https://target.com',
    dataEndpoint: '/api/data',
    
    // WASM 文件路径或 URL
    wasmSource: './main.wasm',    // 本地文件
    // wasmSource: 'https://target.com/main.wasm',  // 远程 URL
    
    // WASM 导出函数名
    encryptFunction: 'encode',
    
    // WASM 类型: 'plain' | 'emscripten' | 'wasm-bindgen' | 'go'
    wasmType: 'plain',
    
    totalPages: 5,
    delay: 1500,
    
    cookies: {},
    headers: {
        'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
    },
};

// ============ 主逻辑 ============

let wasmExports = null;

async function initWasm() {
    console.log('[*] 加载 WASM 模块...');
    
    // 根据 WASM 类型补全环境
    if (CONFIG.wasmType === 'wasm-bindgen') {
        patchWasmBindgen();
    }
    
    // 分析 WASM 结构
    if (CONFIG.wasmSource.startsWith('http')) {
        console.log('[*] 从远程下载 WASM...');
        const result = await loadWasmFromURL(CONFIG.wasmSource);
        wasmExports = result.exports;
        console.log('[+] WASM 导出函数:', Object.keys(result.exports).filter(k => typeof result.exports[k] === 'function'));
    } else {
        const wasmPath = path.resolve(CONFIG.wasmSource);
        
        // 先分析 WASM 结构
        const analysis = await analyzeWasm(wasmPath);
        console.log('[*] WASM 导出:', analysis.exports.map(e => `${e.name}(${e.kind})`).join(', '));
        console.log('[*] WASM 导入:', analysis.imports.map(i => `${i.module}.${i.name}`).join(', '));
        
        const result = await loadWasmFromFile(wasmPath);
        wasmExports = result.exports;
    }
    
    // 验证加密函数存在
    if (typeof wasmExports[CONFIG.encryptFunction] !== 'function') {
        throw new Error(`WASM 导出函数 "${CONFIG.encryptFunction}" 不存在。可用函数: ${Object.keys(wasmExports).filter(k => typeof wasmExports[k] === 'function').join(', ')}`);
    }
    
    console.log(`[+] WASM 加载成功，加密函数: ${CONFIG.encryptFunction}`);
}

function generateEncryptedParam(page) {
    // TODO: 根据逆向分析修改参数生成逻辑
    const timestamp = Math.floor(Date.now() / 1000);
    
    // 调用 WASM 导出的加密函数
    const encrypted = wasmExports[CONFIG.encryptFunction](timestamp, page);
    
    // TODO: 根据实际格式组装参数
    return `${encrypted}|${timestamp}|${page}`;
}

async function fetchPage(page) {
    const m = generateEncryptedParam(page);
    
    const response = await axios.get(`${CONFIG.baseURL}${CONFIG.dataEndpoint}`, {
        params: { page, m },
        headers: {
            ...CONFIG.headers,
            Cookie: Object.entries(CONFIG.cookies).map(([k, v]) => `${k}=${v}`).join('; '),
            Referer: CONFIG.baseURL,
        },
    });
    
    return response.data;
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function main() {
    console.log(`[*] 项目：${CONFIG.name}`);
    console.log(`[*] 目标：${CONFIG.description}`);
    console.log('');
    
    // 初始化 WASM
    await initWasm();
    console.log('');
    
    // 验证：测试加密函数
    const testResult = generateEncryptedParam(1);
    console.log(`[*] 加密测试: page=1 → m=${testResult}`);
    console.log('');
    
    // 采集数据
    const allData = [];
    
    for (let page = 1; page <= CONFIG.totalPages; page++) {
        try {
            const responseData = await fetchPage(page);
            const pageData = responseData.data || [];
            allData.push(...pageData);
            console.log(`[+] 正在采集第 ${page}/${CONFIG.totalPages} 页... ✓ 获取 ${pageData.length} 条数据`);
            
            if (page < CONFIG.totalPages) {
                await sleep(CONFIG.delay + Math.random() * 500);
            }
        } catch (error) {
            console.error(`[-] 第 ${page} 页采集失败: ${error.message}`);
        }
    }
    
    console.log(`\n[+] 采集完成，共 ${allData.length} 条数据`);
    
    // TODO: 计算结果
    const result = allData.reduce((sum, item) => sum + (item.value || 0), 0);
    
    console.log('\n' + '='.repeat(30) + ' 计算结果 ' + '='.repeat(30));
    console.log(`答案：${result}`);
    console.log('='.repeat(70));
}

main().catch(error => {
    console.error('\n[!] 运行错误:', error.message);
    process.exit(1);
});
