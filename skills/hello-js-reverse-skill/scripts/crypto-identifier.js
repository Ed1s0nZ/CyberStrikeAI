/**
 * 加密算法识别器
 * 根据密文特征自动识别可能的加密算法
 * 
 * 使用方式:
 *   node crypto-identifier.js "e10adc3949ba59abbe56e057f20f883e"
 *   node crypto-identifier.js --file=captured_params.json
 */

const PATTERNS = [
    {
        name: 'MD5',
        regex: /^[0-9a-f]{32}$/i,
        description: '32位十六进制字符串',
        length: 32,
        charset: 'hex',
        verify: '对比标准 MD5 输出。如果不一致，可能是自定义 MD5（如 chrsz=16）',
    },
    {
        name: 'MD5 (大写)',
        regex: /^[0-9A-F]{32}$/,
        description: '32位大写十六进制',
        length: 32,
        charset: 'HEX',
    },
    {
        name: 'SHA-1',
        regex: /^[0-9a-f]{40}$/i,
        description: '40位十六进制字符串',
        length: 40,
        charset: 'hex',
    },
    {
        name: 'SHA-256',
        regex: /^[0-9a-f]{64}$/i,
        description: '64位十六进制字符串',
        length: 64,
        charset: 'hex',
    },
    {
        name: 'SHA-512',
        regex: /^[0-9a-f]{128}$/i,
        description: '128位十六进制字符串',
        length: 128,
        charset: 'hex',
    },
    {
        name: 'Base64',
        regex: /^[A-Za-z0-9+/]+=*$/,
        description: '标准 Base64 编码',
        charset: 'base64',
        verify: '尝试 atob() 解码，检查结果是否有意义',
    },
    {
        name: 'Base64url',
        regex: /^[A-Za-z0-9_-]+$/,
        description: 'URL安全 Base64 编码（无填充）',
        charset: 'base64url',
        verify: '替换 -_ 为 +/ 后尝试解码',
    },
    {
        name: 'AES-CBC/ECB (Base64)',
        regex: /^[A-Za-z0-9+/]+=*$/,
        test: (s) => {
            const raw = Buffer.from(s, 'base64');
            return raw.length > 0 && raw.length % 16 === 0;
        },
        description: 'Base64 编码，解码后长度为 16 的倍数',
        verify: '需要找到 key (16/24/32字节) 和 iv (16字节，CBC模式)',
    },
    {
        name: 'AES-CBC/ECB (Hex)',
        regex: /^[0-9a-f]+$/i,
        test: (s) => s.length >= 32 && s.length % 32 === 0,
        description: '十六进制编码，长度为 32 的倍数',
        verify: '需要找到 key 和 iv',
    },
    {
        name: 'DES-ECB/CBC (Base64)',
        regex: /^[A-Za-z0-9+/]+=*$/,
        test: (s) => {
            const raw = Buffer.from(s, 'base64');
            return raw.length > 0 && raw.length % 8 === 0 && raw.length % 16 !== 0;
        },
        description: 'Base64 编码，解码后长度为 8 的倍数但不是 16 的倍数',
        verify: '需要找到 8 字节 key',
    },
    {
        name: 'RSA (Base64)',
        regex: /^[A-Za-z0-9+/]+=*$/,
        test: (s) => {
            const raw = Buffer.from(s, 'base64');
            return raw.length >= 64;
        },
        description: '较长的 Base64 字符串（64字节以上）',
        verify: '需要找到公钥/私钥',
    },
    {
        name: 'RSA (Hex)',
        regex: /^[0-9a-f]+$/i,
        test: (s) => s.length >= 128,
        description: '较长的十六进制字符串',
        verify: '搜索 RSA、publicKey、modulus 关键词',
    },
    {
        name: 'HMAC-SHA256',
        regex: /^[0-9a-f]{64}$/i,
        description: '64位十六进制（与 SHA-256 相同格式）',
        verify: '搜索 HMAC、createHmac、HmacSHA256 关键词',
    },
    {
        name: 'UUID',
        regex: /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
        description: 'UUID 格式',
    },
    {
        name: '时间戳 (秒)',
        regex: /^1[0-9]{9}$/,
        description: '10位数字，Unix 时间戳（秒）',
    },
    {
        name: '时间戳 (毫秒)',
        regex: /^1[0-9]{12}$/,
        description: '13位数字，Unix 时间戳（毫秒）',
    },
    {
        name: 'JWT',
        regex: /^eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/,
        description: 'JSON Web Token（三段 Base64url）',
        verify: '分别解码三段：Header.Payload.Signature',
    },
    {
        name: '十六进制编码字符串',
        regex: /^([0-9a-f]{2})+$/i,
        test: (s) => s.length > 10 && s.length % 2 === 0,
        description: '连续十六进制对',
        verify: '尝试 Buffer.from(str, "hex").toString()',
    },
];

function identify(ciphertext) {
    const results = [];
    const trimmed = ciphertext.trim();
    
    for (const pattern of PATTERNS) {
        if (pattern.regex.test(trimmed)) {
            if (pattern.test && !pattern.test(trimmed)) continue;
            if (pattern.length && trimmed.length !== pattern.length) continue;
            
            results.push({
                algorithm: pattern.name,
                confidence: calculateConfidence(trimmed, pattern),
                description: pattern.description,
                verify: pattern.verify || '',
            });
        }
    }
    
    results.sort((a, b) => b.confidence - a.confidence);
    return results;
}

