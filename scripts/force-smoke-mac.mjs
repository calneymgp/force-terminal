import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import net from 'node:net';
import { spawn } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';
import WebSocket from 'ws';

// Exercise the unmodified app copied from the DMG, never the development entrypoint.
assert.equal(process.platform, 'darwin');
assert.equal(process.arch, 'arm64');
const appBundle = path.resolve(process.argv[2] || '');
const output = path.resolve(process.argv[3] || 'smoke-results');
const executable = path.join(appBundle, 'Contents/MacOS/Force Terminal');
assert.ok(fs.existsSync(executable), 'packaged executable missing');
assert.ok(fs.existsSync(path.join(appBundle, 'Contents/Resources/force-adhoc.json')), 'expected test package');
fs.mkdirSync(output, { recursive: true });
const root = fs.mkdtempSync(path.join(os.tmpdir(), 'force-mac-smoke-'));
const dirs = Object.fromEntries(['data', 'config', 'cache'].map(name => [name, path.join(root, name)]));
for (const dir of Object.values(dirs)) fs.mkdirSync(dir, { mode: 0o700 });
fs.writeFileSync(path.join(dirs.config, 'settings.json'), JSON.stringify({
    'telemetry:enabled': false, 'autoupdate:enabled': false, 'term:disablewebgl': true,
}));
const report = { platform: process.platform, arch: process.arch, checks: [], passed: false,
    limits: ['No physical M5, Finder/Dock, SSH, Wave coexistence, Keychain or signed update test.',
        'CI extraction does not reproduce browser quarantine or first-download Gatekeeper handling.'] };
