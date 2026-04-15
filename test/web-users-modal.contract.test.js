const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const repoRoot = path.resolve(__dirname, '..');

function read(relativePath) {
    return fs.readFileSync(path.join(repoRoot, relativePath), 'utf8');
}

test('security settings provides dedicated system modals for web users and access roles', () => {
    const html = read('web/templates/index.html');

    assert.match(html, /id="web-user-modal"/, 'expected web user modal in index.html');
    assert.match(html, /id="web-user-password-modal"/, 'expected web user password reset modal in index.html');
    assert.match(html, /id="web-access-role-modal"/, 'expected web access role modal in index.html');
    assert.match(html, /id="security-confirm-modal"/, 'expected security confirmation modal in index.html');
});

test('web user management no longer relies on prompt-based configuration flows', () => {
    const script = read('web/static/js/web-users.js');

    assert.doesNotMatch(script, /window\.prompt\s*\(/, 'expected modal-based forms instead of window.prompt');
    assert.doesNotMatch(script, /window\.confirm\s*\(/, 'expected modal-based confirmations instead of window.confirm');
});
