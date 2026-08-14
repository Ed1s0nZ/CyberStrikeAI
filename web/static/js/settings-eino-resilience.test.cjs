const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = process.cwd();
const settings = fs.readFileSync(path.join(root, 'web/static/js/settings.js'), 'utf8');
const template = fs.readFileSync(path.join(root, 'web/templates/index.html'), 'utf8');
const zh = JSON.parse(fs.readFileSync(path.join(root, 'web/static/i18n/zh-CN.json'), 'utf8'));
const en = JSON.parse(fs.readFileSync(path.join(root, 'web/static/i18n/en-US.json'), 'utf8'));

test('Eino 模型 retry/failover 设置页读写链路完整', () => {
    [
        'eino-model-retry-max-retries',
        'eino-model-retry-max-backoff-sec',
        'eino-model-failover-channels',
        'eino-model-failover-max-retries'
    ].forEach((id) => {
        assert.match(template, new RegExp(`id="${id}"`));
        assert.match(settings, new RegExp(`getElementById\\('${id}'\\)`));
    });

    [
        'model_retry_max_retries',
        'model_retry_max_backoff_sec',
        'model_failover_channels',
        'model_failover_max_retries'
    ].forEach((key) => {
        assert.match(settings, new RegExp(`${key}`));
    });

    assert.match(settings, /failoverChannelsRaw\.split\(\s*\/\[\\n,，\]\//);
    assert.match(settings, /Array\.from\(new Set\(/);
});

test('Eino 模型 retry/failover 设置项有中英文文案', () => {
    [
        'einoModelRetryMaxRetries',
        'einoModelRetryMaxRetriesHint',
        'einoModelRetryMaxBackoffSec',
        'einoModelRetryMaxBackoffSecHint',
        'einoModelFailoverChannels',
        'einoModelFailoverChannelsPlaceholder',
        'einoModelFailoverChannelsHint',
        'einoModelFailoverMaxRetries',
        'einoModelFailoverMaxRetriesHint'
    ].forEach((key) => {
        assert.equal(typeof zh.settingsBasic[key], 'string', `zh ${key}`);
        assert.ok(zh.settingsBasic[key].length > 0, `zh ${key} is empty`);
        assert.equal(typeof en.settingsBasic[key], 'string', `en ${key}`);
        assert.ok(en.settingsBasic[key].length > 0, `en ${key} is empty`);
    });
});
