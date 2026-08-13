/**
 * 加密参数生成模块
 * 根据逆向分析还原的加密逻辑，生成请求所需的加密参数
 */

const crypto = require('crypto');

function md5(str) {
    return crypto.createHash('md5').update(str).digest('hex');
}

function sha256(str) {
    return crypto.createHash('sha256').update(str).digest('hex');
}

function hmacSha256(message, secret) {
    return crypto.createHmac('sha256', secret).update(message).digest('hex');
}

function aesEncrypt(plaintext, key, iv, mode = 'cbc') {
    const algorithm = `aes-${key.length * 8}-${mode}`;
    const cipher = crypto.createCipheriv(algorithm, key, mode === 'ecb' ? null : iv);
    cipher.setAutoPadding(true);
    let encrypted = cipher.update(plaintext, 'utf8', 'base64');
    encrypted += cipher.final('base64');
    return encrypted;
}

function aesDecrypt(ciphertext, key, iv, mode = 'cbc') {
    const algorithm = `aes-${key.length * 8}-${mode}`;
    const decipher = crypto.createDecipheriv(algorithm, key, mode === 'ecb' ? null : iv);
    decipher.setAutoPadding(true);
    let decrypted = decipher.update(ciphertext, 'base64', 'utf8');
    decrypted += decipher.final('utf8');
    return decrypted;
}

/**
 * 生成请求签名
 * TODO: 根据实际逆向分析修改此函数
 * @param {Object} params - 请求参数
 * @returns {string} 签名值
 */
function generateSign(params) {
    const { page, timestamp } = params;
    // 示例：sign = md5(page + "|" + timestamp + "|" + SECRET)
    const SECRET = 'your_secret_here';
    return md5(`${page}|${timestamp}|${SECRET}`);
}

/**
 * 生成加密参数 m
 * TODO: 根据实际逆向分析修改此函数
 * @param {number} page - 页码
 * @returns {string} 加密后的参数
 */
function generateM(page) {
    const timestamp = Math.floor(Date.now() / 1000);
    const sign = generateSign({ page, timestamp });
    return `${sign}|${timestamp}`;
}

module.exports = {
    md5,
    sha256,
    hmacSha256,
    aesEncrypt,
    aesDecrypt,
    generateSign,
    generateM,
};
