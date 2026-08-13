/**
 * WASM 环境补全模块
 * 为不同类型的 WASM（Emscripten、wasm-bindgen 等）提供最小运行环境
 */

/**
 * 补全 wasm-bindgen (Rust) 生成的 WASM 所需的环境
 * 常见于 Rust 编写的加密模块
 */
function patchWasmBindgen() {
    class Window {
        constructor() {
            this.document = {
                body: {},
                head: {},
                createElement: () => ({}),
                getElementById: () => null,
                querySelector: () => null,
            };
        }
    }
    
    const win = new Window();
    win.window = win;
    win.self = win;
    win.globalThis = win;
    
    globalThis.Window = Window;
    globalThis.window = win;
    globalThis.self = win;
    globalThis.document = win.document;
    
    // wasm-bindgen 可能使用 TextEncoder/TextDecoder
    if (typeof TextEncoder === 'undefined') {
        const { TextEncoder, TextDecoder } = require('util');
        globalThis.TextEncoder = TextEncoder;
        globalThis.TextDecoder = TextDecoder;
    }
    
    return win;
}

/**
 * 补全 Emscripten 生成的 WASM 所需的环境
 */
function patchEmscripten() {
    return {
        print: console.log,
        printErr: console.error,
        TOTAL_MEMORY: 16777216,
        TOTAL_STACK: 5242880,
        noInitialRun: true,
        noExitRuntime: true,
        onRuntimeInitialized: null,
        preRun: [],
        postRun: [],
    };
}

/**
 * 补全 Go WASM 所需的环境
 */
function patchGoWasm() {
    globalThis.process = globalThis.process || { argv: ['node'], env: {} };
    globalThis.fs = globalThis.fs || require('fs');
    
    // Go WASM 需要 performance.now
    if (typeof performance === 'undefined') {
        globalThis.performance = {
            now: () => Date.now(),
        };
    }
    
    // Go WASM 需要 crypto.getRandomValues
    if (typeof globalThis.crypto === 'undefined') {
        const nodeCrypto = require('crypto');
        globalThis.crypto = {
            getRandomValues: (arr) => {
                const bytes = nodeCrypto.randomBytes(arr.length);
                arr.set(bytes);
                return arr;
            },
        };
    }
}

/**
 * 清理环境补丁（测试完成后调用）
 */
function cleanupPatches() {
    delete globalThis.Window;
    delete globalThis.window;
    delete globalThis.self;
    delete globalThis.document;
}

module.exports = { patchWasmBindgen, patchEmscripten, patchGoWasm, cleanupPatches };
