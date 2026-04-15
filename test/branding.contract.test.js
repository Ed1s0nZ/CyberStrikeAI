const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const repoRoot = path.resolve(__dirname, '..');

function read(relativePath) {
    return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
}

test('visible branding uses 能盾智御 in web templates and i18n bundles', () => {
    const indexHtml = read('web/templates/index.html');
    const apiDocsHtml = read('web/templates/api-docs.html');
    const zh = read('web/static/i18n/zh-CN.json');
    const en = read('web/static/i18n/en-US.json');
    const terminalJs = read('web/static/js/terminal.js');

    assert.match(indexHtml, /<title>能盾智御<\/title>/, 'expected main page title to use 能盾智御');
    assert.match(indexHtml, /<h1>能盾智御<\/h1>/, 'expected header title to use 能盾智御');
    assert.match(apiDocsHtml, /能盾智御/, 'expected API docs page to use 能盾智御');
    assert.match(zh, /"title": "能盾智御"/, 'expected zh-CN header title to use 能盾智御');
    assert.match(zh, /"title": "登录 能盾智御"/, 'expected zh-CN login title to use 能盾智御');
    assert.match(en, /"title": "能盾智御"/, 'expected en-US header title to use 能盾智御');
    assert.match(en, /"title": "Sign in to 能盾智御"/, 'expected en-US login title to use 能盾智御');
    assert.match(terminalJs, /能盾智御终端|能盾智御 终端|能盾智御 Terminal/, 'expected terminal welcome line to use 能盾智御');
});
