/**
 * 请求封装模块（VM沙箱版本）
 * 支持两阶段请求：先获取动态JS生成Cookie，再携带Cookie请求数据
 */

const axios = require('axios');

const DEFAULT_HEADERS = {
    'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    'Accept': 'application/json, text/javascript, */*; q=0.01',
    'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
    'X-Requested-With': 'XMLHttpRequest',
};

class TwoPhaseClient {
    constructor(options = {}) {
        this.baseURL = options.baseURL || '';
        this.cookies = options.cookies || {};
        this.headers = { ...DEFAULT_HEADERS, ...options.headers };
        this.delay = options.delay || 1500;

        this.client = axios.create({
            baseURL: this.baseURL,
            timeout: options.timeout || 30000,
            headers: this.headers,
        });
    }

    getCookieString() {
        return Object.entries(this.cookies).map(([k, v]) => `${k}=${v}`).join('; ');
    }

    setCookies(cookieObj) {
        Object.assign(this.cookies, cookieObj);
    }

    /**
     * 第一阶段：获取动态JS代码
     * @param {string} url - 返回JS代码的接口
     * @returns {string} JS 代码字符串
     */
    async fetchDynamicJS(url, params = {}) {
        const response = await this.client.get(url, {
            params,
            headers: { Cookie: this.getCookieString() },
            transformResponse: [(data) => data],
        });
        return response.data;
    }

    /**
     * 第二阶段：携带Cookie请求数据
     */
    async fetchData(url, params = {}) {
        const response = await this.client.get(url, {
            params,
            headers: { Cookie: this.getCookieString() },
        });
        return response.data;
    }

    async post(url, data = {}) {
        const response = await this.client.post(url, data, {
            headers: { Cookie: this.getCookieString() },
        });
        return response.data;
    }

    sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    async fetchWithDelay(url, params = {}) {
        const data = await this.fetchData(url, params);
        const jitter = Math.random() * this.delay * 0.3;
        await this.sleep(this.delay + jitter);
        return data;
    }
}

module.exports = { TwoPhaseClient, DEFAULT_HEADERS };