const owned = new Set();
const connections = new Set();
function checked(name) { report.checks.push(name); console.log(`PASS: ${name}`); }
async function until(fn, label, timeout = 45000) {
    const end = Date.now() + timeout;
    while (Date.now() < end) {
        if (await fn()) return;
        await delay(200);
    }
    throw new Error(`Timed out: ${label}`);
}
async function port() {
    const server = net.createServer();
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const result = server.address().port;
    await new Promise(resolve => server.close(resolve));
    return result;
}
async function connect(url) {
    const ws = new WebSocket(url);
    await new Promise((resolve, reject) => { ws.once('open', resolve); ws.once('error', reject); });
    let serial = 0;
    const pending = new Map();
    const rejectPending = () => { for (const item of pending.values()) item.reject(new Error('CDP closed')); pending.clear(); };
    ws.on('error', rejectPending);
    ws.on('close', rejectPending);
    ws.on('message', raw => {
        const message = JSON.parse(raw);
        const item = pending.get(message.id);
        if (!item) return;
        pending.delete(message.id);
        clearTimeout(item.timer);
        if (message.error || message.result?.exceptionDetails) item.reject(new Error('CDP command failed'));
        else item.resolve(message.result);
    });
    const cdp = {
        call(method, params = {}) {
            return new Promise((resolve, reject) => {
                const id = ++serial;
                const timer = setTimeout(() => { pending.delete(id); reject(new Error(`CDP timeout: ${method}`)); }, 12000);
                pending.set(id, { resolve, reject, timer });
                ws.send(JSON.stringify({ id, method, params }));
            });
        },
        async evaluate(expression) {
            const result = await this.call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
            return result.result?.value;
        },
        close() { ws.close(); connections.delete(this); },
    };
    connections.add(cdp);
    return cdp;
}
async function launch() {
    const debugPort = await port();
    const logPath = path.join(dirs.data, 'waveapp.log');
    const logOffset = fs.existsSync(logPath) ? fs.statSync(logPath).size : 0;
    const env = Object.fromEntries(['PATH', 'HOME', 'USER', 'LOGNAME', 'SHELL', 'LANG', 'LC_ALL', 'TMPDIR', 'TERM']
        .filter(key => process.env[key] !== undefined).map(key => [key, process.env[key]]));
    const proc = spawn(executable, ['--remote-debugging-address=127.0.0.1', `--remote-debugging-port=${debugPort}`], {
        detached: true, stdio: ['ignore', 'pipe', 'pipe'], env: {
            ...env,
            FORCE_TERMINAL_DATA_HOME: dirs.data, FORCE_TERMINAL_CONFIG_HOME: dirs.config,
            FORCE_TERMINAL_CACHE_HOME: dirs.cache,
            WCLOUD_ENDPOINT: 'https://127.0.0.1:1', WCLOUD_PING_ENDPOINT: 'https://127.0.0.1:1',
            WAVETERM_WAVEAI_ENDPOINT: 'https://127.0.0.1:1',
        },
    });
    owned.add(proc);
    let logs = '';
    proc.stdout.on('data', data => { logs += data; });
    proc.stderr.on('data', data => { logs += data; });
    let exited;
    proc.once('error', error => { exited = { error: error.message }; });
    proc.once('close', (code, signal) => { exited = { code, signal }; owned.delete(proc); });
    let renderer;
    await until(async () => {
        if (exited) throw new Error(`Packaged app exited before renderer was ready: ${JSON.stringify(exited)}`);
        let targets;
        try {
            targets = await (await fetch(`http://127.0.0.1:${debugPort}/json/list`, { signal: AbortSignal.timeout(1000) })).json();
        } catch { return false; }
        for (const target of targets.filter(t => t.type === 'page' && t.webSocketDebuggerUrl)) {
            const cdp = await connect(target.webSocketDebuggerUrl);
            const ready = await cdp.evaluate('Boolean(window.api && window.RpcApi && window.TabRpcClient && window.globalAtoms?.staticTabId)');
            if (ready) { renderer = cdp; return true; }
            cdp.close();
        }
        return false;
    }, 'packaged renderer readiness');
    return { proc, renderer, getExit: () => exited,
        getLogs: () => (fs.existsSync(logPath) ? fs.readFileSync(logPath).subarray(logOffset).toString() : '') + logs };
}
async function screenshot(cdp, name) {
    const result = await cdp.call('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(output, name), Buffer.from(result.data, 'base64'));
}
async function quit(app) {
    app.proc.kill('SIGTERM'); // The real main-process handler calls app.quit(), including persistence.
    await until(() => Boolean(app.getExit()), 'normal app/backend quit', 40000);
    assert.deepEqual(app.getExit(), { code: 0, signal: null });
    assert.match(app.getLogs(), /shutdown complete/);
    assert.doesNotMatch(app.getLogs(), /shutdown storage flush failed|secretstore: error writing secrets/);
    app.renderer.close();
}
async function clickButton(cdp, label) {
    const expression = `Array.from(document.querySelectorAll('button')).find(button=>button.textContent.trim().startsWith(${JSON.stringify(label)}))`;
    await until(() => cdp.evaluate(`Boolean(${expression})`), `onboarding ${label}`);
    assert.equal(await cdp.evaluate(`(()=>{const button=${expression};button.click();return true})()`), true);
}
try {
    const first = await launch();
    const cdp = first.renderer;
    assert.equal(await cdp.evaluate('document.title.includes("Force Terminal")'), true);
    const profile = await cdp.evaluate('({data:window.api.getDataDir(),config:window.api.getConfigDir(),updater:window.api.getUpdaterStatus().status})');
    assert.equal(profile.data, dirs.data);
    assert.equal(profile.config, dirs.config);
    assert.equal(profile.updater, 'dev-disabled');
    checked('packaged renderer, Force profile and ad hoc updater isolation');
    await until(() => cdp.evaluate('document.body.innerText.includes("Welcome to Force Terminal")'), 'Force onboarding');
    await screenshot(cdp, '01-onboarding.png');
    await clickButton(cdp, 'Continue');
    await clickButton(cdp, 'Maybe Later');
    await clickButton(cdp, 'Skip Feature Tour');
    await until(() => cdp.evaluate('Boolean(document.querySelector(".xterm-helper-textarea")) && Array.from(document.querySelectorAll(".xterm-rows > div")).some(row => row.textContent.trim())'), 'terminal mount and shell output');
    await cdp.evaluate('document.querySelector(".xterm-helper-textarea").focus();true');
    const suffix = crypto.randomUUID().replaceAll('-', '');
    const marker = `FORCE_MAC_SMOKE_${suffix}`;
    await cdp.call('Input.insertText', { text: `printf 'FORCE_%s\\n' 'MAC_SMOKE_${suffix}'` });
    await cdp.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Enter', code: 'Enter', text: '\r', unmodifiedText: '\r', windowsVirtualKeyCode: 13 });
    await cdp.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 });
    await until(() => cdp.evaluate(`Array.from(document.querySelectorAll('.xterm-rows > div')).some(row=>row.textContent.trim()===${JSON.stringify(marker)})`), 'terminal command output');
    checked('local terminal executed command and rendered distinct output');
    const file = path.join(root, 'fixture.txt');
    fs.writeFileSync(file, 'before-smoke\n');
    const fileData = { info: { path: file }, data64: Buffer.from('after-smoke\n').toString('base64') };
    assert.equal(await cdp.evaluate(`window.RpcApi.FileReadCommand(window.TabRpcClient,{info:{path:${JSON.stringify(file)}}}).then(r=>atob(r.data64)==='before-smoke\\n')`), true);
    await cdp.evaluate(`window.RpcApi.FileWriteCommand(window.TabRpcClient,${JSON.stringify(fileData)}).then(()=>true)`);
    assert.equal(await cdp.evaluate(`window.RpcApi.FileReadCommand(window.TabRpcClient,{info:{path:${JSON.stringify(file)}}}).then(r=>atob(r.data64)==='after-smoke\\n')`), true);
    assert.equal(fs.readFileSync(file, 'utf8'), 'after-smoke\n');
    checked('real renderer/backend local file read and write');
    const tabId = await cdp.evaluate('window.globalStore.get(window.globalAtoms.staticTabId)');
    assert.ok(tabId);
    const metaRequest = { oref: `tab:${tabId}`, meta: { 'force:smoke': marker } };
    await cdp.evaluate(`window.RpcApi.SetMetaCommand(window.TabRpcClient,${JSON.stringify(metaRequest)}).then(()=>true)`);
    await screenshot(cdp, '02-terminal.png');
    await quit(first);
    checked('first normal quit completed backend persistence');
    const second = await launch();
    assert.equal(await second.renderer.evaluate('window.globalStore.get(window.globalAtoms.staticTabId)'), tabId);
    assert.equal(await second.renderer.evaluate(`window.RpcApi.GetMetaCommand(window.TabRpcClient,{oref:${JSON.stringify(`tab:${tabId}`)}}).then(r=>r['force:smoke']===${JSON.stringify(marker)})`), true);
    assert.equal(await second.renderer.evaluate(`window.RpcApi.FileReadCommand(window.TabRpcClient,{info:{path:${JSON.stringify(file)}}}).then(r=>atob(r.data64)==='after-smoke\\n')`), true);
    await screenshot(second.renderer, '03-reopened.png');
    await quit(second);
    checked('same tab, database metadata and file survived normal quit/relaunch');
    report.passed = true;
} catch (error) {
    // No raw process logs or environment: those can contain internal authentication data.
    report.error = error.message;
    console.error(`FAIL: ${error.message}`);
    process.exitCode = 1;
} finally {
    for (const connection of connections) connection.close();
    for (const proc of owned) { try { process.kill(-proc.pid, 'SIGTERM'); } catch {} }
    await delay(1000);
    for (const proc of owned) { try { process.kill(-proc.pid, 'SIGKILL'); } catch {} }
    fs.rmSync(root, { recursive: true, force: true });
    fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
}
