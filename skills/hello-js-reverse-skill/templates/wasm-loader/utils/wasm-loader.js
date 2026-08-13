/**
 * WASM 加载与执行模块
 * 在 Node.js 中加载 WebAssembly 模块并调用导出函数
 */

const fs = require('fs');
const path = require('path');

/**
 * 加载本地 WASM 文件
 * @param {string} wasmPath - WASM 文件路径
 * @param {Object} importObject - 导入对象（环境依赖）
 * @returns {Object} WASM 实例的 exports
 */
async function loadWasmFromFile(wasmPath, importObject = {}) {
    const wasmBuffer = fs.readFileSync(wasmPath);
    return loadWasm(wasmBuffer, importObject);
}

/**
 * 从 URL 下载并加载 WASM
 * @param {string} url - WASM 文件 URL
 * @param {Object} importObject - 导入对象
 * @returns {Object} WASM 实例的 exports
 */
async function loadWasmFromURL(url, importObject = {}) {
    const axios = require('axios');
    const response = await axios.get(url, { responseType: 'arraybuffer' });
    return loadWasm(response.data, importObject);
}

/**
 * 从二进制数据加载 WASM
 */
async function loadWasm(wasmBuffer, importObject = {}) {
    const defaultImports = {
        env: {
            memory: new WebAssembly.Memory({ initial: 256, maximum: 512 }),
            table: new WebAssembly.Table({ initial: 0, element: 'anyfunc' }),
            abort: (msg, file, line, col) => {
                console.error(`WASM abort at ${file}:${line}:${col}: ${msg}`);
            },
            __memory_base: 0,
            __table_base: 0,
            ...importObject.env,
        },
        wasi_snapshot_preview1: {
            fd_write: () => 0,
            fd_read: () => 0,
            fd_close: () => 0,
            fd_seek: () => 0,
            fd_fdstat_get: () => 0,
            environ_sizes_get: () => 0,
            environ_get: () => 0,
            proc_exit: (code) => { throw new Error(`WASM proc_exit(${code})`); },
            clock_time_get: () => BigInt(Date.now()) * 1000000n,
            ...importObject.wasi_snapshot_preview1,
        },
        ...importObject,
    };
    
    // 检查 WASM 需要哪些导入
    const module = await WebAssembly.compile(wasmBuffer);
    const requiredImports = WebAssembly.Module.imports(module);
    
    // 确保所有需要的导入都存在
    for (const imp of requiredImports) {
        if (!defaultImports[imp.module]) {
            defaultImports[imp.module] = {};
        }
        if (!defaultImports[imp.module][imp.name]) {
            switch (imp.kind) {
                case 'function':
                    defaultImports[imp.module][imp.name] = () => {};
                    break;
                case 'global':
                    defaultImports[imp.module][imp.name] = new WebAssembly.Global({ value: 'i32', mutable: true }, 0);
                    break;
                case 'memory':
                    defaultImports[imp.module][imp.name] = new WebAssembly.Memory({ initial: 256 });
                    break;
                case 'table':
                    defaultImports[imp.module][imp.name] = new WebAssembly.Table({ initial: 0, element: 'anyfunc' });
                    break;
            }
        }
    }
    
    const instance = await WebAssembly.instantiate(module, defaultImports);
    
    return {
        exports: instance.exports,
        module,
        instance,
        imports: requiredImports,
    };
}

/**
 * 分析 WASM 模块的导出和导入
 */
async function analyzeWasm(wasmPath) {
    const wasmBuffer = fs.readFileSync(wasmPath);
    const module = await WebAssembly.compile(wasmBuffer);
    
    const exports = WebAssembly.Module.exports(module);
    const imports = WebAssembly.Module.imports(module);
    
    return {
        exports: exports.map(e => ({ name: e.name, kind: e.kind })),
        imports: imports.map(i => ({ module: i.module, name: i.name, kind: i.kind })),
    };
}

module.exports = { loadWasmFromFile, loadWasmFromURL, loadWasm, analyzeWasm };
