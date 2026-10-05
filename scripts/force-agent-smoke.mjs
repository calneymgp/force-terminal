import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import net from 'node:net';
import crypto from 'node:crypto';
import { spawn } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';
import WebSocket from 'ws';

// Real application/backend, disposable data, and a test-owned executable named
// claude. This executable never contacts a provider or reads CLI history.
const output = path.resolve(process.argv[2] || '/tmp/force-agent-smoke-results');
const appPath = process.argv[3] ? path.resolve(process.argv[3]) : null;
fs.mkdirSync(output, { recursive: true });
const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'force-agent-smoke-')));
const dirs = Object.fromEntries(['data', 'config', 'cache', 'bin', 'project with space ç'].map(key => [key, path.join(root, key)]));
for (const dir of Object.values(dirs)) fs.mkdirSync(dir, { mode: 0o700 });
const projectRoot = dirs['project with space ç'];
const tracePath = path.join(root, 'fake-cli.jsonl');
const prompt = 'DevOps de teste: preservar contexto e pedir revisão. Unicode: ação 🚀';
const promptHash = crypto.createHash('sha256').update(prompt).digest('hex');
fs.writeFileSync(path.join(dirs.config, 'settings.json'), JSON.stringify({
    'autoupdate:enabled': false, 'term:disablewebgl': true, 'app:confirmquit': false,
    'term:localshellpath': process.platform === 'darwin' ? '/bin/zsh' : '/bin/bash',
    'web:defaulturl': 'https://127.0.0.1:1',
}));
const fakeCLI = path.join(dirs.bin, 'claude');
// Use an absolute Node executable so a login-shell PATH cannot select another
// interpreter. The application inherits a PATH with only this Claude first.
fs.writeFileSync(fakeCLI, `#!${process.execPath}
const fs = require('node:fs'), crypto = require('node:crypto');
const trace = ${JSON.stringify(tracePath)};
const args = process.argv.slice(2), promptPath = args[args.indexOf('--append-system-prompt-file') + 1];
const record = value => fs.appendFileSync(trace, JSON.stringify(value) + '\\n');
record({kind:'launch', args, cwd:process.cwd(), pid:process.pid,
    promptHash:crypto.createHash('sha256').update(fs.readFileSync(promptPath)).digest('hex'),
    fileMode:fs.statSync(promptPath).mode & 511, parentMode:fs.statSync(require('node:path').dirname(promptPath)).mode & 511});
process.stdout.write('FORCE_FAKE_AGENT_READY\\r\\n');
let input = '';
process.stdin.on('data', data => { input += data.toString(); if(input.includes('FORCE_FAKE_EXIT')) finish(); });
const finish = () => { record({kind:'exit',pid:process.pid}); process.stdout.write('FORCE_FAKE_AGENT_EXIT\\r\\n'); process.exit(0); };
process.on('SIGTERM', finish);
process.stdin.resume();
`, { mode: 0o700 });
const launches = () => fs.existsSync(tracePath) ? fs.readFileSync(tracePath, 'utf8').trim().split('\n').filter(Boolean).map(JSON.parse).filter(row => row.kind === 'launch') : [];
const report = { platform: process.platform, arch: process.arch, packaged: Boolean(appPath), checks: [], passed: false,
    limits: ['Fake CLI verifies requested argv and local persistence; no provider-confirmed conversation, real SSH, physical M5 reboot, or signed update validation.'] };