function calculateConfidence(text, pattern) {
    let score = 50;
    
    if (pattern.length && text.length === pattern.length) score += 30;
    if (pattern.charset === 'hex' && /^[0-9a-f]+$/i.test(text)) score += 10;
    if (pattern.charset === 'base64' && text.endsWith('=')) score += 10;
    if (pattern.test && pattern.test(text)) score += 20;
    
    return Math.min(score, 95);
}

function analyzeMultiple(samples) {
    const allResults = samples.map(s => ({ sample: s, matches: identify(s) }));
    
    const lengthSet = new Set(samples.map(s => s.length));
    const charsetConsistent = samples.every(s => /^[0-9a-f]+$/i.test(s)) ||
                              samples.every(s => /^[A-Za-z0-9+/]+=*$/.test(s));
    
    const analysis = {
        sampleCount: samples.length,
        lengths: [...lengthSet],
        fixedLength: lengthSet.size === 1,
        charsetConsistent,
        results: allResults,
    };
    
    if (analysis.fixedLength && analysis.charsetConsistent) {
        analysis.recommendation = '所有样本长度相同且字符集一致，很可能是同一算法的不同输出';
    } else if (!analysis.fixedLength) {
        analysis.recommendation = '样本长度不一致，可能涉及变长输入或不同算法';
    }
    
    return analysis;
}

function formatReport(ciphertext) {
    const results = identify(ciphertext);
    
    let report = '';
    report += `\n🔍 密文分析报告\n`;
    report += `${'━'.repeat(50)}\n`;
    report += `输入: ${ciphertext.substring(0, 80)}${ciphertext.length > 80 ? '...' : ''}\n`;
    report += `长度: ${ciphertext.length}\n`;
    report += `字符集: ${detectCharset(ciphertext)}\n`;
    report += `${'━'.repeat(50)}\n`;
    
    if (results.length === 0) {
        report += `\n❌ 未识别到已知的加密模式\n`;
        report += `建议：搜索 JS 源码中的加密函数名\n`;
    } else {
        report += `\n可能的算法（按可信度排序）:\n\n`;
        results.forEach((r, i) => {
            report += `  ${i + 1}. ${r.algorithm} (${r.confidence}%)\n`;
            report += `     描述: ${r.description}\n`;
            if (r.verify) report += `     验证: ${r.verify}\n`;
            report += `\n`;
        });
    }
    
    // Base64 解码尝试
    if (/^[A-Za-z0-9+/]+=*$/.test(ciphertext)) {
        try {
            const decoded = Buffer.from(ciphertext, 'base64');
            report += `📦 Base64 解码结果:\n`;
            report += `  二进制长度: ${decoded.length} 字节\n`;
            if (decoded.length % 16 === 0) report += `  ⚠ 长度是 16 的倍数（可能是 AES）\n`;
            if (decoded.length % 8 === 0) report += `  ⚠ 长度是 8 的倍数（可能是 DES）\n`;
            const textAttempt = decoded.toString('utf-8');
            if (/^[\x20-\x7e]+$/.test(textAttempt)) {
                report += `  文本: ${textAttempt.substring(0, 100)}\n`;
            } else {
                report += `  十六进制: ${decoded.toString('hex').substring(0, 100)}\n`;
            }
        } catch (e) {}
    }
    
    return report;
}

function detectCharset(str) {
    if (/^[0-9]+$/.test(str)) return '纯数字';
    if (/^[0-9a-f]+$/i.test(str)) return '十六进制';
    if (/^[A-Za-z0-9+/]+=*$/.test(str)) return 'Base64';
    if (/^[A-Za-z0-9_-]+$/.test(str)) return 'Base64url / 字母数字';
    if (/^[\x20-\x7e]+$/.test(str)) return '可打印ASCII';
    return '混合/二进制';
}

// CLI
if (require.main === module) {
    const args = process.argv.slice(2);
    
    if (args.length === 0) {
        console.log('用法:');
        console.log('  node crypto-identifier.js "<密文>"');
        console.log('  node crypto-identifier.js --file=params.json');
        console.log('  node crypto-identifier.js --compare "<样本1>" "<样本2>" ...');
        process.exit(0);
    }
    
    if (args[0] === '--compare') {
        const samples = args.slice(1);
        const analysis = analyzeMultiple(samples);
        console.log('\n📊 多样本对比分析');
        console.log('━'.repeat(50));
        console.log(`样本数量: ${analysis.sampleCount}`);
        console.log(`长度分布: ${analysis.lengths.join(', ')}`);
        console.log(`固定长度: ${analysis.fixedLength ? '是' : '否'}`);
        console.log(`字符集一致: ${analysis.charsetConsistent ? '是' : '否'}`);
        if (analysis.recommendation) console.log(`\n💡 ${analysis.recommendation}`);
        analysis.results.forEach((r, i) => {
            console.log(`\n--- 样本 ${i + 1}: ${r.sample.substring(0, 40)}... ---`);
            r.matches.slice(0, 3).forEach(m => {
                console.log(`  ${m.algorithm} (${m.confidence}%)`);
            });
        });
    } else if (args[0].startsWith('--file=')) {
        const fs = require('fs');
        const data = JSON.parse(fs.readFileSync(args[0].substring(7), 'utf-8'));
        const samples = Array.isArray(data) ? data : Object.values(data);
        samples.forEach((s, i) => {
            console.log(formatReport(String(s)));
        });
    } else {
        console.log(formatReport(args[0]));
    }
}

module.exports = { identify, analyzeMultiple, formatReport };
