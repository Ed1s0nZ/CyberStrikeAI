/**
 * 请求封装模块
 * 封装 HTTP 请求，处理 Headers、Cookie、重试、频率控制
 */

const axios = require('axios');

const DEFAULT_HEADERS = {
    'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'Accept': 'application/json, text/javascript, */*; q=0.01',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'Accept-Encoding': 'gzip, deflate, br',
    'Connection': 'keep-alive',
    'X-Requested-With': 'XMLHttpRequest',
};

class RequestClient {
    constructor(options = {}) {
        this.baseURL = options.baseURL || '';
        this.cookies = options.cookies || {};
        this.headers = { ...DEFAULT_HEADERS, ...options.headers };
        this.delay = options.delay || 1000;
        this.maxRetries = options.maxRetries || 3;
        this.referer = options.referer || '';

        this.client = axios.create({
            baseURL: this.baseURL,
            timeout: options.timeout || 30000,
            headers: this.headers,
            maxRedirects: 5,
        });
    }

    getCookieString() {
        return Object.entries(this.cookies)
            .map(([k, v]) => `${k}=${v}`)
            .join('; ');
    }

    setCookie(name, value) {
        this.cookies[name] = value;
    }

    parseCookies(setCookieHeader) {
        if (!setCookieHeader) return;
        const headers = Array.isArray(setCookieHeader) ? setCookieHeader : [setCookieHeader];
        headers.forEach(h => {
            const mainPart = h.split(';')[0];
            const eqIdx = mainPart.indexOf('=');
            if (eqIdx > 0) {
                this.cookies[mainPart.substring(0, eqIdx).trim()] = mainPart.substring(eqIdx + 1).trim();
            }
        });
    }

    async request(config) {
        const finalConfig = {
            ...config,
            headers: {
                ...this.headers,
                ...config.headers,
                'Cookie': this.getCookieString(),
            },
        };

        if (this.referer) {
            finalConfig.headers['Referer'] = this.referer;
        }

        let lastError;
        for (let attempt = 1; attempt <= this.maxRetries; attempt++) {
            try {
                const response = await this.client.request(finalConfig);
                this.parseCookies(response.headers['set-cookie']);
                return response;
            } catch (error) {
                lastError = error;
                const status = error.response?.status;
                console.error(`[Request] 第 ${attempt}/${this.maxRetries} 次请求失败: ${status || error.message}`);

                if (status === 429) {
                    const waitTime = this.delay * attempt * 2;
                    console.log(`[Request] 频率限制，等待 ${waitTime}ms...`);
                    await this.sleep(waitTime);
                } else if (status >= 500) {
                    await this.sleep(this.delay * attempt);
                } else if (status === 403 || status === 412) {
                    console.error('[Request] 访问被拒绝，可能需要更新 Cookie 或检查加密参数');
                    throw error;
                } else {
                    throw error;
                }
            }
        }
        throw lastError;
    }

    async get(url, params = {}, extraHeaders = {}) {
        return this.request({
            method: 'GET',
            url,
            params,
            headers: extraHeaders,
        });
    }

    async post(url, data = {}, extraHeaders = {}) {
        return this.request({
            method: 'POST',
            url,
            data,
            headers: extraHeaders,
        });
    }

    sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    async requestWithDelay(config) {
        const response = await this.request(config);
        const jitter = Math.random() * this.delay * 0.5;
        await this.sleep(this.delay + jitter);
        return response;
    }
}

module.exports = { RequestClient, DEFAULT_HEADERS };
