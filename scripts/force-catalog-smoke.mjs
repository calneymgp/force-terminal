import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import net from 'node:net';
import { spawn } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';
import WebSocket from 'ws';

// Use the real Electron app/backend with disposable Force data. No CLI or SSH
// session is started by this test; the remote project is only a saved reference.
const output = path.resolve(process.argv[2] || '/tmp/force-catalog-smoke-results');
const appPath = process.argv[3] ? path.resolve(process.argv[3]) : null;
fs.mkdirSync(output, { recursive: true });
const root = fs.mkdtempSync(path.join(os.tmpdir(), 'force-catalog-smoke-'));
const dirs = Object.fromEntries(['data', 'config', 'cache', 'alpha', 'beta'].map(key => [key, path.join(root, key)]));
for (const dir of Object.values(dirs)) fs.mkdirSync(dir, { mode: 0o700 });
fs.writeFileSync(path.join(dirs.config, 'settings.json'), JSON.stringify({
    'autoupdate:enabled': false, 'term:disablewebgl': true, 'app:confirmquit': false,
    'term:localshellpath': process.platform === 'darwin' ? '/bin/zsh' : '/bin/bash',
    'web:defaulturl': 'https://127.0.0.1:1',
}));
const report = { platform: process.platform, arch: process.arch, packaged: Boolean(appPath), checks: [], passed: false,
    limits: ['No physical M5, real SSH, agent execution/resume or signed update validation.'] };