const owned = new Set(), connections = new Set();
const checked = name => { report.checks.push(name); console.log(`PASS: ${name}`); };
async function until(fn, label, timeout = 40000) {
    const end = Date.now() + timeout;
    while (Date.now() < end) { if (await fn()) return; await delay(150); }
    throw new Error(`Timed out: ${label}`);
}
async function freePort() {
    const server = net.createServer();
    await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
    const port = server.address().port;
    await new Promise(resolve => server.close(resolve)); return port;
}
async function connect(url) {
    const ws = new WebSocket(url);
    await new Promise((resolve, reject) => { ws.once('open', resolve); ws.once('error', reject); });
    const pending = new Map(), contexts = new Set(); let serial = 0;
    const rejectPending = () => { for (const entry of pending.values()) { clearTimeout(entry.timer); entry.reject(new Error('CDP closed')); } pending.clear(); };
    ws.on('close', rejectPending); ws.on('error', rejectPending);
    ws.on('message', raw => {
        const msg = JSON.parse(raw);
        if (msg.method === 'Runtime.executionContextCreated') contexts.add(msg.params.context.id);
        if (msg.method === 'Runtime.executionContextDestroyed') contexts.delete(msg.params.executionContextId);
        const entry = pending.get(msg.id);
        if (!entry) return;
        pending.delete(msg.id); clearTimeout(entry.timer);
        if (msg.error || msg.result?.exceptionDetails) {
            const detail = msg.error?.message || msg.result?.exceptionDetails?.exception?.description || msg.result?.exceptionDetails?.text;
            entry.reject(new Error(`CDP evaluation failed: ${String(detail).split('\n')[0].replace(/[A-Za-z0-9_-]{24,}(?:\.[A-Za-z0-9_-]+)*/g, '<redacted>')}`));
        } else entry.resolve(msg.result);
    });
    const cdp = {
        call(method, params = {}) { return new Promise((resolve, reject) => {
            const id = ++serial, timer = setTimeout(() => { pending.delete(id); reject(new Error(`CDP timeout: ${method}`)); }, 12000);
            pending.set(id, { resolve, reject, timer }); ws.send(JSON.stringify({ id, method, params }));
        }); },
        contexts,
        async evaluate(expression, awaitPromise = true) { return (await this.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise, ...(this.contextID ? { contextId: this.contextID } : {}) })).result?.value; },
        close() { ws.close(); connections.delete(this); },
    };
    connections.add(cdp); return cdp;
}
const electronExpression = `process.getBuiltinModule('module').createRequire(${JSON.stringify(path.resolve('dist/main/index.js'))})('electron')`;
async function launch() {
    report.stage = 'launch isolated application';
    const debugPort = await freePort(), inspectPort = await freePort();
    const executable = appPath ? path.join(appPath, 'Contents/MacOS/Force Terminal') : path.resolve('node_modules/electron/dist/electron');
    assert.ok(fs.existsSync(executable), 'Electron executable missing');
    const env = Object.fromEntries(['PATH', 'HOME', 'USER', 'LOGNAME', 'SHELL', 'LANG', 'LC_ALL', 'TMPDIR', 'TERM', 'DISPLAY', 'XAUTHORITY']
        .filter(key => process.env[key] !== undefined).map(key => [key, process.env[key]]));
    env.PATH = `${dirs.bin}${path.delimiter}${env.PATH || ''}`;
    const proc = spawn(executable, ['--no-sandbox', `--inspect=127.0.0.1:${inspectPort}`,
        '--remote-debugging-address=127.0.0.1', `--remote-debugging-port=${debugPort}`, ...(appPath ? [] : [path.resolve('dist/main/index.js')])], {
        detached: true, stdio: ['ignore', 'pipe', 'pipe'], env: { ...env,
            FORCE_TERMINAL_DATA_HOME: dirs.data, FORCE_TERMINAL_CONFIG_HOME: dirs.config,
            FORCE_TERMINAL_CACHE_HOME: dirs.cache, WCLOUD_ENDPOINT: 'https://127.0.0.1:1',
            WCLOUD_PING_ENDPOINT: 'https://127.0.0.1:1', WAVETERM_WAVEAI_ENDPOINT: 'https://127.0.0.1:1' },
    });
    owned.add(proc); proc.stdout.resume(); proc.stderr.resume();
    let ended;
    proc.once('error', error => { ended = { error: error.message }; });
    proc.once('close', (code, signal) => { ended = { code, signal }; owned.delete(proc); });
    async function target(port, renderer) {
        let result;
        await until(async () => {
            if (ended) throw new Error(`App exited: ${JSON.stringify(ended)}`);
            try {
                const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`, { signal: AbortSignal.timeout(1000) })).json();
                for (const target of targets.filter(t => !renderer || t.type === 'page')) {
                    const cdp = await connect(target.webSocketDebuggerUrl);
                    if (!renderer) {
                        await cdp.call('Runtime.enable');
                        for (const id of cdp.contexts) {
                            cdp.contextID = id;
                            if (await cdp.evaluate(`typeof process !== 'undefined' && process.pid === ${proc.pid} && Boolean(process.env && process.getBuiltinModule)`, false)) { result = cdp; return true; }
                        }
                    } else if (await cdp.evaluate('Boolean(window.WOS && window.globalStore && window.api && window.globalAtoms)')) { result = cdp; return true; }
                    cdp.close();
                }
            } catch { return false; } return false;
        }, renderer ? 'renderer ready' : 'main inspector ready'); return result;
    }
    const renderer = await target(debugPort, true);
    const main = await target(inspectPort, false);
    report.stage = 'verify test executable search path';
    assert.equal(await main.evaluate(`process.env.PATH.split(${JSON.stringify(path.delimiter)})[0]`, false), dirs.bin);
    return { proc, main, renderer, getExit: () => ended };
}
async function click(cdp, label, selector = 'button') {
    const expr = `Array.from(document.querySelectorAll(${JSON.stringify(selector)})).find(e => (e.getAttribute('aria-label') || e.textContent.trim()) === ${JSON.stringify(label)})`;
    await until(() => cdp.evaluate(`Boolean(${expr})`), `button ${label}`);
    assert.equal(await cdp.evaluate(`(()=>{const e=${expr};if(e.disabled)return false;e.click();return true})()`), true, label);
}
const service = (cdp, method, args = []) => cdp.evaluate(`window.WOS.callBackendService('force',${JSON.stringify(method)},${JSON.stringify(args)},true)`);
const instances = (cdp, project) => service(cdp, 'ListAgentInstances', [project.oid]);
async function selectProject(cdp, name) {
    const expression = `Array.from(document.querySelectorAll('.force-list .force-item-main')).find(e=>e.querySelector('strong')?.textContent===${JSON.stringify(name)})`;
    await until(() => cdp.evaluate(`Boolean(${expression})`), `project ${name}`);
    await cdp.evaluate(`${expression}.click();true`);
    await until(() => cdp.evaluate(`document.querySelector('.force-list [aria-current=true] strong')?.textContent===${JSON.stringify(name)}`), `selected project ${name}`);
}
async function prepare(app) {
    return app.main.evaluate(`(async()=>{
        const electron=${electronExpression}; let view;
        for(const candidate of electron.webContents.getAllWebContents().filter(w=>w.getURL().includes('/index.html'))){
            if(await candidate.executeJavaScript('Boolean(document.querySelector(".force-agent-section"))')){view=candidate;break;}
        }
        if(!view) throw new Error('Agent view missing');
        return new Promise((resolve,reject)=>{
            const id='force-agent-smoke-'+Date.now();
            const listener=(_event,key,result)=>{if(key===id){clearTimeout(timer);electron.ipcMain.removeListener('update-prepare-reply',listener);resolve(result)}};
            const timer=setTimeout(()=>{electron.ipcMain.removeListener('update-prepare-reply',listener);reject(new Error('Update preparation timeout'))},9000);
            electron.ipcMain.on('update-prepare-reply',listener);view.send('update-prepare',id,{freeze:false});
        });
    })()`);
}
async function quit(app) {
    await app.main.evaluate(`${electronExpression}.app.quit();true`);
    app.main.close(); app.renderer.close();
    await until(() => Boolean(app.getExit()), 'normal quit', 40000);
    assert.deepEqual(app.getExit(), { code: 0, signal: null });
}
async function shot(cdp, name) {
    fs.writeFileSync(path.join(output, name), Buffer.from((await cdp.call('Page.captureScreenshot', { format: 'png' })).data, 'base64'));
}
try {
    const app = await launch(), cdp = app.renderer;
    report.stage = 'verify isolated data';
    assert.equal(await cdp.evaluate('window.api.getDataDir()'), dirs.data);
    report.stage = 'dismiss initial welcome';
    await until(() => cdp.evaluate('document.body.innerText.includes("Welcome to Force Terminal")'), 'welcome');
    await click(cdp, 'Continue');
    await until(() => cdp.evaluate('Boolean(document.querySelector(".force-sidebar")) && !document.querySelector(".modal-wrapper")'), 'single welcome dismissed');
    const project = await service(cdp, 'SaveProject', [{ id: '', expectedversion: 0, creationkey: crypto.randomUUID(), name: 'Agent smoke', icon: 'folder', connection: '', rootpath: projectRoot }]);
    const profile = await service(cdp, 'SaveProfile', [{ id: '', expectedversion: 0, creationkey: crypto.randomUUID(), title: 'DevOps smoke', icon: 'server', systemprompt: prompt, adapter: 'claude-code' }]);
    await until(() => cdp.evaluate('Boolean(Array.from(document.querySelectorAll(".force-list .force-item-main")).find(e=>e.querySelector("strong")?.textContent==="Agent smoke"))'), 'catalog updated');
    await cdp.evaluate('Array.from(document.querySelectorAll(".force-list .force-item-main")).find(e=>e.querySelector("strong")?.textContent==="Agent smoke").click();true');
    await until(() => cdp.evaluate('Boolean(document.querySelector(".force-agent-section"))'), 'project agent panel');
    assert.equal(launches().length, 0);
    checked('isolated real app; one welcome; catalog save and selection did not launch CLI');
    await click(cdp, 'Novo agente');
    assert.equal(await cdp.evaluate('document.querySelector(".force-editor select")?.value'), profile.oid);
    assert.ok((await prepare(app)).reasons.some(reason => /agente/i.test(reason)), 'new agent dialog must defer update restart');
    checked('update preparation deferred restart while the new-agent dialog was open');
    assert.equal(await cdp.evaluate(`(()=>{const form=document.querySelector('[data-testid="force-new-agent-form"]');form.requestSubmit();form.requestSubmit();return true})()`), true);
    let instance;
    await until(async () => { const rows = await instances(cdp, project); instance = rows[0]; return rows.length === 1 && instance.status === 'running' && launches().length === 1; }, 'one explicitly launched agent');
    const original = { oid: instance.oid, blockid: instance.blockid, tabid: instance.tabid, session: instance.claudesessionid, hash: instance.prompthash, profileversion: instance.profileversion };
    const first = launches()[0];
    assert.equal(first.cwd, projectRoot); assert.equal(first.promptHash, promptHash);
    assert.equal(first.fileMode, 0o600); assert.equal(first.parentMode, 0o700);
    assert.equal(first.args[first.args.indexOf('--session-id') + 1], original.session);
    assert.ok(!first.args.includes('--last')); assert.equal(instance.identityevidence, 'requested');
    await until(() => cdp.evaluate(`Boolean(document.querySelector('[data-blockid="${original.blockid}"]'))`), 'original terminal focused');
    checked('repeated UI submit created one instance/process; private prompt and exact requested UUID');
    await click(cdp, 'Reconectar', '.force-agent-actions button');
    await until(() => cdp.evaluate('!document.querySelector(".force-agent-section [role=status]")'), 'live reconnect complete');
    assert.equal(launches().length, 1); assert.equal((await instances(cdp, project))[0].localpid, first.pid);
    const guarded = await cdp.evaluate(`(async()=>{try{await window.RpcApi.ControllerDestroyCommand(window.TabRpcClient,${JSON.stringify(original.blockid)});return false}catch{return true}})()`);
    assert.equal(guarded, true, 'generic controller destroy must reject Force agent');
    await cdp.evaluate(`window.RpcApi.ControllerResyncCommand(window.TabRpcClient,{tabid:${JSON.stringify(original.tabid)},blockid:${JSON.stringify(original.blockid)},forcerestart:true})`);
    assert.equal(launches().length, 1); assert.equal((await instances(cdp, project))[0].status, 'running');
    checked('live reconnect and generic force-restart preserved the process and terminal');
    await service(cdp, 'SaveProfile', [{ id: profile.oid, expectedversion: profile.version, creationkey: '', title: profile.title, icon: profile.icon, systemprompt: 'Changed profile must not replace snapshot', adapter: profile.adapter }]);
    await shot(cdp, '01-running.png');
    await quit(app);
    const next = await launch(), nextCDP = next.renderer;
    await until(() => nextCDP.evaluate('Boolean(document.querySelector(".force-agent-section"))'), 'agents restored');
    assert.equal(launches().length, 1, 'restore must not start CLI');
    assert.equal(await nextCDP.evaluate('Boolean(document.querySelector(".modal-wrapper"))'), false);
    const restored = (await instances(nextCDP, project))[0];
    assert.equal(restored.oid, original.oid); assert.equal(restored.blockid, original.blockid); assert.equal(restored.tabid, original.tabid);
    assert.equal(restored.claudesessionid, original.session); assert.equal(restored.prompthash, original.hash); assert.equal(restored.profileversion, original.profileversion);
    checked('normal app/backend relaunch restored the same frozen instance without launching');
    await click(nextCDP, 'Reconectar', '.force-agent-actions button');
    await until(async () => launches().length === 2 && (await instances(nextCDP, project))[0].status === 'running', 'exact resume');
    const second = launches()[1];
    assert.equal(second.args[second.args.indexOf('--resume') + 1], original.session);
    assert.equal(second.args.includes('--session-id'), false); assert.equal(second.promptHash, promptHash); assert.equal(second.cwd, projectRoot);
    assert.equal((await instances(nextCDP, project))[0].blockid, original.blockid);
    checked('explicit reconnect resumed the saved UUID/root/snapshot in the same terminal');
    await nextCDP.evaluate(`window.RpcApi.ControllerInputCommand(window.TabRpcClient,{blockid:${JSON.stringify(original.blockid)},inputdata64:btoa('FORCE_FAKE_EXIT\\r')})`);
    await until(async () => (await instances(nextCDP, project))[0].status === 'exited', 'confirmed fake process exit');
    await shot(nextCDP, '02-resumed.png');
    checked('terminal input reached the fake CLI; confirmed exit persisted before normal quit');
    // Catch any accidental mount-driven connection before it can reach SSH.
    // The backend also rejects SSH execution as unavailable in this delivery.
    await nextCDP.evaluate(`window.__forceConnAttempts=[];window.__forceOriginalConnEnsure=window.RpcApi.ConnEnsureCommand;window.RpcApi.ConnEnsureCommand=async(_client,data)=>{window.__forceConnAttempts.push(data.connname);throw new Error('Unexpected connection in inert-agent smoke')};true`);
    const remote = await service(nextCDP, 'SaveProject', [{ id: '', expectedversion: 0, creationkey: crypto.randomUUID(), name: 'Remote inert smoke', icon: 'server', connection: 'nobody@force-smoke.invalid', rootpath: '/srv/force-smoke' }]);
    await selectProject(nextCDP, remote.name);
    await click(nextCDP, 'Novo agente'); await click(nextCDP, 'Criar agente');
    await until(async () => (await instances(nextCDP, remote)).length === 1, 'inert SSH agent');
    await click(nextCDP, 'Abrir terminal', '.force-agent-actions button');
    await until(() => nextCDP.evaluate('!document.querySelector(".force-agent-section [role=status]")'), 'inert SSH focus');
    await delay(500);
    assert.deepEqual(await nextCDP.evaluate('window.__forceConnAttempts'), []);
    assert.equal(launches().length, 2);
    assert.equal((await instances(nextCDP, remote))[0].status, 'prepared');
    assert.equal(await nextCDP.evaluate('document.querySelector(".force-agent-status")?.textContent'), 'Execução indisponível');
    checked('saved SSH agent opened inert terminal without connection, credential prompt, or CLI launch');
    const codex = await service(nextCDP, 'SaveProfile', [{ id: '', expectedversion: 0, creationkey: crypto.randomUUID(), title: 'Codex inert smoke', icon: 'robot', systemprompt: 'Test-owned inert Codex profile', adapter: 'codex' }]);
    await selectProject(nextCDP, project.name); await click(nextCDP, 'Novo agente');
    await until(() => nextCDP.evaluate(`Boolean(document.querySelector('.force-editor select option[value="${codex.oid}"]'))`), 'Codex profile option');
    await nextCDP.evaluate(`(()=>{const select=document.querySelector('.force-editor select');Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype,'value').set.call(select,${JSON.stringify(codex.oid)});select.dispatchEvent(new Event('change',{bubbles:true}));return true})()`);
    await click(nextCDP, 'Criar agente');
    await until(async () => (await instances(nextCDP, project)).length === 2, 'inert Codex agent');
    assert.equal(launches().length, 2);
    const inertCodex = (await instances(nextCDP, project)).find(row => row.adapter === 'codex');
    assert.equal(inertCodex.status, 'prepared'); assert.equal(inertCodex.identityevidence, 'unavailable'); assert.equal(Boolean(inertCodex.claudesessionid), false);
    checked('Codex instance remained explicitly unavailable without fictitious conversation or process');
    await nextCDP.evaluate('window.RpcApi.ConnEnsureCommand=window.__forceOriginalConnEnsure;delete window.__forceOriginalConnEnsure;delete window.__forceConnAttempts;true');
    await quit(next);
    report.stage = 'complete';
    report.passed = true;
} catch (error) {
    report.error = error.message; console.error(`FAIL: ${error.message}`); process.exitCode = 1;
    for (const cdp of connections) { try { await shot(cdp, 'failure.png'); } catch {} }
} finally {
    for (const cdp of connections) cdp.close();
    for (const proc of owned) { try { process.kill(-proc.pid, 'SIGTERM'); } catch {} }
    // Give the backend's bounded controller shutdown time to signal and drain
    // its test-owned PTYs, which have their own process groups.
    if (owned.size) await delay(2500);
    for (const proc of owned) { try { process.kill(-proc.pid, 'SIGKILL'); } catch {} }
    fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
}