const owned = new Set();
const connections = new Set();
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
    await new Promise(resolve => server.close(resolve));
    return port;
}
async function connect(url) {
    const ws = new WebSocket(url);
    await new Promise((resolve, reject) => { ws.once('open', resolve); ws.once('error', reject); });
    const pending = new Map();
    let serial = 0;
    const rejectPending = () => { for (const entry of pending.values()) { clearTimeout(entry.timer); entry.reject(new Error('CDP closed')); } pending.clear(); };
    ws.on('close', rejectPending); ws.on('error', rejectPending);
    ws.on('message', raw => {
        const msg = JSON.parse(raw), entry = pending.get(msg.id);
        if (!entry) return;
        pending.delete(msg.id); clearTimeout(entry.timer);
        if (msg.error || msg.result?.exceptionDetails) {
            const detail = msg.error?.message || msg.result?.exceptionDetails?.exception?.description || msg.result?.exceptionDetails?.text;
            entry.reject(new Error(`CDP evaluation failed: ${String(detail).split('\n')[0].replace(/[A-Za-z0-9_-]{24,}(?:\.[A-Za-z0-9_-]+)*/g, '<redacted>')}`));
        }
        else entry.resolve(msg.result);
    });
    const cdp = {
        call(method, params = {}) {
            return new Promise((resolve, reject) => {
                const id = ++serial;
                const timer = setTimeout(() => { pending.delete(id); reject(new Error(`CDP timeout: ${method}`)); }, 12000);
                pending.set(id, { resolve, reject, timer }); ws.send(JSON.stringify({ id, method, params }));
            });
        },
        async evaluate(expression) { return (await this.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })).result?.value; },
        close() { ws.close(); connections.delete(this); },
    };
    connections.add(cdp); return cdp;
}
async function launch() {
    const debugPort = await freePort(), inspectPort = await freePort();
    const executable = appPath ? path.join(appPath, 'Contents/MacOS/Force Terminal') : path.resolve('node_modules/electron/dist/electron');
    assert.ok(fs.existsSync(executable), 'Electron executable missing');
    const env = Object.fromEntries(['PATH', 'HOME', 'USER', 'LOGNAME', 'SHELL', 'LANG', 'LC_ALL', 'TMPDIR', 'TERM', 'DISPLAY', 'XAUTHORITY']
        .filter(key => process.env[key] !== undefined).map(key => [key, process.env[key]]));
    const proc = spawn(executable, ['--no-sandbox', `--inspect=127.0.0.1:${inspectPort}`,
        '--remote-debugging-address=127.0.0.1', `--remote-debugging-port=${debugPort}`, ...(appPath ? [] : [path.resolve('dist/main/index.js')])], {
        detached: true, stdio: ['ignore', 'pipe', 'pipe'], env: { ...env,
            FORCE_TERMINAL_DATA_HOME: dirs.data, FORCE_TERMINAL_CONFIG_HOME: dirs.config,
            FORCE_TERMINAL_CACHE_HOME: dirs.cache, WCLOUD_ENDPOINT: 'https://127.0.0.1:1',
            WCLOUD_PING_ENDPOINT: 'https://127.0.0.1:1', WAVETERM_WAVEAI_ENDPOINT: 'https://127.0.0.1:1' },
    });
    owned.add(proc);
    // Never publish raw logs: app environments can include internal auth material.
    proc.stdout.resume(); proc.stderr.resume();
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
                    if (!renderer || await cdp.evaluate('Boolean(window.WOS && window.globalStore && window.api && window.globalAtoms)')) { result = cdp; return true; }
                    cdp.close();
                }
            } catch { return false; }
            return false;
        }, renderer ? 'renderer ready' : 'main inspector ready');
        return result;
    }
    return { proc, main: await target(inspectPort, false), renderer: await target(debugPort, true), getExit: () => ended };
}
async function click(cdp, label, selector = 'button') {
    const expr = `Array.from(document.querySelectorAll(${JSON.stringify(selector)})).find(e => (e.getAttribute('aria-label') || e.textContent.trim()) === ${JSON.stringify(label)})`;
    await until(() => cdp.evaluate(`Boolean(${expr})`), `button ${label}`);
    assert.equal(await cdp.evaluate(`(()=>{const e=${expr};if(e.disabled)return false;e.click();return true})()`), true, label);
}
async function field(cdp, label, value) {
    const result = await cdp.evaluate(`(()=>{
        const label=Array.from(document.querySelectorAll('.force-editor label')).find(e=>e.textContent.trim()===${JSON.stringify(label)});
        const input=label && document.getElementById(label.htmlFor); if(!input)return false;
        const type=input.tagName==='TEXTAREA'?HTMLTextAreaElement:input.tagName==='SELECT'?HTMLSelectElement:HTMLInputElement;
        Object.getOwnPropertyDescriptor(type.prototype,'value').set.call(input,${JSON.stringify(value)});
        input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new Event('change',{bubbles:true}));return true;
    })()`);
    assert.equal(result, true, `field ${label}`); await delay(50);
}
async function submitTwice(cdp) {
    assert.equal(await cdp.evaluate(`(()=>{const form=document.querySelector('.force-editor form');if(!form)return false;form.requestSubmit();form.requestSubmit();return true;})()`), true);
}
async function selectProject(cdp, name) {
    assert.equal(await cdp.evaluate(`(()=>{const button=Array.from(document.querySelectorAll('.force-list .force-item-main')).find(e=>e.querySelector('strong')?.textContent===${JSON.stringify(name)});if(!button)return false;button.click();return true;})()`), true);
}
async function focusedEditor(cdp) {
    await cdp.evaluate(`(()=>{const buttons=document.querySelectorAll('.force-editor button:not(:disabled)');buttons[buttons.length-1].focus();})()`);
    await cdp.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 });
    await cdp.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 });
    assert.equal(await cdp.evaluate('document.activeElement === document.querySelector(".force-editor button:not(:disabled)")'), true);
    await cdp.call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9, modifiers: 8 });
    await cdp.call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Tab', code: 'Tab', windowsVirtualKeyCode: 9 });
    assert.equal(await cdp.evaluate(`(()=>{const buttons=document.querySelectorAll('.force-editor button:not(:disabled)');return document.activeElement===buttons[buttons.length-1];})()`), true);
}
const catalog = cdp => cdp.evaluate('window.WOS.callBackendService("force","GetCatalog",[],true)');
// The bundled main process is ESM; its inspector has no global require.
const electronExpression = `process.getBuiltinModule('module').createRequire(${JSON.stringify(path.resolve('dist/main/index.js'))})('electron')`;
async function noEditor(cdp) { await until(() => cdp.evaluate('!document.querySelector(".force-editor")'), 'editor saved'); }
async function shot(cdp, name) { fs.writeFileSync(path.join(output, name), Buffer.from((await cdp.call('Page.captureScreenshot', { format: 'png' })).data, 'base64')); }
async function prepare(app) {
    return app.main.evaluate(`(async()=>{
        const electron=${electronExpression};
        let view;
        for(const candidate of electron.webContents.getAllWebContents().filter(w=>w.getURL().includes('/index.html'))){
            if(await candidate.executeJavaScript('Boolean(document.querySelector(".force-sidebar"))')){view=candidate;break;}
        }
        return new Promise((resolve,reject)=>{
        const id='force-catalog-smoke-'+Date.now();
        if(!view){reject(new Error('View missing'));return;}
        const listener=(_event,requestId,result)=>{if(requestId===id){clearTimeout(timer);electron.ipcMain.removeListener('update-prepare-reply',listener);resolve(result);}};
        const timer=setTimeout(()=>{electron.ipcMain.removeListener('update-prepare-reply',listener);reject(new Error('Preparation timed out'));},9000);
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
try {
    const app = await launch(), cdp = app.renderer;
    assert.equal(await cdp.evaluate('window.api.getDataDir()'), dirs.data);
    await until(() => cdp.evaluate('document.body.innerText.includes("Welcome to Force Terminal")'), 'welcome');
    await click(cdp, 'Continue');
    await until(() => cdp.evaluate('Boolean(document.querySelector(".force-sidebar")) && !document.querySelector(".modal-wrapper")'), 'catalog sidebar');
    const initial = await catalog(cdp);
    assert.deepEqual(initial.projects, []); assert.deepEqual(initial.profiles, []);
    // False booleans use omitempty in the Go settings JSON; the renderer uses
    // the same boolean coercion for its telemetry checkbox.
    assert.equal(await cdp.evaluate('Boolean(window.globalStore.get(window.globalAtoms.fullConfigAtom)?.settings?.["telemetry:enabled"])'), false);
    checked('real app/backend loaded isolated empty catalog with telemetry disabled');
    await click(cdp, 'Novo projeto');
    await focusedEditor(cdp);
    await field(cdp, 'Nome do projeto', 'Alpha'); await field(cdp, 'Pasta raiz', path.join(root, 'missing'));
    await click(cdp, 'Salvar projeto');
    await until(() => cdp.evaluate('Boolean(document.querySelector(".force-editor [role=alert]"))'), 'invalid directory error');
    assert.equal((await catalog(cdp)).projects.length, 0);
    assert.ok((await prepare(app)).reasons.some(reason => /projeto/i.test(reason)));
    await field(cdp, 'Pasta raiz', dirs.alpha); await submitTwice(cdp); await noEditor(cdp);
    await until(() => cdp.evaluate('document.querySelector(".force-list [aria-current=true] strong")?.textContent === "Alpha"'), 'new project automatically selected');
    assert.equal(await cdp.evaluate('Boolean(document.querySelector(".force-agent-section"))'), true);
    assert.equal((await prepare(app)).reasons.some(reason => /projeto|perfil/i.test(reason)), false);
    for (const [name, cwd] of [['Beta', dirs.beta], ['Ops', '/srv/force-smoke']]) {
        await click(cdp, 'Novo projeto'); await field(cdp, 'Nome do projeto', name);
        if (name === 'Ops') { await field(cdp, 'Local/SSH', 'ssh'); await field(cdp, 'Conexão SSH', 'nobody@force-smoke.invalid'); }
        await field(cdp, 'Pasta raiz', cwd);
        if (name === 'Beta') {
            await cdp.evaluate(`window.__forceCreateSetMeta=window.RpcApi.SetMetaCommand;window.RpcApi.SetMetaCommand=async()=>{throw new Error('New project selection failure (smoke)')};true`);
        }
        await click(cdp, 'Salvar projeto');
        if (name === 'Beta') {
            await until(() => cdp.evaluate('document.querySelector(".force-editor [role=alert]")?.textContent.includes("Projeto salvo, mas a seleção falhou")'), 'saved project selection error');
            const beforeRetry = (await catalog(cdp)).projects.find(project => project.name === 'Beta');
            assert.ok(beforeRetry);
            assert.equal(await cdp.evaluate('document.querySelector(".force-list [aria-current=true] strong")?.textContent'), 'Alpha');
            assert.equal(await cdp.evaluate('document.querySelector(".force-editor input")?.disabled'), true);
            await cdp.evaluate('window.RpcApi.SetMetaCommand=window.__forceCreateSetMeta;delete window.__forceCreateSetMeta;true');
            await click(cdp, 'Tentar selecionar');
            await noEditor(cdp);
            const afterRetry = (await catalog(cdp)).projects.filter(project => project.name === 'Beta');
            assert.equal(afterRetry.length, 1);
            assert.deepEqual(afterRetry[0], beforeRetry);
            checked('selection retry retained the saved project without another save or duplicate');
        }
        await noEditor(cdp);
        await until(() => cdp.evaluate(`document.querySelector('.force-list [aria-current=true] strong')?.textContent === ${JSON.stringify(name)}`), 'saved project automatically selected');
    }
    checked('new projects became selected immediately and revealed the agent action');
    checked('UI created three local/SSH projects; invalid path kept form and blocked update');
    checked('keyboard focus stayed in dialog; repeated submit created one project');
    await selectProject(cdp, 'Alpha');
    await until(() => cdp.evaluate('document.querySelector(".force-list [aria-current=true] strong")?.textContent === "Alpha"'), 'project selected');
    await cdp.evaluate(`window.__forceOriginalSetMeta=window.RpcApi.SetMetaCommand;window.RpcApi.SetMetaCommand=async()=>{throw new Error('Selection persistence failure (smoke)')};true`);
    await selectProject(cdp, 'Beta');
    await until(() => cdp.evaluate('document.querySelector(".force-sidebar").innerText.includes("A seleção não foi salva")'), 'selection error');
    assert.equal(await cdp.evaluate('document.querySelector(".force-list [aria-current=true] strong")?.textContent'), 'Alpha');
    await cdp.evaluate('window.RpcApi.SetMetaCommand=window.__forceOriginalSetMeta;delete window.__forceOriginalSetMeta;true');
    await selectProject(cdp, 'Beta');
    await until(() => cdp.evaluate('document.querySelector(".force-list [aria-current=true] strong")?.textContent === "Beta"'), 'selection retry');
    checked('selection changed only after persistence; failed save kept prior project selected');
    for (const name of ['DevOps', 'ETL de dados', 'Marketing']) {
        await click(cdp, 'Novo perfil'); await click(cdp, name, '.force-presets button');
        await field(cdp, 'System prompt', `Papel ${name}: confirme contexto e peça revisão quando necessário.`);
        await submitTwice(cdp); await noEditor(cdp);
    }
    const saved = await catalog(cdp);
    assert.equal(saved.projects.length, 3); assert.equal(saved.profiles.length, 3);
    assert.equal(saved.projects.find(project => project.name === 'Ops').connection, 'nobody@force-smoke.invalid');
    checked('UI saved reusable DevOps/ETL/Marketing profiles with distinct prompts');
    for (const width of [1440, 900]) {
        await app.main.evaluate(`(()=>{const window=${electronExpression}.BaseWindow.getAllWindows()[0];window.setBounds({...window.getBounds(),width:${width},height:960});return true;})()`);
        await delay(300);
        const bounds = await cdp.evaluate(`(()=>{const sidebar=document.querySelector('.force-sidebar').getBoundingClientRect(),layout=document.querySelector('[data-testid=force-layout]').getBoundingClientRect();return {sidebarWidth:sidebar.width,sidebarRight:sidebar.right,layoutLeft:layout.left,layoutRight:layout.right,viewport:window.innerWidth};})()`);
        assert.ok(bounds.sidebarWidth >= 200 && bounds.sidebarWidth <= 266, `sidebar at ${width}`);
        assert.ok(bounds.layoutLeft >= bounds.sidebarRight - 1 && bounds.layoutRight <= bounds.viewport + 1, `layout fits at ${width}`);
        await shot(cdp, `layout-${width}.png`);
    }
    checked('real window layout fit at 1440 and 900 pixels without sidebar overlap');
    await shot(cdp, '01-profiles.png');
    await click(cdp, 'Editar projeto Alpha');
    await click(cdp, 'Arquivar'); await noEditor(cdp);
    await click(cdp, 'Mostrar arquivados'); await click(cdp, 'Editar projeto Alpha');
    await click(cdp, 'Reativar'); await noEditor(cdp);
    const beforeQuit = await catalog(cdp);
    assert.equal(beforeQuit.projects.find(project => project.name === 'Alpha').archived, false);
    const alpha = beforeQuit.projects.find(project => project.name === 'Alpha');
    const originalWorkspace = await cdp.evaluate('window.globalStore.get(window.globalAtoms.workspace).oid');
    await cdp.evaluate('window.api.openNewWindow();true');
    // New windows may reuse preloaded WebContents; identify their workspace,
    // rather than assuming the renderer must have a newly allocated ID.
    let secondView;
    await until(async () => {
        secondView = await app.main.evaluate(`(async()=>{
            for(const view of ${electronExpression}.webContents.getAllWebContents().filter(view=>view.getURL().includes('/index.html'))){
                if(await view.executeJavaScript(${JSON.stringify(`Boolean(document.querySelector('.force-sidebar')) && window.globalStore.get(window.globalAtoms.workspace)?.oid !== ${JSON.stringify(originalWorkspace)}`)}))return view.id;
            }
        })()`);
        return Boolean(secondView);
    }, 'second window catalog');
    const inSecondView = expression => app.main.evaluate(`${electronExpression}.webContents.fromId(${secondView}).executeJavaScript(${JSON.stringify(expression)})`);
    await until(() => inSecondView('document.querySelector(".force-sidebar")?.innerText.includes("Alpha")'), 'catalog in second window');
    await inSecondView(`window.WOS.callBackendService('force','SaveProject',[${JSON.stringify({ id: alpha.oid, expectedversion: alpha.version, creationkey: '', name: 'Alpha revisado', icon: alpha.icon, connection: alpha.connection, rootpath: alpha.rootpath })}],true)`);
    await until(() => cdp.evaluate('document.querySelector(".force-sidebar").innerText.includes("Alpha revisado")'), 'event refreshed sidebar');
    await until(() => inSecondView('document.querySelector(".force-sidebar").innerText.includes("Alpha revisado")'), 'event refreshed second sidebar');
    await inSecondView(`Array.from(document.querySelectorAll('.force-list .force-item-main')).find(e=>e.querySelector('strong')?.textContent==='Beta').click();true`);
    await until(() => inSecondView('document.querySelector(".force-list [aria-current=true] strong")?.textContent === "Beta"'), 'second window selected project');
    checked('archive/reactivate preserved identity; save from second window refreshed both sidebars');
    await shot(cdp, '02-projects.png');
    const durable = await catalog(cdp);
    await quit(app);
    const next = await launch();
    await until(() => next.renderer.evaluate('document.querySelector(".force-sidebar")?.innerText.includes("Alpha revisado")'), 'catalog restored');
    assert.equal(await next.renderer.evaluate('Boolean(document.querySelector(".modal-wrapper"))'), false);
    assert.deepEqual(await catalog(next.renderer), durable);
    assert.equal(await next.renderer.evaluate('document.querySelector(".force-list [aria-current=true] strong")?.textContent'), 'Beta');
    await shot(next.renderer, '03-reopened.png'); await quit(next);
    checked('same IDs, versions, projects and profiles survived normal app/backend relaunch');
    report.passed = true;
} catch (error) {
    report.error = error.message; console.error(`FAIL: ${error.message}`); process.exitCode = 1;
    for (const cdp of connections) { try { await shot(cdp, 'failure.png'); } catch {} }
} finally {
    for (const cdp of connections) cdp.close();
    for (const proc of owned) { try { process.kill(-proc.pid, 'SIGTERM'); } catch {} }
    await delay(1000);
    for (const proc of owned) { try { process.kill(-proc.pid, 'SIGKILL'); } catch {} }
    fs.rmSync(root, { recursive: true, force: true });
    fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2));
}
