// Bird — standalone (Oracle-hosted) build, derived from bird.js v3.3.0
// Adapted: removed Tampermonkey/GM_* deps, removed CroxyProxy iframe path,
// rewrote endpoint URLs to relative /bird/* paths.
// Original: D:\gartic\bird\bird.js

// === Browser-environment shims (replacing Tampermonkey APIs) ===
const W = window;
const _gmStore = {};
const _gmListeners = {};
function GM_addStyle(css){const s=document.createElement('style');s.textContent=css;document.head.appendChild(s);return s;}
function GM_setValue(k,v){const old=_gmStore[k];_gmStore[k]=v;try{localStorage.setItem('bird_gm_'+k,JSON.stringify(v));}catch(e){}if(!Object.is(old,v))(_gmListeners[k]||[]).forEach(fn=>fn(k,old,v,false));}
function GM_getValue(k,d){if(k in _gmStore)return _gmStore[k];try{const r=localStorage.getItem('bird_gm_'+k);if(r!==null){const p=JSON.parse(r);_gmStore[k]=p;return p;}}catch(e){}return d;}
function GM_addValueChangeListener(k,fn){(_gmListeners[k]=_gmListeners[k]||[]).push(fn);return _gmListeners[k].length-1;}
function GM_removeValueChangeListener(){}
function GM_getResourceText(){return '';}
const GM_info = {scriptHandler:'StandaloneBrowser', version:'1.0.0'};
function GM_xmlhttpRequest(opts){
  const init = { method: opts.method || 'GET', headers: opts.headers || {}, credentials: opts.anonymous ? 'omit' : 'same-origin' };
  if (opts.data) init.body = opts.data;
  fetch(opts.url, init).then(async r => {
    const text = await r.text();
    const respHeaders = [];
    r.headers.forEach((v,k) => respHeaders.push(k+': '+v));
    const resp = { status: r.status, statusText: r.statusText, response: text, responseText: text, responseHeaders: respHeaders.join('\r\n'), finalUrl: r.url };
    opts.onload && opts.onload(resp);
    opts.onloadend && opts.onloadend(resp);
  }).catch(e => {
    opts.onerror && opts.onerror({ error: String(e) });
    opts.onloadend && opts.onloadend({ error: String(e) });
  });
  return { abort(){} };
}
const _bus = new EventTarget();
function GM_scriptSendMessage(c,...d){_bus.dispatchEvent(new CustomEvent(c,{detail:{data:d}}));}
function GM_scriptOnMessage(c,f){_bus.addEventListener(c, e => f && f(...e.detail.data));}

// One-time migration: pastel_gm_PastelLive-* localStorage → bird_gm_Bird-*, and pastel-fv-snapshot:* → bird-fv-snapshot:*.
// Runs once on first load after the rename so existing users don't lose stored proxies/tracked-users/snapshots.
(()=>{try{
    if(localStorage.getItem('bird_gm_Bird-MigratedFromLegacy')==='true') return;
    const exact={
        'pastel_gm_PastelLive-Proxies':'bird_gm_Bird-Proxies',
        'pastel_gm_PastelLive-TrackedUsers':'bird_gm_Bird-TrackedUsers'
    };
    let n=0;
    for(const [oldK,newK] of Object.entries(exact)){
        const v=localStorage.getItem(oldK);
        if(v!==null){localStorage.setItem(newK,v);localStorage.removeItem(oldK);n++;}
    }
    const so='pastel-fv-snapshot:',sn='bird-fv-snapshot:',mig=[];
    for(let i=0;i<localStorage.length;i++){const k=localStorage.key(i);if(k&&k.startsWith(so))mig.push(k);}
    for(const k of mig){const v=localStorage.getItem(k);localStorage.setItem(sn+k.slice(so.length),v);localStorage.removeItem(k);n++;}
    localStorage.setItem('bird_gm_Bird-MigratedFromLegacy','true');
    if(n)console.log('[bird] migrated',n,'legacy keys');
}catch(e){}})();
const unsafeWindow = window;
const baseHeaders={Accept:"*/*","Content-Type":"application/x-www-form-urlencoded","sec-ch-ua-mobile":"?0","sec-ch-ua-platform":"\"Windows\"","sec-fetch-dest":"document","sec-fetch-mode":"navigate","sec-fetch-site":"none","sec-fetch-user":"?1","upgrade-insecure-requests":"1",Origin:"https://gartic.io",Referer:"https://gartic.io/"};

const [GM_onMessage,GM_sendMessage,getCookie,onBodyReady,observer,observer2,rand,GM_req,enc]=[
    (k,c)=>GM_addValueChangeListener(k,(_,__,o)=>c(...o)),
    (k,...d)=>GM_setValue(k,d),
    c=>document.cookie.split("; ").find(e=>e.startsWith(c+"="))?.split("=")[1],
    cb=>document.body?cb():new MutationObserver((_,o)=>document.body&&(o.disconnect(),cb())).observe(document.documentElement,{childList:1}),
    (s,c)=>{const w=()=>{const e=document.querySelector(s);e&&(c(e),o.disconnect())};const o=new MutationObserver(w);document.body?o.observe(document.body,{childList:1,subtree:1}):new MutationObserver((_,m)=>{document.body&&(m.disconnect(),o.observe(document.body,{childList:1,subtree:1}))}).observe(document.documentElement,{childList:1})},
    (s,c)=>{const w=()=>{const e=document.querySelector(s);if(e){c(e);o.disconnect()}};const o=new MutationObserver(w);o.observe(document.documentElement,{childList:true,subtree:true})},
    _=>Math.floor(Math.random()*100)+1,
    q=>GM_xmlhttpRequest({...q,onerror:e=>console.error(e)}),
    ()=>{const t=/^[A-Za-z0-9+/]+={0,2}$/;function n(t,n,r,e){const o=function(t){let n=t>>>0;return function(){return n^=n<<13,n>>>=0,n^=n>>>17,n>>>=0,n^=n<<5,n>>>=0,n>>>0}}(function(t){let n=5381;for(let r=0;r<t.length;r++)n=(n<<5)+n^t.charCodeAt(r),n>>>=0;return n||1}(t+"|"+String.fromCharCode(...n)+"|"+String.fromCharCode(...r))),c=new Uint8Array(e);for(let t=0;t<e;t++)c[t]=255&o();return c}function r(t,n){const r=new Uint8Array(t.length);for(let e=0;e<t.length;e++)r[e]=t[e]^n[e%n.length];return r}function e(t){const n=new Uint8Array(t);return crypto.getRandomValues(n),n}return function(o,c,f={}){if("string"!=typeof o)throw new TypeError("text must be a string");if("string"!=typeof c||0===c.length)throw new TypeError("password must be non-empty string");if(t.test(o)){const t=function(t){try{const n=atob(t),r=new Uint8Array(n.length);for(let t=0;t<n.length;t++)r[t]=n.charCodeAt(t);return r.buffer}catch(t){return null}}(o);if(t){const e=new Uint8Array(t);if(e.length>=17){const t=e.slice(0,8),o=e.slice(8,16),f=e.slice(16),u=r(f,n(c,t,o,f.length)),s=(i=u,(new TextDecoder).decode(i));if(/^[\t\n\r\x20-\x7E]*$/.test(s)&&s.length>0)return s}}}var i;const u=e(8),s=e(8),a=(l=o,(new TextEncoder).encode(l));var l;const g=r(a,n(c,u,s,a.length)),h=function(...t){const n=t.reduce(((t,n)=>t+n.byteLength),0),r=new Uint8Array(n);let e=0;for(const n of t)r.set(new Uint8Array(n),e),e+=n.byteLength;return r.buffer}(u.buffer,s.buffer,g.buffer);return y=h,btoa(String.fromCharCode(...new Uint8Array(y)));var y}}
];

// CroxyProxy iframe handlers were removed — Bird now uses Webshare premium
// proxies exclusively (HTTP CONNECT direct to gartic.io via the local Go server).

// =========================================================================
// === Fast Viewer mode ===
// Triggered by gartic.io/live#fv=<ROOMCODE>. Stays on the /live URL with
// the hash intact — rewriting the path to /<CODE>/viewer crashes gartic's
// Next.js router ("An unexpected error has occurred"), which the ICEbot
// extension's crash detector then reloads out of. Our CSS hides gartic's
// /live UI; the FastViewer class below renders our own fast UI on top.
// =========================================================================
class FastViewer {
    constructor(roomCode) {
        this.roomCode = roomCode;                // e.g., "491KS4"
        this.roomId = roomCode.substring(2);     // gartic's "sala" field drops the 2-char server prefix
        this.players = new Map();                // garticId -> { nick, avatar, foto, pts, isDrawer, isBot }
        this.chat = [];                          // ring buffer, capped at 200
        this.turn = null;                        // { drawerId, wordMask, startedAt, durationMs, turnNum, totalTurns }
        this.botIds = new Set();                 // garticIds of bots (populated via Go control WS in Task 12)
        this.controlSocket = null;               // Go server control WS (bot-list only)
        this.language = null;
        this.serverName = null;
        this.playerLimit = null;
        this.timerRaf = null;
        this.botPollInterval = null;
    }

    injectStyles() {
        if (document.getElementById('bird-fv-styles')) return;
        const css = `
            html.bird-fv, body.bird-fv { margin:0; padding:0; background:#0F172A; color:#E2E8F0;
                font-family: 'Plus Jakarta Sans', system-ui, sans-serif; overflow:hidden; }
            body.bird-fv > *:not(#bird-fv-root):not(#bird-fv-styles) { display:none !important; }
            #bird-fv-root { position:fixed; inset:0; display:grid;
                grid-template-rows: 48px 1fr 8px;
                grid-template-columns: 320px 1fr;
                grid-template-areas: "header header" "players main" "timer timer";
                gap: 0; }
            #bird-fv-header { grid-area: header; display:flex; align-items:center; gap:16px;
                padding:0 20px; background:#1E293B; border-bottom:1px solid #334155;
                font-size:13px; font-weight:500; }
            #bird-fv-header .sep { color:#475569; }
            #bird-fv-players { grid-area: players; overflow-y:auto; background:#0B1120;
                border-right:1px solid #334155; padding:8px; }
            #bird-fv-players h3 { font-size:11px; text-transform:uppercase; letter-spacing:0.1em;
                color:#94A3B8; margin:8px 8px 12px; }
            .pfv-player-row { display:flex; align-items:center; gap:10px; padding:8px;
                border-radius:8px; margin-bottom:4px; background:#1E293B; position:relative; }
            .pfv-player-row.drawer { outline:2px solid #0EA5E9; }
            .pfv-player-row .avatar { width:40px; height:40px; border-radius:8px; flex-shrink:0; background:#334155; }
            .pfv-player-row .meta { flex:1; min-width:0; }
            .pfv-player-row .nick { font-size:13px; font-weight:500; white-space:nowrap;
                overflow:hidden; text-overflow:ellipsis; }
            .pfv-player-row .pts { font-size:11px; color:#94A3B8; }
            .pfv-player-row .kick-btn { width:28px; height:28px; border:none; background:transparent;
                cursor:pointer; font-size:18px; flex-shrink:0; opacity:0.7; }
            .pfv-player-row .kick-btn:hover { opacity:1; }
            .pfv-player-row .role-icon { position:absolute; top:4px; right:4px; font-size:11px; }
            #bird-fv-main { grid-area: main; display:grid; grid-template-rows: auto 1fr; gap:0; }
            #bird-fv-turn { padding:16px 20px; background:#1E293B; border-bottom:1px solid #334155; }
            #bird-fv-turn .drawer-line { font-size:13px; color:#94A3B8; margin-bottom:6px; }
            #bird-fv-turn .word-mask { font-size:22px; font-weight:600; letter-spacing:0.15em; font-family:monospace; }
            #bird-fv-turn .timer-line { font-size:12px; color:#94A3B8; margin-top:8px; }
            #bird-fv-chat { overflow-y:auto; padding:12px 20px; background:#0B1120; }
            .pfv-chat-row { font-size:13px; margin-bottom:6px; line-height:1.4; }
            .pfv-chat-row .author { font-weight:500; color:#38BDF8; }
            .pfv-chat-row.system { color:#94A3B8; font-style:italic; font-size:12px; }
            #bird-fv-timer { grid-area: timer; background:#334155; position:relative; }
            #bird-fv-timer .fill { position:absolute; inset:0 auto 0 0; background:#0EA5E9;
                transition: width 250ms linear; width:0%; }
            .pfv-banner { position:fixed; top:8px; left:50%; transform:translateX(-50%);
                padding:8px 16px; border-radius:8px; font-size:13px; z-index:9999;
                background:#B91C1C; color:white; box-shadow:0 4px 12px rgba(0,0,0,0.5); }
            .pfv-banner.info { background:#0E7490; }
            .pfv-banner button { margin-left:12px; background:white; color:#0F172A; border:none;
                padding:4px 10px; border-radius:4px; cursor:pointer; font-weight:500; }
        `;
        const style = document.createElement('style');
        style.id = 'bird-fv-styles';
        style.textContent = css;
        (document.head || document.documentElement).appendChild(style);
    }

    renderSkeleton() {
        if (document.getElementById('bird-fv-root')) return;
        const root = document.createElement('div');
        root.id = 'bird-fv-root';
        root.innerHTML = `
            <div id="bird-fv-header">
                <span id="pfv-h-room">${this.roomCode}</span>
                <span class="sep">·</span>
                <span id="pfv-h-lang">—</span>
                <span class="sep">·</span>
                <span id="pfv-h-players">0/0 players</span>
                <span class="sep">·</span>
                <span id="pfv-h-turn">Turn —</span>
                <span class="sep">·</span>
                <span id="pfv-h-server">Server —</span>
            </div>
            <div id="bird-fv-players">
                <h3>Players (<span id="pfv-pcount">0</span>)</h3>
                <div id="pfv-player-list"></div>
            </div>
            <div id="bird-fv-main">
                <div id="bird-fv-turn">
                    <div class="drawer-line">Waiting for turn…</div>
                    <div class="word-mask">—</div>
                    <div class="timer-line">⏱ —</div>
                </div>
                <div id="bird-fv-chat"></div>
            </div>
            <div id="bird-fv-timer"><div class="fill"></div></div>
        `;
        document.body.appendChild(root);
        document.getElementById('pfv-player-list').addEventListener('click', e => {
            const btn = e.target.closest('.kick-btn');
            if (!btn) return;
            const id = Number(btn.getAttribute('data-kick'));
            if (!id) return;
            window.postMessage('kickuser.' + id, '*');
            btn.disabled = true;
            btn.textContent = '⏳';
            setTimeout(() => { btn.textContent = '😀'; btn.disabled = false; }, 2000);
        });
    }

    showBanner(kind, text, actionLabel, actionFn) {
        this.hideBanner();
        const b = document.createElement('div');
        b.className = 'pfv-banner' + (kind === 'info' ? ' info' : '');
        b.id = 'bird-fv-banner';
        b.textContent = text;
        if (actionLabel && actionFn) {
            const btn = document.createElement('button');
            btn.textContent = actionLabel;
            btn.onclick = () => { this.hideBanner(); actionFn(); };
            b.appendChild(btn);
        }
        document.body.appendChild(b);
    }
    hideBanner() {
        document.getElementById('bird-fv-banner')?.remove();
    }

    updateHeader() {
        const langNames = {1:'Portuguese',2:'English',3:'Spanish',4:'French',6:'Italian',
            7:'Russian',10:'Polish',14:'German',15:'Japanese',19:'Arabic'};
        const lang = this.language;
        const langStr = lang ? `${langNames[lang] || 'lang '+lang} (lang ${lang})` : '—';
        const el = id => document.getElementById(id);
        if (el('pfv-h-lang')) el('pfv-h-lang').textContent = langStr;
        if (el('pfv-h-players')) el('pfv-h-players').textContent =
            `${this.players.size}/${this.playerLimit ?? '?'} players`;
        if (el('pfv-h-server')) el('pfv-h-server').textContent =
            this.serverName ? `Server ${this.serverName.replace('server','')}` : 'Server —';
        if (el('pfv-h-turn')) el('pfv-h-turn').textContent = this.turn
            ? `Turn ${this.turn.turnNum}${this.turn.totalTurns?'/'+this.turn.totalTurns:''}`
            : 'Turn —';
        if (el('pfv-pcount')) el('pfv-pcount').textContent = String(this.players.size);
    }

    connectViaChannel() {
        try {
            this.channel = new BroadcastChannel('bird-fv:' + this.roomCode);
        } catch (err) {
            console.warn('[bird-fv] BroadcastChannel unavailable', err);
            this.channel = null;
            return;
        }
        this.channel.onmessage = ev => {
            const m = ev.data;
            if (!m || m.type !== 'wsMessage' || typeof m.data !== 'string') return;
            const raw = m.data;
            // Match the live page's filter: skip pings (FV doesn't pong), only act
            // on socket.io '42' frames. Live page already responded to '2' for us.
            if (raw === '2') return;
            if (!raw.startsWith('42')) return;
            let data;
            try { data = JSON.parse(raw.slice(2)); } catch { return; }
            if (!this._channelGotFirstMessage) {
                this._channelGotFirstMessage = true;
                this.hideBanner();    // clear "open bird live" banner — events are flowing
            }
            this.handleGameMessage(data);
        };
        console.log('[bird-fv] subscribed to channel bird-fv:' + this.roomCode);
    }

    handleGameMessage(data) {
        const code = String(data[0]);
        if (code === '5')  return this.onEvent5(data);
        if (code === '11' || code === '13') return this.onEvent11(data);
        if (code === '23') return this.onEvent23(data);
        if (code === '24') return this.onEvent24(data);
        if (code === '17') return this.onEvent17(data);
        if (code === '22') return this.onEvent22(data);
        if (code === '27') return this.onEvent27(data);
        if (code === '29') return this.onEvent29(data);
        if (code === '6') return this.onEvent6(data);
        if (!['30','16','10'].includes(code)) {
            console.log('[bird-fv] unhandled event', code, data);
        }
    }

    onEvent6(data) {
        const sub = data[1];
        const msgs = {
            3: 'Room is full.',
            4: 'Already connected (another tab?).',
            6: 'Room unreachable (code or server mismatch).',
        };
        const msg = msgs[sub] ?? `Room error (code ${sub ?? 'unknown'}).`;
        this.showBanner('error', msg, 'Retry', () => location.reload());
        cancelAnimationFrame(this.timerRaf);
    }

    addSystemChat(text) {
        this.chat.push({ authorId: 0, nick: null, text, isSystem: true, ts: Date.now() });
        if (this.chat.length > 5000) this.chat.shift();
        // renderChat() lands in Task 8; no-op until then.
        if (typeof this.renderChat === 'function') this.renderChat();
    }

    chatRowHTML(row) {
        if (row.isSystem) {
            return `<div class="pfv-chat-row system">— ${this.escapeHTML(row.text)}</div>`;
        }
        const p = this.players.get(row.authorId);
        const nick = p?.nick ?? row.nick ?? `#${row.authorId}`;
        return `<div class="pfv-chat-row"><span class="author">${this.escapeHTML(nick)}:</span> ${this.escapeHTML(row.text)}</div>`;
    }

    renderChat() {
        const box = document.getElementById('bird-fv-chat');
        if (!box) return;
        const near = box.scrollTop + box.clientHeight >= box.scrollHeight - 40;
        box.innerHTML = this.chat.map(r => this.chatRowHTML(r)).join('');
        if (near) box.scrollTop = box.scrollHeight;
    }

    onEvent11(data) {
        const authorId = data[1];
        const text = String(data[2] ?? '');
        this.chat.push({ authorId, nick: null, text, isSystem: false, ts: Date.now() });
        if (this.chat.length > 5000) this.chat.shift();
        this.renderChat();
    }

    onEvent23(data) {
        const p = data[1];
        if (!p || typeof p !== 'object' || !p.id) return;
        p.foto ||= `https://gartic.io/static/images/avatar/svg/${p.avatar}.svg`;
        this.players.set(p.id, { id: p.id, nick: p.nick, avatar: p.avatar, foto: p.foto, pts: p.pts ?? 0 });
        this.renderPlayers();
        this.addSystemChat(`${p.nick} joined`);
    }

    onEvent24(data) {
        const id = data[1];
        const p = this.players.get(id);
        if (!p) return;
        this.players.delete(id);
        this.renderPlayers();
        this.addSystemChat(`${p.nick} left`);
        // Trigger ICEbot's existing client-side rejoin. content_script.js handles
        // _autoRejoinDupe with a 500ms debounce + per-userId dedup, so it's safe
        // to fire on every leave (including bursts when multiple players leave).
        try { window.postMessage('_autoRejoinDupe', '*'); } catch (_) {}
    }

    onEvent5(data) {
        const arr = Array.isArray(data[5]) ? data[5] : [];
        this.players.clear();
        for (const p of arr) {
            this.players.set(p.id, {
                id: p.id, nick: p.nick, avatar: p.avatar, foto: p.foto, pts: p.pts ?? 0
            });
        }
        // Event 5 also carries room config at data[4] per reverse-engineered spec §5.
        // data[4] commonly has { limite, idioma, tipo, ... }. Update if present.
        if (data[4] && typeof data[4] === 'object') {
            if (data[4].limite) this.playerLimit = data[4].limite;
            if (data[4].idioma) this.language = data[4].idioma;
        }
        this.renderPlayers();
        console.log('[bird-fv] event 5:', this.players.size, 'players');
    }

    onEvent17(data) {
        // Defensive parse: find a string (word/mask), a number (duration seconds),
        // and an id matching a player (drawerId).
        let wordMask = '', durationSec = 60, drawerId = null, turnNum = null, totalTurns = null;
        for (let i = 1; i < data.length; i++) {
            const v = data[i];
            if (typeof v === 'string' && !wordMask) wordMask = v;
            else if (typeof v === 'number' && this.players.has(v) && drawerId === null) drawerId = v;
            else if (typeof v === 'number' && v > 10 && v < 600 && durationSec === 60) durationSec = v;
            else if (typeof v === 'number' && turnNum === null) turnNum = v;
            else if (typeof v === 'number' && totalTurns === null) totalTurns = v;
        }
        this.turn = {
            drawerId,
            wordMask,
            startedAt: Date.now(),
            durationMs: durationSec * 1000,
            turnNum, totalTurns
        };
        this.renderPlayers();       // re-render to add drawer highlight
        this.renderTurn();
        this.startTimerLoop();
        const drawer = this.players.get(drawerId);
        if (drawer) this.addSystemChat(`${drawer.nick} is drawing`);
    }

    onEvent22(data) {
        if (!this.turn) return;
        // Form A: [22, "new mask"]
        if (typeof data[1] === 'string') {
            this.turn.wordMask = data[1];
        }
        // Form B: [22, index, letter] — splice into existing mask
        else if (typeof data[1] === 'number' && typeof data[2] === 'string') {
            const idx = data[1];
            const ch = data[2];
            const mask = this.turn.wordMask;
            if (idx >= 0 && idx < mask.length) {
                this.turn.wordMask = mask.slice(0, idx) + ch + mask.slice(idx + 1);
            }
        }
        this.renderTurn();
    }

    onEvent27(data) {
        // data commonly: [27, guesserId, answer] or [27, {scores: {...}, answer: "..."}]
        let answer = null, guesserId = null;
        if (typeof data[1] === 'number') {
            guesserId = data[1];
            if (typeof data[2] === 'string') answer = data[2];
        } else if (data[1] && typeof data[1] === 'object') {
            answer = data[1].answer ?? data[1].palavra ?? null;
            if (data[1].scores) {
                for (const [id, pts] of Object.entries(data[1].scores)) {
                    const p = this.players.get(Number(id));
                    if (p) p.pts = pts;
                }
                this.renderPlayers();
            }
        }
        if (guesserId) {
            const g = this.players.get(guesserId);
            if (g) this.addSystemChat(`${g.nick} guessed the word`);
        }
        if (answer) this.addSystemChat(`Word was: ${answer}`);
    }

    onEvent29(data) {
        // Time up — reveal word if carried, clear timer.
        let answer = null;
        if (typeof data[1] === 'string') answer = data[1];
        else if (data[1] && typeof data[1] === 'object') answer = data[1].answer ?? data[1].palavra ?? null;
        if (answer) this.addSystemChat(`Time up — word was: ${answer}`);
        if (this.turn) {
            this.turn.startedAt = 0;
            this.turn.durationMs = 1;
        }
        this.renderTurn();
        const bar = document.querySelector('#bird-fv-timer .fill');
        if (bar) bar.style.width = '100%';
    }

    renderTurn() {
        const t = this.turn;
        const drawerEl = document.querySelector('#bird-fv-turn .drawer-line');
        const maskEl = document.querySelector('#bird-fv-turn .word-mask');
        const timerEl = document.querySelector('#bird-fv-turn .timer-line');
        if (!drawerEl || !maskEl || !timerEl) return;
        if (!t) { drawerEl.textContent = 'Waiting for turn…'; maskEl.textContent = '—'; timerEl.textContent = '⏱ —'; return; }
        const drawer = this.players.get(t.drawerId);
        drawerEl.textContent = `Drawing: ${drawer?.nick ?? '#'+t.drawerId}`;
        const displayMask = (t.wordMask || '').split('').map(ch =>
            ch === ' ' ? '  ' : (ch === '_' ? '_' : ch)).join(' ');
        maskEl.textContent = displayMask || '—';
        const remaining = Math.max(0, (t.startedAt + t.durationMs) - Date.now());
        timerEl.textContent = `⏱ ${Math.ceil(remaining/1000)}s`;
    }

    startTimerLoop() {
        cancelAnimationFrame(this.timerRaf);
        const tick = () => {
            const t = this.turn;
            const bar = document.querySelector('#bird-fv-timer .fill');
            if (t && bar) {
                const elapsed = Date.now() - t.startedAt;
                const pct = Math.min(100, (elapsed / t.durationMs) * 100);
                bar.style.width = pct + '%';
                const timerEl = document.querySelector('#bird-fv-turn .timer-line');
                if (timerEl) timerEl.textContent = `⏱ ${Math.max(0, Math.ceil((t.durationMs - elapsed)/1000))}s`;
            }
            this.timerRaf = requestAnimationFrame(tick);
        };
        this.timerRaf = requestAnimationFrame(tick);
    }

    playerRowHTML(p) {
        const foto = p.foto || `https://gartic.io/static/images/avatar/svg/${p.avatar}.svg`;
        const role = p.isDrawer ? '🎨' : (p.isBot ? '🤖' : '👤');
        const showKick = !p.isBot;
        const kickBtn = showKick
            ? `<button class="kick-btn" data-kick="${p.id}" title="Vote-kick (bots)">😀</button>`
            : '';
        return `
            <div class="pfv-player-row${p.isDrawer?' drawer':''}" data-id="${p.id}">
                <img class="avatar" src="${foto}" alt="">
                <div class="meta">
                    <div class="nick">${this.escapeHTML(p.nick)}</div>
                    <div class="pts">${p.pts ?? 0} pts</div>
                </div>
                ${kickBtn}
                <div class="role-icon">${role}</div>
            </div>`;
    }

    escapeHTML(s) {
        return String(s).replace(/[&<>"']/g, c =>
            ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    }

    renderPlayers() {
        const list = document.getElementById('pfv-player-list');
        if (!list) return;
        const drawerId = this.turn?.drawerId;
        const rows = [];
        for (const p of this.players.values()) {
            rows.push(this.playerRowHTML({ ...p, isDrawer: p.id === drawerId, isBot: this.botIds.has(p.id) }));
        }
        list.innerHTML = rows.join('');
        this.updateHeader();
    }

    connectControlWS() {
        let ws;
        try {
            ws = new WebSocket(`wss://${location.host}/bird/control?room=${this.roomCode}`);
        } catch {
            this.showBanner('info', 'Bot detection disabled — control WS open failed.');
            return;
        }
        this.controlSocket = ws;

        ws.onopen = () => {
            console.log('[bird-fv] control WS open');
            ws.send(JSON.stringify({ cmd: 'getBotList' }));
        };

        ws.onmessage = e => {
            let msg;
            try { msg = JSON.parse(e.data); } catch { return; }
            const ev = msg.event;
            if (ev === 'botList' || ev === 'botSync') {
                this.botIds = new Set((msg.bots || []).map(b => b.garticId).filter(Boolean));
                this.renderPlayers();
            } else if (ev === 'botJoined') {
                const gid = msg.bot?.garticId || msg.garticId;
                if (gid) { this.botIds.add(gid); this.renderPlayers(); }
            } else if (ev === 'botDisconnected') {
                // botDisconnected carries numericId, not garticId. We get the
                // garticId from the prior botList entry we no longer have; easiest
                // to re-fetch.
                ws.send(JSON.stringify({ cmd: 'getBotList' }));
            }
            // other events (roomUsers, turnSignal, etc.) are ignored — the
            // extension's own controller handles gameplay events.
        };

        ws.onclose = () => {
            this.controlSocket = null;
            console.log('[bird-fv] control WS closed');
        };
        ws.onerror = () => { /* close will follow */ };
    }

    detectExtension() {
        setTimeout(() => {
            const hasPanel = document.querySelector('[id^="icebot"]');
            if (!hasPanel) {
                this.showBanner('info',
                    'ICEbot extension not detected — fast viewer still works but kicks won\'t fire.');
            }
        }, 3000);
    }

    loadSnapshot() {
        this.snapshotFresh = false;
        try {
            const snapStr = localStorage.getItem(`bird-fv-snapshot:${this.roomCode}`);
            if (!snapStr) return;
            const snap = JSON.parse(snapStr);
            if (!snap) return;
            const ageMs = Date.now() - (snap.ts || 0);
            if (ageMs > 60_000) return;     // older snapshot is unhelpful
            for (const p of (snap.players || [])) {
                if (p && p.id != null) this.players.set(p.id, p);
            }
            if (Array.isArray(snap.chat)) this.chat.push(...snap.chat);
            this.snapshotFresh = true;
        } catch (e) { /* corrupt or missing — ignore */ }
    }

    async start() {
        document.documentElement.classList.add('bird-fv');
        this.injectStyles();
        this.loadSnapshot();
        await new Promise(resolve => onBodyReady(resolve));
        document.body.classList.add('bird-fv');
        this.renderSkeleton();
        this.renderPlayers();      // paint pre-seeded players immediately
        this.renderChat();         // paint pre-seeded chat immediately
        // Header info comes from the snapshot if present and from event 5 once
        // it arrives via the channel — no fetches, no awaits, nothing to block.
        this.updateHeader();
        this.connectViaChannel();  // live updates from the live page's WS
        if (!this.snapshotFresh) {
            this.showBanner('info',
                'Open Bird (gartic.io/live) and let it find this room — the viewer will populate automatically.');
        }
        this.connectControlWS();   // bot list + active session marker for Go server
        this.detectExtension();
    }
}

// Launcher placed AFTER the class declaration: `class` declarations are not
// hoisted and live in the temporal dead zone, so referencing FastViewer
// before this point throws a (silently swallowed) ReferenceError, which is
// why the body.bird-fv class never got applied and gartic's 404 error
// remained visible on /live#fv=<CODE>.
// standalone: drop the gartic.io/live hostname guard — we run from /bird; keep the #fv= hash check
true && location.hash.startsWith('#fv=') && (() => {
    // Gartic room codes are case-sensitive (e.g. "491KRo" ≠ "491KRO"), so
    // preserve the original casing from the URL — uppercasing here makes the
    // viewer-join sala mismatch the real room and Event 5 never arrives.
    const m = location.hash.match(/^#fv=([A-Za-z0-9]+)/);
    if (!m) return;
    W.__birdFV = new FastViewer(m[1]);
    W.__birdFV.start();
})();

// =========================================================================
// === Main Live Page ===
// =========================================================================
// standalone: drop the gartic.io/live hostname guard — we run from /bird; main UI fires when no #fv= hash is present
true && !location.hash.startsWith('#fv=') && (() => {
    // Load Plus Jakarta Sans font via <link> (GM_addStyle @import doesn't work in all userscript managers)
    const fontLink = Object.assign(document.createElement('link'), {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:wght@400;600;700;800&display=swap'
    });
    document.documentElement.appendChild(fontLink);

    GM_addStyle(`
        :root{
            --pl-bg-primary:#1d1a14;
            --pl-bg-secondary:#28241d;
            --pl-bg-tertiary:#363127;
            --pl-bg-elevated:#2c2820;
            --pl-bg-overlay:rgba(0,0,0,0.7);
            --pl-text-primary:#ebe6db;
            --pl-text-secondary:#a8a294;
            --pl-text-tertiary:#787268;
            --pl-text-inverse:#1a1a1a;
            --pl-accent-online:#22c55e;
            --pl-accent-new:#3b82f6;
            --pl-accent-leaving:#ef4444;
            --pl-accent-warning:#f59e0b;
            --pl-accent-tracked:#a855f7;
            --pl-interactive:#a855f7;
            --pl-interactive-hover:#c084fc;
            --pl-interactive-active:#9333ea;
            --pl-border:#3d3830;
            --pl-border-strong:#5a5448;
            --pl-shadow-sm:0 1px 2px rgba(0,0,0,0.3);
            --pl-shadow-md:0 2px 8px rgba(0,0,0,0.4);
            --pl-shadow-lg:0 4px 16px rgba(0,0,0,0.5);
            --pl-radius-sm:4px;
            --pl-radius-md:8px;
            --pl-radius-lg:12px;
            --pl-radius-full:9999px;
            --pl-font:'Plus Jakarta Sans','Segoe UI',system-ui,sans-serif;
            --pl-font-size-xs:0.75rem;
            --pl-font-size-sm:0.875rem;
            --pl-font-size-base:1rem;
            --pl-font-size-lg:1.25rem;
            --pl-font-size-xl:1.5rem;
            --pl-font-size-2xl:2rem;
            --pl-font-size-3xl:2.5rem;
            --pl-line-height-tight:1.15;
            --pl-line-height-normal:1.5;
            --pl-line-height-relaxed:1.65;
            --pl-space-1:4px;
            --pl-space-2:8px;
            --pl-space-3:12px;
            --pl-space-4:16px;
            --pl-space-5:20px;
            --pl-space-6:24px;
            --pl-space-8:32px;
            --pl-space-10:40px;
            --pl-transition-fast:0.15s ease;
            --pl-transition-normal:0.25s ease;
        }

        /* Global / Overlay */
        #BirdOverlay *{margin:0;padding:0;box-sizing:border-box}
        #BirdOverlay{position:fixed;top:0;left:0;width:100vw;height:100vh;z-index:999;overflow:hidden;display:flex;flex-direction:column;font-family:var(--pl-font);color:var(--pl-text-primary);background:var(--pl-bg-primary);font-kerning:normal;line-height:var(--pl-line-height-normal)}
        #BirdOverlay *:focus-visible{outline:2px solid var(--pl-interactive);outline-offset:2px}
        body{font-family:var(--pl-font);height:100vh;display:flex;flex-direction:column;background:var(--pl-bg-primary);color:var(--pl-text-primary);overflow:hidden;font-kerning:normal;line-height:var(--pl-line-height-normal)}

        /* Main Title */
        #BirdOverlay .Bird-mainTitle{font-size:var(--pl-font-size-2xl);font-weight:700;text-align:center;padding:var(--pl-space-5);color:var(--pl-text-primary);line-height:var(--pl-line-height-tight)}

        /* Start Screen */
        #BirdOverlay #Bird-startScreen{position:fixed;inset:0;display:flex;flex-direction:column;justify-content:center;align-items:center;gap:var(--pl-space-6);background:var(--pl-bg-primary);color:var(--pl-text-primary);z-index:9999;font-family:var(--pl-font);animation:plFadeIn 0.6s ease}
        #BirdOverlay #Bird-startScreen h1{font-size:var(--pl-font-size-3xl);font-weight:800;color:var(--pl-text-primary);letter-spacing:-0.5px;line-height:var(--pl-line-height-tight)}
        #BirdOverlay #Bird-version{font-size:var(--pl-font-size-xs);color:var(--pl-text-tertiary);margin-left:var(--pl-space-2)}
        #BirdOverlay .Bird-subtitle{font-size:var(--pl-font-size-lg);color:var(--pl-text-secondary)}
        #BirdOverlay .Bird-proxyRow{display:flex;gap:var(--pl-space-2);align-items:center;flex-wrap:wrap}
        #BirdOverlay #Bird-proxyBtn,
        #BirdOverlay #Bird-proxyAddBtn,
        #BirdOverlay #Bird-proxyResetBtn{padding:var(--pl-space-2) var(--pl-space-5);font-size:var(--pl-font-size-sm);font-weight:600;background:transparent;color:var(--pl-text-secondary);border:1px solid var(--pl-border);border-radius:var(--pl-radius-sm);cursor:pointer;transition:var(--pl-transition-fast);font-family:var(--pl-font)}
        #BirdOverlay #Bird-proxyBtn:hover,
        #BirdOverlay #Bird-proxyAddBtn:hover,
        #BirdOverlay #Bird-proxyResetBtn:hover{border-color:var(--pl-interactive);color:var(--pl-interactive)}
        #BirdOverlay #Bird-proxyBtn:disabled,
        #BirdOverlay #Bird-proxyAddBtn:disabled,
        #BirdOverlay #Bird-proxyResetBtn:disabled{opacity:0.5;cursor:not-allowed;pointer-events:none}
        #BirdOverlay #Bird-proxyAddBtn{padding:var(--pl-space-2) var(--pl-space-3)}
        #BirdOverlay #Bird-proxyResetBtn{padding:var(--pl-space-2) var(--pl-space-3)}

        /* Animations */
        @keyframes plFadeIn{from{opacity:0}to{opacity:1}}
        @keyframes plCardSlideIn{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:translateY(0)}}
        @keyframes plCardFadeOut{from{opacity:1;transform:scale(1)}to{opacity:0;transform:scale(0.95)}}
        @keyframes plRoomFadeOut{from{opacity:1;transform:scaleY(1)}to{opacity:0;transform:scaleY(0)}}
        @keyframes plSpin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
        @keyframes plPulse{0%,100%{opacity:0.5}50%{opacity:1}}
        @keyframes plMsgAppear{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:translateY(0)}}
        @keyframes plMergePulse{0%,100%{opacity:0.7}50%{opacity:1}}
        @keyframes plTrackGlow{0%{box-shadow:0 0 0 0 rgba(147,51,234,0.7)}30%{box-shadow:0 0 18px 6px rgba(147,51,234,0.5)}60%{box-shadow:0 0 8px 2px rgba(147,51,234,0.3)}100%{box-shadow:0 0 0 0 rgba(147,51,234,0)}}
        @keyframes plBellPulse{0%{box-shadow:0 0 0 0 rgba(245,158,11,0.7)}40%{box-shadow:0 0 16px 5px rgba(245,158,11,0.45)}100%{box-shadow:0 0 0 0 rgba(245,158,11,0)}}
        #BirdOverlay .Bird-room-glow{animation:plTrackGlow 1.2s ease-out;border-color:var(--pl-accent-tracked)}
        #BirdOverlay .Bird-trackedUser.bell-flash{animation:plBellPulse 1.4s ease-out}

        /* Main Layout */
        #BirdOverlay main.Bird-main{display:none;flex:1;grid-template-columns:2fr 1fr;gap:var(--pl-space-5);padding:var(--pl-space-4);overflow:hidden;opacity:0;animation:plFadeIn 0.6s forwards;min-height:0}
        #BirdOverlay main.Bird-main.Bird-main--active{display:grid}
        #BirdOverlay .Bird-left{display:flex;flex-direction:column;gap:var(--pl-space-4);overflow:hidden;min-height:0}

        /* Filters Bar */
        #BirdOverlay .Bird-filters{display:flex;gap:var(--pl-space-3);align-items:center;background:var(--pl-bg-secondary);padding:var(--pl-space-4);border-radius:var(--pl-radius-md);box-shadow:var(--pl-shadow-sm);flex-wrap:wrap;justify-content:space-between;opacity:0;animation:plFadeIn 0.6s 0.2s forwards}
        #BirdOverlay .Bird-filters input{flex:2;padding:var(--pl-space-2) var(--pl-space-3);border-radius:var(--pl-radius-sm);border:1px solid var(--pl-border);background:var(--pl-bg-tertiary);color:var(--pl-text-primary);font-family:var(--pl-font);font-size:var(--pl-font-size-base)}
        #BirdOverlay .Bird-filters input::placeholder{color:var(--pl-text-tertiary)}
        #BirdOverlay .Bird-filters button{padding:var(--pl-space-2) var(--pl-space-4);border-radius:var(--pl-radius-sm);border:none;cursor:pointer;background:var(--pl-interactive);color:var(--pl-text-inverse);font-weight:600;transition:var(--pl-transition-fast)}
        #BirdOverlay .Bird-filters button:hover{background:var(--pl-interactive-hover);transform:translateY(-1px)}
        #BirdOverlay .Bird-filters button:disabled{background:var(--pl-bg-tertiary);color:var(--pl-text-tertiary);cursor:not-allowed;transform:none}

        /* Mode option buttons */
        #BirdOverlay .Bird-modeOptions{display:flex;gap:var(--pl-space-4);flex-wrap:wrap;justify-content:center}
        #BirdOverlay .Bird-modeBtn{display:flex;flex-direction:column;align-items:center;gap:var(--pl-space-1);padding:var(--pl-space-4) var(--pl-space-6);background:var(--pl-bg-secondary);border:2px solid var(--pl-border);border-radius:var(--pl-radius-md);color:var(--pl-text-primary);cursor:pointer;transition:all 0.2s ease;font-family:var(--pl-font);min-width:170px}
        #BirdOverlay .Bird-modeBtn:hover:not(:disabled){border-color:var(--pl-interactive);color:var(--pl-interactive);transform:translateY(-3px);box-shadow:var(--pl-shadow-md)}
        #BirdOverlay .Bird-modeBtn:disabled{opacity:0.35;cursor:not-allowed;transform:none;box-shadow:none}
        #BirdOverlay .Bird-modeName{font-weight:700;font-size:var(--pl-font-size-base)}
        #BirdOverlay .Bird-modeDesc{font-size:var(--pl-font-size-xs);color:var(--pl-text-secondary);font-weight:400}

        /* Stats */
        #BirdOverlay #Bird-stats{display:flex;gap:var(--pl-space-2);flex-direction:column;color:var(--pl-text-secondary);font-size:var(--pl-font-size-xs);font-weight:600;text-transform:uppercase}

        /* Tracked Users Bar */
        #BirdOverlay .Bird-trackedBar{display:flex;gap:var(--pl-space-3);align-items:center;padding:var(--pl-space-3) var(--pl-space-4);background:var(--pl-bg-secondary);border-radius:var(--pl-radius-md);border-top:2px solid var(--pl-accent-tracked);flex-wrap:wrap;min-height:48px}
        #BirdOverlay .Bird-trackedLabel{font-size:var(--pl-font-size-xs);color:var(--pl-accent-tracked);font-weight:700;letter-spacing:0.5px;text-transform:uppercase;margin-right:var(--pl-space-1);white-space:nowrap}
        #BirdOverlay .Bird-trackedUser{position:relative;display:flex;flex-direction:column;align-items:center;gap:2px;cursor:pointer;padding:var(--pl-space-1) var(--pl-space-2);border-radius:var(--pl-radius-md);background:var(--pl-bg-elevated);border:1px solid var(--pl-border);transition:all 0.25s ease;min-width:60px}
        #BirdOverlay .Bird-trackedUser:hover{transform:translateY(-2px);box-shadow:var(--pl-shadow-md)}
        #BirdOverlay .Bird-trackedUser.online{border-left:3px solid var(--pl-accent-online)}
        #BirdOverlay .Bird-trackedUser.offline{opacity:0.45;filter:grayscale(0.5)}
        #BirdOverlay .Bird-trackedUser.merge-target{border:2px dashed var(--pl-accent-warning);animation:plMergePulse 1.5s ease infinite}
        #BirdOverlay .Bird-trackedUser .Bird-trackedAvatar{width:34px;height:34px;border-radius:var(--pl-radius-full);background:var(--pl-bg-tertiary);object-fit:cover}
        #BirdOverlay .Bird-trackedUser .Bird-trackedNoAvatar{width:34px;height:34px;border-radius:var(--pl-radius-full);background:var(--pl-bg-tertiary);display:flex;align-items:center;justify-content:center;font-size:var(--pl-font-size-base);color:var(--pl-text-tertiary)}
        #BirdOverlay .Bird-trackedUser .Bird-trackedName{font-size:var(--pl-font-size-xs);color:var(--pl-text-primary);max-width:65px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;text-align:center}
        #BirdOverlay .Bird-trackedUser .Bird-trackedRoom{font-size:var(--pl-font-size-xs);color:var(--pl-accent-online);max-width:65px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;text-align:center}
        #BirdOverlay .Bird-trackedBtns{position:absolute;top:-6px;right:-6px;display:flex;gap:2px;opacity:0;transition:opacity var(--pl-transition-fast)}
        #BirdOverlay .Bird-trackedUser:hover .Bird-trackedBtns{opacity:1}
        #BirdOverlay .Bird-trackedBtn{width:28px;height:28px;border-radius:var(--pl-radius-full);color:var(--pl-text-inverse);font-size:var(--pl-font-size-xs);line-height:28px;text-align:center;cursor:pointer;font-weight:700;border:none;padding:0;background:none;font-family:var(--pl-font)}
        #BirdOverlay .Bird-trackedBtn.remove{background:var(--pl-accent-leaving);color:var(--pl-text-inverse)}
        #BirdOverlay .Bird-trackedBtn.edit{background:var(--pl-interactive);color:var(--pl-text-inverse)}
        #BirdOverlay .Bird-trackedBtn.bell{background:var(--pl-bg-tertiary);color:var(--pl-text-secondary);font-size:13px;line-height:28px}
        #BirdOverlay .Bird-trackedBtn.bell.on{background:var(--pl-accent-warning);color:var(--pl-text-inverse)}
        #BirdOverlay .Bird-masterMute{font-size:var(--pl-font-size-sm);color:var(--pl-text-secondary);cursor:pointer;padding:2px 6px;border-radius:var(--pl-radius-md);user-select:none;line-height:1;transition:color var(--pl-transition-fast),background var(--pl-transition-fast)}
        #BirdOverlay .Bird-masterMute:hover{color:var(--pl-text-primary);background:var(--pl-bg-elevated)}
        #BirdOverlay .Bird-masterMute.muted{color:var(--pl-accent-leaving)}
        #BirdOverlay .Bird-masterAD{font-size:var(--pl-font-size-xs);font-weight:700;letter-spacing:0.5px;text-transform:uppercase;cursor:pointer;padding:4px 10px;border-radius:var(--pl-radius-md);user-select:none;line-height:1;border:1px solid transparent;transition:color var(--pl-transition-fast),background var(--pl-transition-fast),border-color var(--pl-transition-fast);background:rgba(56,176,108,0.12);color:#3CC57F;border-color:rgba(56,176,108,0.4)}
        #BirdOverlay .Bird-masterAD:hover{background:rgba(56,176,108,0.22)}
        #BirdOverlay .Bird-masterAD.off{background:rgba(231,76,60,0.14);color:#E74C3C;border-color:rgba(231,76,60,0.4)}
        #BirdOverlay .Bird-masterAD.off:hover{background:rgba(231,76,60,0.24)}
        #BirdOverlay .Bird-immunePill{font-size:var(--pl-font-size-xs);font-weight:700;letter-spacing:0.5px;text-transform:uppercase;cursor:pointer;padding:4px 10px;border-radius:var(--pl-radius-md);user-select:none;line-height:1;border:1px solid rgba(149,165,166,0.4);background:rgba(149,165,166,0.12);color:#95A5A6;transition:color var(--pl-transition-fast),background var(--pl-transition-fast)}
        #BirdOverlay .Bird-immunePill:hover{background:rgba(149,165,166,0.22)}
        #BirdOverlay .Bird-immunePill.has{color:#F1C40F;background:rgba(241,196,15,0.12);border-color:rgba(241,196,15,0.4)}
        #BirdOverlay .Bird-immunePill.has:hover{background:rgba(241,196,15,0.22)}
        #BirdOverlay .Bird-immuneShield{cursor:pointer;padding:2px 6px;margin-left:4px;font-size:13px;line-height:1;border-radius:var(--pl-radius-md);user-select:none;color:var(--pl-text-secondary);transition:color var(--pl-transition-fast),background var(--pl-transition-fast)}
        #BirdOverlay .Bird-immuneShield:hover{color:var(--pl-text-primary);background:var(--pl-bg-elevated)}
        #BirdOverlay .Bird-immuneShield.on{color:#F1C40F}
        #BirdOverlay .Bird-room-group.immune{outline:1px dashed rgba(241,196,15,0.4);outline-offset:-2px}
        #BirdOverlay .Bird-immuneOverlay{position:fixed;inset:0;background:rgba(0,0,0,0.6);display:flex;align-items:center;justify-content:center;z-index:10001}
        #BirdOverlay .Bird-immunePanel{background:var(--pl-bg-secondary);border-radius:var(--pl-radius-lg);padding:var(--pl-space-5);max-width:420px;width:90%;max-height:80vh;overflow-y:auto;display:flex;flex-direction:column;gap:var(--pl-space-3)}
        #BirdOverlay .Bird-immunePanel h3{margin:0;font-size:var(--pl-font-size-md);color:var(--pl-text-primary)}
        #BirdOverlay .Bird-immunePanel .row{display:flex;gap:6px;align-items:center;background:var(--pl-bg-tertiary);padding:6px 10px;border-radius:var(--pl-radius-md);font-family:var(--pl-font-mono,monospace)}
        #BirdOverlay .Bird-immunePanel .row .code{flex:1;color:#F1C40F}
        #BirdOverlay .Bird-immunePanel .row button{background:transparent;color:var(--pl-accent-leaving);border:none;cursor:pointer;font-size:16px;line-height:1;padding:2px 6px}
        #BirdOverlay .Bird-immunePanel .add{display:flex;gap:6px;margin-top:var(--pl-space-2)}
        #BirdOverlay .Bird-immunePanel .add input{flex:1;background:var(--pl-bg-tertiary);border:1px solid var(--pl-border);color:var(--pl-text-primary);padding:6px 10px;border-radius:var(--pl-radius-md);font-family:var(--pl-font-mono,monospace)}
        #BirdOverlay .Bird-immunePanel .add button{padding:6px 14px}
        #BirdOverlay .Bird-immunePanel .closeRow{display:flex;justify-content:flex-end;margin-top:var(--pl-space-2)}
        #BirdOverlay .Bird-immunePanel .empty{color:var(--pl-text-secondary);font-style:italic;font-size:var(--pl-font-size-xs);text-align:center;padding:8px}

        /* Track button on player cards */
        #BirdOverlay .Bird-trackBtn{position:absolute;top:var(--pl-space-1);right:var(--pl-space-1);font-size:var(--pl-font-size-sm);cursor:pointer;opacity:0;transition:opacity var(--pl-transition-fast);line-height:1;z-index:1;background:none;border:none;padding:var(--pl-space-2);min-width:44px;min-height:44px;display:flex;align-items:center;justify-content:center}
        #BirdOverlay .Bird-card:hover .Bird-trackBtn{opacity:0.7}
        #BirdOverlay .Bird-trackBtn:hover{opacity:1;transform:scale(1.2)}
        #BirdOverlay .Bird-trackBtn.tracked{opacity:0.9;color:var(--pl-accent-warning)}
        #BirdOverlay .Bird-card{position:relative}

        /* Edit Panel (Modal) */
        #BirdOverlay .Bird-editOverlay{position:fixed;inset:0;background:var(--pl-bg-overlay);z-index:10000;display:flex;align-items:center;justify-content:center;animation:plFadeIn 0.2s ease}
        #BirdOverlay .Bird-editPanel{background:var(--pl-bg-elevated);border-radius:var(--pl-radius-lg);padding:var(--pl-space-6);width:360px;max-width:90vw;max-height:80vh;overflow-y:auto;box-shadow:var(--pl-shadow-lg);border:1px solid var(--pl-border);color:var(--pl-text-primary);font-family:var(--pl-font)}
        #BirdOverlay .Bird-editPanel h3{margin:0 0 var(--pl-space-4);font-size:var(--pl-font-size-lg);color:var(--pl-text-primary);display:flex;align-items:center;gap:var(--pl-space-2)}
        #BirdOverlay .Bird-editPanel h3 img{width:36px;height:36px;border-radius:var(--pl-radius-full);border:2px solid var(--pl-accent-tracked);object-fit:cover}
        #BirdOverlay .Bird-editPanel h3 .Bird-editNoAvatar{width:36px;height:36px;border-radius:var(--pl-radius-full);background:var(--pl-bg-tertiary);display:inline-flex;align-items:center;justify-content:center;font-size:var(--pl-font-size-base);color:var(--pl-text-tertiary);border:2px solid var(--pl-border)}
        #BirdOverlay .Bird-editField{margin-bottom:var(--pl-space-3)}
        #BirdOverlay .Bird-editField label{display:block;font-size:var(--pl-font-size-xs);color:var(--pl-text-secondary);text-transform:uppercase;letter-spacing:0.5px;margin-bottom:var(--pl-space-1);font-weight:600}
        #BirdOverlay .Bird-editField input{width:100%;padding:var(--pl-space-2) var(--pl-space-3);border-radius:var(--pl-radius-sm);border:1px solid var(--pl-border);background:var(--pl-bg-tertiary);color:var(--pl-text-primary);font-size:var(--pl-font-size-base);font-family:var(--pl-font);outline:none;transition:border var(--pl-transition-fast)}
        #BirdOverlay .Bird-editField input:focus{border-color:var(--pl-interactive)}
        #BirdOverlay .Bird-editIds{display:flex;flex-direction:column;gap:6px;margin-bottom:var(--pl-space-3)}
        #BirdOverlay .Bird-editIds label{font-size:var(--pl-font-size-xs);color:var(--pl-text-secondary);text-transform:uppercase;letter-spacing:0.5px;margin-bottom:2px;font-weight:600}
        #BirdOverlay .Bird-editId{display:flex;align-items:center;gap:var(--pl-space-2);padding:6px var(--pl-space-3);background:var(--pl-bg-tertiary);border-radius:var(--pl-radius-sm);border:1px solid var(--pl-border);font-size:var(--pl-font-size-sm)}
        #BirdOverlay .Bird-editId .type{color:var(--pl-interactive);font-weight:700;text-transform:uppercase;font-size:var(--pl-font-size-xs);min-width:38px}
        #BirdOverlay .Bird-editId .value{flex:1;color:var(--pl-text-secondary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
        #BirdOverlay .Bird-editId .delId{color:var(--pl-accent-leaving);cursor:pointer;font-weight:700;font-size:var(--pl-font-size-lg);flex-shrink:0;background:none;border:none;padding:var(--pl-space-1);font-family:var(--pl-font)}
        #BirdOverlay .Bird-editId .delId:hover{opacity:0.7}
        #BirdOverlay .Bird-editActions{display:flex;gap:var(--pl-space-2);flex-wrap:wrap;margin-top:var(--pl-space-4)}
        #BirdOverlay .Bird-editActions button{flex:1;padding:var(--pl-space-2) 0;border:none;border-radius:var(--pl-radius-sm);font-weight:700;font-size:var(--pl-font-size-sm);cursor:pointer;transition:all var(--pl-transition-fast);font-family:var(--pl-font)}
        #BirdOverlay .Bird-editActions button:hover{transform:translateY(-1px)}
        #BirdOverlay .Bird-editActions .save{background:var(--pl-interactive);color:var(--pl-text-inverse)}
        #BirdOverlay .Bird-editActions .merge{background:var(--pl-accent-warning);color:var(--pl-text-inverse)}
        #BirdOverlay .Bird-editActions .delete{background:var(--pl-accent-leaving);color:var(--pl-text-inverse)}
        #BirdOverlay .Bird-editActions .cancel{background:var(--pl-bg-tertiary);color:var(--pl-text-secondary)}
        #BirdOverlay .Bird-mergeHint{text-align:center;padding:6px;background:var(--pl-accent-warning);color:var(--pl-text-inverse);border-radius:var(--pl-radius-sm);font-size:var(--pl-font-size-sm);font-weight:700;margin-bottom:6px;animation:plMergePulse 1.5s ease infinite}

        /* Players / Room Groups */
        #BirdOverlay .Bird-players{flex:1;overflow-y:auto;display:flex;flex-direction:column;gap:var(--pl-space-4);padding-right:var(--pl-space-2);opacity:0;animation:plFadeIn 0.6s 0.3s forwards;scrollbar-color:var(--pl-border-strong) var(--pl-bg-secondary);scrollbar-width:thin;contain:strict}
        #BirdOverlay .Bird-room-group{background:var(--pl-bg-elevated);border-radius:var(--pl-radius-md);padding:var(--pl-space-4);border:1px solid var(--pl-border);transition:opacity 0.4s ease,transform 0.4s ease,border-color 0.6s ease;box-shadow:var(--pl-shadow-sm);content-visibility:auto;contain-intrinsic-size:auto 200px}
        #BirdOverlay .Bird-room-header{color:var(--pl-text-primary);font-weight:700;font-size:var(--pl-font-size-lg);line-height:var(--pl-line-height-tight);margin-bottom:var(--pl-space-3);padding-bottom:var(--pl-space-2);border-bottom:1px solid var(--pl-border);display:flex;align-items:center;gap:var(--pl-space-2)}
        #BirdOverlay .Bird-headerBtn{cursor:pointer;background:none;border:none;padding:var(--pl-space-1) var(--pl-space-2);border-radius:var(--pl-radius-sm);font-size:var(--pl-font-size-base);transition:var(--pl-transition-fast);color:var(--pl-text-secondary);font-family:var(--pl-font)}
        #BirdOverlay .Bird-headerBtn:hover{background:var(--pl-bg-tertiary);color:var(--pl-interactive)}
        #BirdOverlay .Bird-room-list{display:flex;flex-direction:row;flex-wrap:wrap;gap:var(--pl-space-3)}

        /* Player Cards */
        #BirdOverlay .Bird-card{background:var(--pl-bg-secondary);border-radius:var(--pl-radius-md);padding:var(--pl-space-4);text-align:center;cursor:pointer;transition:all 0.25s ease;box-shadow:var(--pl-shadow-sm);display:flex;flex-direction:column;align-items:center;min-width:100px;flex:0 1 120px;position:relative}
        #BirdOverlay .Bird-card:hover{transform:translateY(-2px);box-shadow:var(--pl-shadow-md)}
        #BirdOverlay .Bird-avatar{width:48px;height:48px;border-radius:var(--pl-radius-full);margin-bottom:var(--pl-space-2);background:var(--pl-bg-tertiary);display:flex;align-items:center;justify-content:center;font-size:var(--pl-font-size-lg);color:var(--pl-text-tertiary)}
        #BirdOverlay .Bird-name{font-size:var(--pl-font-size-sm);font-weight:600;margin-top:var(--pl-space-1);margin-bottom:2px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:110px;color:var(--pl-text-primary)}

        /* Diff refresh animations */
        #BirdOverlay .Bird-room-new{border-left:3px solid var(--pl-accent-new)}
        #BirdOverlay .Bird-newBadge{display:inline-block;font-size:var(--pl-font-size-xs);font-weight:700;color:var(--pl-text-inverse);background:var(--pl-accent-new);padding:2px var(--pl-space-2);border-radius:var(--pl-radius-sm);margin-left:auto;letter-spacing:0.5px;transition:opacity 1.5s ease}
        #BirdOverlay .Bird-newBadge.fading{opacity:0}
        #BirdOverlay .Bird-card-entering{animation:plCardSlideIn 0.3s ease-out both}
        #BirdOverlay .Bird-card-leaving{animation:plCardFadeOut 0.25s ease-in both;pointer-events:none}
        #BirdOverlay .Bird-room-leaving{animation:plRoomFadeOut 0.4s ease-in both;transform-origin:top;pointer-events:none}
        #BirdOverlay .Bird-card[data-stale="1"]{opacity:0.4;transition:opacity 0.3s ease}
        #BirdOverlay .Bird-room-group[data-stale="1"]{opacity:0.5;transition:opacity 0.3s ease}
        #BirdOverlay .Bird-card[data-stale="0"],#BirdOverlay .Bird-room-group[data-stale="0"]{opacity:1;transition:opacity 0.3s ease}

        /* Refresh spinner */
        #BirdOverlay .Bird-refreshing{position:relative;pointer-events:none;color:transparent}
        #BirdOverlay .Bird-refreshing::after{content:"";position:absolute;inset:0;margin:auto;width:16px;height:16px;border:2px solid var(--pl-text-primary);border-top-color:transparent;border-radius:var(--pl-radius-full);animation:plSpin 0.6s linear infinite}

        /* Chat Panel */
        #BirdOverlay .Bird-chat{background:var(--pl-bg-elevated);border-radius:var(--pl-radius-md);display:flex;flex-direction:column;overflow:hidden;box-shadow:var(--pl-shadow-sm);opacity:0;animation:plFadeIn 0.6s 0.4s forwards}
        #BirdOverlay .Bird-chatHeader{background:var(--pl-bg-secondary);padding:var(--pl-space-4);color:var(--pl-text-primary);font-weight:600;font-size:var(--pl-font-size-lg);display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid var(--pl-border)}
        #BirdOverlay .Bird-chatFilters{display:flex;align-items:center;gap:var(--pl-space-2);font-size:var(--pl-font-size-sm);color:var(--pl-text-primary);flex-wrap:wrap}
        #BirdOverlay .Bird-chatFilters label{display:flex;align-items:center;gap:var(--pl-space-1)}
        #BirdOverlay .Bird-chatFilters select{padding:2px 6px;border-radius:var(--pl-radius-sm);border:1px solid var(--pl-border);background:var(--pl-bg-tertiary);color:var(--pl-text-primary);font-family:var(--pl-font);font-size:var(--pl-font-size-sm);cursor:pointer}
        #BirdOverlay .Bird-chatButtons{display:flex;gap:var(--pl-space-1)}
        #BirdOverlay .Bird-chatButtons button{padding:var(--pl-space-1) var(--pl-space-3);border:2px solid var(--pl-border-strong);border-radius:var(--pl-radius-sm);background:transparent;color:var(--pl-text-primary);cursor:pointer;transition:var(--pl-transition-fast);font-family:var(--pl-font);font-size:var(--pl-font-size-sm)}
        #BirdOverlay .Bird-chatButtons button:hover{border-color:var(--pl-interactive);color:var(--pl-interactive)}
        #BirdOverlay .Bird-searchMsg{padding:var(--pl-space-2);background:var(--pl-bg-secondary)}
        #BirdOverlay .Bird-searchMsg input{width:100%;padding:var(--pl-space-2) var(--pl-space-3);border-radius:var(--pl-radius-sm);border:1px solid var(--pl-border);background:var(--pl-bg-tertiary);color:var(--pl-text-primary);font-family:var(--pl-font);font-size:var(--pl-font-size-base)}
        #BirdOverlay .Bird-searchMsg input::placeholder{color:var(--pl-text-tertiary)}
        #BirdOverlay .Bird-msgs{flex:1;overflow-y:auto;position:relative;background:var(--pl-bg-primary);scrollbar-color:var(--pl-border-strong) var(--pl-bg-secondary);scrollbar-width:thin;contain:strict}
        #BirdOverlay .pl-scroll-content{position:relative;width:100%}
        #BirdOverlay .pl-render-zone{position:absolute;left:0;right:0;padding:0 var(--pl-space-4)}
        #BirdOverlay .Bird-msg{height:29px;line-height:29px;font-size:var(--pl-font-size-base);color:var(--pl-text-primary);display:flex;flex-direction:row;align-items:center;text-align:left;overflow:hidden}
        #BirdOverlay .Bird-msg .pl-text{flex:1;min-width:0;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
        #BirdOverlay .Bird-msg .pl-badge{font-size:var(--pl-font-size-xs);font-weight:700;padding:1px var(--pl-space-1);border-radius:var(--pl-radius-sm);color:var(--pl-text-inverse);margin-right:var(--pl-space-1);flex-shrink:0}
        #BirdOverlay .Bird-msg .pl-author{font-weight:700;font-size:var(--pl-font-size-sm);color:oklch(55% 0.15 250);white-space:nowrap;flex-shrink:0}
        #BirdOverlay .Bird-filterRoomBtn.active{background:var(--pl-interactive);color:var(--pl-text-inverse)}

        /* Webkit scrollbars */
        #BirdOverlay .Bird-players::-webkit-scrollbar,#BirdOverlay .Bird-msgs::-webkit-scrollbar{width:8px}
        #BirdOverlay .Bird-players::-webkit-scrollbar-track,#BirdOverlay .Bird-msgs::-webkit-scrollbar-track{background:var(--pl-bg-secondary);border-radius:var(--pl-radius-full)}
        #BirdOverlay .Bird-players::-webkit-scrollbar-thumb,#BirdOverlay .Bird-msgs::-webkit-scrollbar-thumb{background:var(--pl-border-strong);border-radius:var(--pl-radius-full)}
        #BirdOverlay .Bird-players::-webkit-scrollbar-thumb:hover,#BirdOverlay .Bird-msgs::-webkit-scrollbar-thumb:hover{background:var(--pl-text-tertiary)}

        /* Bottom tab bar — desktop hides it, mobile shows */
        #BirdOverlay .Bird-tabBar{display:none}

        /* Tablet portrait: stack panels vertically, keep both visible */
        @media(max-width:900px) and (min-width:721px){
            #BirdOverlay main.Bird-main.Bird-main--active{grid-template-columns:1fr;grid-template-rows:3fr 2fr}
        }

        /* Mobile (≤720px): single-panel view, bottom tabs switch between Players & Chat */
        @media(max-width:720px){
            #BirdOverlay{height:100dvh;padding-top:env(safe-area-inset-top);padding-left:env(safe-area-inset-left);padding-right:env(safe-area-inset-right)}
            #BirdOverlay .Bird-mainTitle{display:none}

            /* Main app becomes a flex column hosting one panel + tab bar */
            #BirdOverlay main.Bird-main.Bird-main--active{
                display:flex;flex-direction:column;
                padding:var(--pl-space-2);gap:var(--pl-space-2);
            }
            #BirdOverlay .Bird-left,#BirdOverlay .Bird-chat{flex:1;min-height:0}
            #BirdOverlay[data-mv="players"] .Bird-chat{display:none}
            #BirdOverlay[data-mv="chat"] .Bird-left{display:none}

            /* Filters: stack inputs full-width, place buttons on a single row */
            #BirdOverlay .Bird-filters{
                flex-direction:column;align-items:stretch;
                gap:var(--pl-space-2);padding:var(--pl-space-3);
            }
            #BirdOverlay .Bird-filters input{width:100%;min-height:44px;font-size:16px}
            #BirdOverlay .Bird-filters > button{min-height:44px;flex:1 1 auto;order:3}
            #BirdOverlay #Bird-refreshBtn{order:3}
            #BirdOverlay #Bird-runOnServerBtn{order:4;font-size:var(--pl-font-size-sm)}
            #BirdOverlay #Bird-stats{order:5;flex-direction:row;gap:var(--pl-space-3);align-self:stretch;justify-content:space-between;font-size:11px;color:var(--pl-text-tertiary);padding-top:var(--pl-space-1);border-top:1px solid var(--pl-border)}

            /* Tracked bar: horizontal scroll instead of wrap */
            #BirdOverlay .Bird-trackedBar{
                flex-wrap:nowrap;overflow-x:auto;
                -webkit-overflow-scrolling:touch;
                scrollbar-width:none;padding:var(--pl-space-2) var(--pl-space-3);
            }
            #BirdOverlay .Bird-trackedBar::-webkit-scrollbar{display:none}
            #BirdOverlay .Bird-trackedUser{flex-shrink:0;min-width:64px}
            #BirdOverlay .Bird-trackedUser .Bird-trackedBtns{opacity:1}

            /* Player cards: 3-up grid with tight gaps */
            #BirdOverlay .Bird-room-group{padding:var(--pl-space-3)}
            #BirdOverlay .Bird-room-list{gap:var(--pl-space-2)}
            #BirdOverlay .Bird-card{flex:1 1 calc(33.333% - var(--pl-space-2));min-width:0;max-width:none;padding:var(--pl-space-3)}
            #BirdOverlay .Bird-avatar{width:40px;height:40px}
            #BirdOverlay .Bird-name{max-width:100%;font-size:var(--pl-font-size-xs)}
            #BirdOverlay .Bird-trackBtn{opacity:0.85}

            /* Chat header: title on its own row, icon-only action buttons */
            #BirdOverlay .Bird-chatHeader{flex-wrap:wrap;gap:var(--pl-space-2);padding:var(--pl-space-3);align-items:center}
            #BirdOverlay #Bird-chatTitle{flex:1 1 100%;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;order:1;font-size:var(--pl-font-size-base)}
            #BirdOverlay .Bird-chatFilters{order:2;font-size:var(--pl-font-size-xs)}
            #BirdOverlay .Bird-chatFilters input[type="checkbox"]{width:20px;height:20px}
            #BirdOverlay .Bird-chatButtons{order:3;margin-left:auto;gap:var(--pl-space-1)}
            #BirdOverlay .Bird-chatButtons button{min-width:44px;min-height:44px;padding:0;font-size:var(--pl-font-size-xs);font-weight:700}
            #BirdOverlay .Bird-searchMsg input{font-size:16px;min-height:44px}

            /* Bottom tab bar */
            #BirdOverlay .Bird-tabBar{
                display:flex;gap:var(--pl-space-1);
                background:var(--pl-bg-secondary);
                border-top:1px solid var(--pl-border);
                padding:var(--pl-space-1) var(--pl-space-2);
                padding-bottom:max(var(--pl-space-1),env(safe-area-inset-bottom));
                margin:0 calc(-1 * var(--pl-space-2)) calc(-1 * var(--pl-space-2));
            }
            #BirdOverlay .Bird-tab{
                flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;
                gap:2px;background:transparent;border:none;cursor:pointer;
                color:var(--pl-text-secondary);font-family:var(--pl-font);
                font-size:var(--pl-font-size-xs);font-weight:600;
                padding:var(--pl-space-2);min-height:52px;
                border-radius:var(--pl-radius-sm);
                transition:var(--pl-transition-fast);
            }
            #BirdOverlay .Bird-tab[aria-selected="true"]{color:var(--pl-interactive);background:var(--pl-bg-elevated)}
            #BirdOverlay .Bird-tab:active{transform:scale(0.97)}
            #BirdOverlay .Bird-tabIcon{font-size:var(--pl-font-size-lg);line-height:1}
            #BirdOverlay .Bird-tabLabel{letter-spacing:0.4px;text-transform:uppercase}

            /* Edit modal → bottom sheet */
            #BirdOverlay .Bird-editOverlay{align-items:flex-end}
            #BirdOverlay .Bird-editPanel{
                width:100%;max-width:100%;max-height:90vh;
                border-radius:var(--pl-radius-lg) var(--pl-radius-lg) 0 0;
                padding:var(--pl-space-5);
                padding-bottom:max(var(--pl-space-5),env(safe-area-inset-bottom));
                animation:plSheetUp 0.25s ease-out;
            }
            #BirdOverlay .Bird-editField input{min-height:44px;font-size:16px}
            #BirdOverlay .Bird-editActions button{min-height:44px;font-size:var(--pl-font-size-base)}

            /* Start screen: scrollable, tighter, mode buttons stacked */
            #BirdOverlay #Bird-startScreen{
                gap:var(--pl-space-4);padding:var(--pl-space-5);
                justify-content:flex-start;overflow-y:auto;
                padding-top:max(var(--pl-space-8),env(safe-area-inset-top));
                padding-bottom:max(var(--pl-space-5),env(safe-area-inset-bottom));
            }
            #BirdOverlay #Bird-startScreen h1{font-size:var(--pl-font-size-2xl)}
            #BirdOverlay .Bird-modeOptions{flex-direction:column;align-items:stretch;width:100%;max-width:320px}
            #BirdOverlay .Bird-modeBtn{min-width:0;width:100%;min-height:64px;padding:var(--pl-space-3) var(--pl-space-4)}
            #BirdOverlay .Bird-proxyRow{width:100%;max-width:320px;justify-content:center;flex-wrap:wrap}
            #BirdOverlay .Bird-proxyRow button{min-height:44px;flex:1 1 100px}
            #BirdOverlay .Bird-credit{position:static;text-align:center;margin-top:auto;padding-top:var(--pl-space-4);left:auto;bottom:auto}
        }
        @keyframes plSheetUp{from{transform:translateY(100%)}to{transform:translateY(0)}}

        /* Credits */
        #BirdOverlay .Bird-credit{position:absolute;bottom:var(--pl-space-3);left:var(--pl-space-3);font-size:var(--pl-font-size-sm);color:var(--pl-text-secondary);font-family:var(--pl-font);opacity:0.8}
        #BirdOverlay .Bird-credit a{color:var(--pl-interactive);text-decoration:none;font-weight:600}
        #BirdOverlay .Bird-credit a:hover{text-decoration:underline}
        #BirdOverlay .Bird-credit .heart{color:var(--pl-accent-leaving);margin:0 2px}

        /* Loading */
        #BirdOverlay .Bird-loading{text-align:center;padding:var(--pl-space-5);color:var(--pl-text-secondary);font-size:var(--pl-font-size-lg);animation:plPulse 1.5s infinite}
        #BirdOverlay .Bird-empty-state{text-align:center;padding:var(--pl-space-10) var(--pl-space-6);color:var(--pl-text-tertiary);font-size:var(--pl-font-size-lg)}
    `);

    const overlay = Object.assign(document.createElement('div'), {
        id: 'BirdOverlay'
    });
    overlay.innerHTML = `
        <div class="Bird-mainTitle">Yosef & Tamer</div>
        <div id="Bird-startScreen">
            <h1>Tamer <span id="Bird-version">v3.3.0</span></h1>
            <p class="Bird-subtitle">Watch rooms & chat in real time</p>
            <div class="Bird-modeOptions">
                <button class="Bird-modeBtn" id="Bird-modeArabic">
                    <span class="Bird-modeName">Arabic Rooms</span>
                    <span class="Bird-modeDesc">Private Arabic rooms</span>
                </button>
                <button class="Bird-modeBtn" id="Bird-modeEnglish">
                    <span class="Bird-modeName">English Rooms</span>
                    <span class="Bird-modeDesc">Private English rooms</span>
                </button>
                <button class="Bird-modeBtn" id="Bird-modeAll">
                    <span class="Bird-modeName">All Rooms</span>
                    <span class="Bird-modeDesc">All private rooms (except Turkish)</span>
                </button>
                <button class="Bird-modeBtn" id="Bird-modeBulgarian">
                    <span class="Bird-modeName">Bulgarian Rooms</span>
                    <span class="Bird-modeDesc">Private Bulgarian rooms</span>
                </button>
                <button class="Bird-modeBtn" id="Bird-modeKhmer">
                    <span class="Bird-modeName">Khmer Rooms</span>
                    <span class="Bird-modeDesc">Private Khmer rooms</span>
                </button>
                <button class="Bird-modeBtn" id="Bird-modePublic">
                    <span class="Bird-modeName">Public Rooms</span>
                    <span class="Bird-modeDesc">All languages</span>
                </button>
            </div>
            <div class="Bird-proxyRow">
                <button id="Bird-proxyBtn">Get Proxy (0)</button>
                <button id="Bird-proxyAddBtn" title="Activate 20 more Webshare proxies on the server">Add +20</button>
                <button id="Bird-proxyResetBtn" title="Clear local proxy cache and re-fetch from server">Reset</button>
            </div>
            <div class="Bird-credit">
                GameSketchers • by <a href="https://github.com/GameSketchers/Pastel-Live" target="_blank">Qwyua</a> <span class="heart">♥</span> with love
            </div>
        </div>
        <main id="Bird-app" class="Bird-main">
            <div class="Bird-left">
                <div class="Bird-filters" role="search" aria-label="Player search">
                    <input id="Bird-search" placeholder="Search Player" aria-label="Search players by name" title='Wrap in quotes for exact-word match, e.g. "us" matches "us" but not "user"'>
                    <input id="Bird-roomCode" placeholder="Search Room" aria-label="Search rooms by code">
                    <button id="Bird-refreshBtn">Refresh</button>
                    <button id="Bird-runOnServerBtn" title="Offload spectating to the server — browser becomes a viewer; closing the tab no longer stops the work">Run on server</button>
                    <div id="Bird-stats">
                        <span id="Bird-activePlayers">Active Players: 0</span>
                        <span id="Bird-activeRooms">Active Rooms: 0</span>
                    </div>
                </div>
                <div class="Bird-trackedBar" id="Bird-trackedBar" aria-label="Tracked players"><span class="Bird-trackedLabel">TRACKED</span></div>
                <div class="Bird-players" id="Bird-players">
                    <div class="Bird-loading" id="Bird-loadingIndicator">Connecting to rooms...</div>
                </div>
            </div>
            <div class="Bird-chat">
                <div class="Bird-chatHeader">
                    <span id="Bird-chatTitle">Chat</span>
                    <div class="Bird-chatFilters">
                        <label><input type="checkbox" id="Bird-filterAccounts"> Accounts Only</label>
                        <label title="How many of the most recent messages to load on reload. The newest page paints instantly; older messages stream in above in chunks until the cap is reached.">Load
                            <select id="Bird-chatLoadLimit" aria-label="Chat backfill limit">
                                <option value="10000">10k newest</option>
                                <option value="50000">50k newest</option>
                                <option value="100000">100k newest</option>
                                <option value="0">All (forever)</option>
                            </select>
                        </label>
                    </div>
                    <div class="Bird-chatButtons">
                        <button id="Bird-showAllChat" aria-label="Show all rooms' chat">Show All</button>
                        <button id="Bird-saveChat" aria-label="Save chat to file">Save</button>
                        <button id="Bird-resetChat" aria-label="Clear all chat messages">Clear</button>
                    </div>
                </div>
                <div class="Bird-searchMsg"><input id="Bird-msgSearch" placeholder="Search message" aria-label="Search chat messages" title='Wrap in quotes for exact-word match, e.g. "us" matches "us" but not "user"'></div>
                <div class="Bird-msgs" id="Bird-msgs" role="log" aria-live="polite" aria-label="Chat messages"></div>
            </div>
            <nav id="Bird-tabBar" class="Bird-tabBar" role="tablist" aria-label="Mobile panel switcher">
                <button class="Bird-tab" id="Bird-tabPlayers" role="tab" aria-selected="true" data-mv="players">
                    <span class="Bird-tabIcon" aria-hidden="true">◐</span>
                    <span class="Bird-tabLabel">Players</span>
                </button>
                <button class="Bird-tab" id="Bird-tabChat" role="tab" aria-selected="false" data-mv="chat">
                    <span class="Bird-tabIcon" aria-hidden="true">◑</span>
                    <span class="Bird-tabLabel">Chat</span>
                </button>
            </nav>
        </main>
    `;
    document.documentElement.prepend(overlay);

    // =========================================================================
    // Bird Class
    // =========================================================================
    class Bird {
        LANGUAGE_CODE = 19; // Arabic only
        // All gartic language codes EXCEPT Turkish(8), Portuguese(1), English(2), French(4), Russian(7), Chinese(9,16,17), Polish(10), Indonesian(45)
        ALL_LANGS = [3,6,11,12,13,14,15,18,19,20,21,22,23,24,25,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,46,47,48,49,50,51,52,53,54,55,56,57,58,59,60,61,62,63,64,65,66,67,68];
        proxies = GM_getValue("Bird-Proxies");
        activeSockets = [];
        activeEventSources = [];  // SSE connections for public rooms
        activeRoomFilters = new Set();
        activeAuthorNames = null;    // Set<string> of nicknames to filter chat by, or null for no filter
        activeAuthorLabel = null;    // display label (alias or lastKnownName) for the active author filter
        activeAuthorTrackedId = null; // tracked-user id behind the filter, so the editor can show toggle state
        roomColors = new Map();
        roomColorPalette = ['#e06c75','#61afef','#e5c07b','#c678dd','#56b6c2','#98c379','#d19a66','#be5046','#7ec8e3','#c3a6ff','#f0a6ca','#82d9a5'];
        trackedUsers = [];   // loaded in constructor with migration
        trackedOnline = new Map();  // trackedId → {name, room, avatar}
        mergeSource = null;  // trackedId during merge mode
        isRefreshing = false;
        refreshCycle = 0;
        staleCleanupTimer = null;
        connectedRooms = new Set();    // room codes with active WS connections
        _autoMonitorTimer = null;      // interval for room scanning
        _autoProxyTimer = null;        // interval for proxy refresh
        _autoMonitorActive = false;    // whether auto-monitor is running
        _lastScanTime = 0;            // timestamp of last room scan
        _totalRoomsEverSeen = 0;      // total unique rooms connected across session
        secb = enc()("aroRXI7nz2N1rwOdDuV2WvlvO1WqhaBI9TkWx04JQnysgb6tIDZdSswwsjHEwTNP7Mm+qySJ8Dy29fhxq+ZnZorH0IdPsa1g6IHOXqbabtW/kb9TfgUiGukAD5Y6aztVLtMVHhlh5Ve65wqEcPDonVlb+o/3","Qwyua");

        // Ping Worker: runs in a separate thread, immune to tab throttling.
        // Doubles as the watchdog for three failure modes the browser doesn't
        // surface on its own:
        //   1. CONNECTING-stuck — relay accepted TCP but upstream hung. The
        //      browser would hold readyState=CONNECTING for ~120s before
        //      onclose fires, during which scanForNewRooms keeps filtering
        //      the room out via connectedRooms.
        //   2. OPEN-but-never-joined — connection opened, join frame went
        //      out, but event 5 never came back (dropped frame, gartic refused
        //      without sending event 6). Engine.IO pings keep _lastFrameAt
        //      fresh so the zombie check below misses it.
        //   3. OPEN-but-silent — connection died mid-stream with no FIN.
        //      Engine.IO server pings every ~25s, so >60s of silence is
        //      decisive.
        // All three paths force-close so makeOnClose runs the retry chain.
        pingWorker = (() => {
            const code = 'setInterval(()=>postMessage("tick"),7000)';
            const w = new Worker(URL.createObjectURL(new Blob([code], {type:'application/javascript'})));
            const ZOMBIE_MS = 60000;
            const CONNECT_TIMEOUT_MS = 15000;
            const JOIN_TIMEOUT_MS = 12000;
            w.onmessage = () => {
                const now = Date.now();
                for (const ws of this.activeSockets) {
                    if (ws.readyState === WebSocket.CONNECTING) {
                        // _lastFrameAt is stamped at construction so it doubles as a creation timestamp.
                        if (now - (ws._lastFrameAt || now) > CONNECT_TIMEOUT_MS) {
                            try {
                                console.log(`[Bird] CONNECTING-stuck WS for room ${ws._roomCode || '?'} — force closing`);
                                ws.close(4001, 'connect-timeout');
                            } catch(e) {}
                        }
                        continue;
                    }
                    if (ws.readyState !== WebSocket.OPEN) continue;
                    ws.send("2"); // legacy keepalive — kept to avoid behavioral change
                    if (ws._openedAt && !ws._joined && now - ws._openedAt > JOIN_TIMEOUT_MS) {
                        try {
                            console.log(`[Bird] join-timeout WS for room ${ws._roomCode || '?'} — force closing`);
                            ws.close(4002, 'join-timeout');
                        } catch(e) {}
                        continue;
                    }
                    const last = ws._lastFrameAt || 0;
                    if (last && now - last > ZOMBIE_MS) {
                        try {
                            console.log(`[Bird] zombie WS for room ${ws._roomCode || '?'} (idle ${Math.round((now-last)/1000)}s) — force closing`);
                            ws.close(4000, 'zombie');
                        } catch(e) {}
                    }
                }
            };
            return w;
        })();

        _debounce(fn, ms) {
            let timer;
            return (...args) => { clearTimeout(timer); timer = setTimeout(() => fn.apply(this, args), ms); };
        }

        // Schedules fn to run once on next animation frame (coalesces rapid calls)
        _scheduleOnce(key, fn) {
            if (this._scheduledFrames?.[key]) return;
            if (!this._scheduledFrames) this._scheduledFrames = {};
            this._scheduledFrames[key] = requestAnimationFrame(() => {
                delete this._scheduledFrames[key];
                fn.call(this);
            });
        }

        // A query wrapped in straight double quotes (e.g. "us") means exact-word match
        // — "us" matches "us" but not "user" or "using". Unquoted = substring (default).
        _parseSearchQuery(raw) {
            const q = (raw || '').trim();
            if (q.length >= 2 && q.charCodeAt(0) === 34 && q.charCodeAt(q.length - 1) === 34) {
                const term = q.slice(1, -1);
                return { term, exact: true, empty: term.length === 0 };
            }
            return { term: q, exact: false, empty: q.length === 0 };
        }

        // Word boundary uses \p{L}|\p{N}|_ so Arabic and other Unicode letters work,
        // unlike \b which only treats ASCII as word chars.
        _buildExactWordRegex(termLower) {
            if (!this._exactWordReCache) this._exactWordReCache = new Map();
            const cached = this._exactWordReCache.get(termLower);
            if (cached) return cached;
            const esc = termLower.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            const re = new RegExp(`(?:^|[^\\p{L}\\p{N}_])${esc}(?=$|[^\\p{L}\\p{N}_])`, 'u');
            if (this._exactWordReCache.size > 64) this._exactWordReCache.clear();
            this._exactWordReCache.set(termLower, re);
            return re;
        }

        constructor(overlay) {
            this.overlay = overlay;
            this.playersContainer = overlay.querySelector("#Bird-players");
            this.messagesContainer = overlay.querySelector("#Bird-msgs");
            this.startScreen = overlay.querySelector("#Bird-startScreen");
            this.app = overlay.querySelector("#Bird-app");
            this.activePlayersSpan = overlay.querySelector("#Bird-activePlayers");
            this.activeRoomsSpan = overlay.querySelector("#Bird-activeRooms");
            this.refreshButton = overlay.querySelector("#Bird-refreshBtn");
            this.runOnServerBtn = overlay.querySelector("#Bird-runOnServerBtn");
            this.proxyButton = overlay.querySelector("#Bird-proxyBtn");
            this.proxyAddButton = overlay.querySelector("#Bird-proxyAddBtn");
            this.proxyResetButton = overlay.querySelector("#Bird-proxyResetBtn");
            this.searchInput = overlay.querySelector("#Bird-search");
            this.roomInput = overlay.querySelector("#Bird-roomCode");
            this.msgSearchInput = overlay.querySelector("#Bird-msgSearch");
            this.accountFilterCheckbox = overlay.querySelector("#Bird-filterAccounts");
            this.chatLoadLimitSelect = overlay.querySelector("#Bird-chatLoadLimit");
            this.chatTitle = overlay.querySelector("#Bird-chatTitle");
            this.showAllChatBtn = overlay.querySelector("#Bird-showAllChat");
            this.modeArabic = overlay.querySelector("#Bird-modeArabic");
            this.modeEnglish = overlay.querySelector("#Bird-modeEnglish");
            this.modeAll = overlay.querySelector("#Bird-modeAll");
            this.modeBulgarian = overlay.querySelector("#Bird-modeBulgarian");
            this.modeKhmer = overlay.querySelector("#Bird-modeKhmer");
            this.modePublic = overlay.querySelector("#Bird-modePublic");
            this.loadingIndicator = overlay.querySelector("#Bird-loadingIndicator");
            this.trackedBar = overlay.querySelector("#Bird-trackedBar");
            this.overlay = overlay;
            this.tabPlayers = overlay.querySelector("#Bird-tabPlayers");
            this.tabChat = overlay.querySelector("#Bird-tabChat");
            overlay.dataset.mv = "players";

            // Load tracked users with migration from old format
            const raw = GM_getValue("Bird-TrackedUsers", []);
            this.trackedUsers = raw.map(t => {
                if (t.identifiers) {
                    if (typeof t.bellOn !== 'boolean') t.bellOn = false;
                    t.autoDeploy = !!t.autoDeploy;
                    t.autoDeployName = t.autoDeployName || 'Botnik 1';
                    t.autoDeployMessage = t.autoDeployMessage || '';
                    t.autoDeployKick = !!t.autoDeployKick;
                    t.autoDeployLoyalty = !!t.autoDeployLoyalty;
                    t.autoDeployAIChat = !!t.autoDeployAIChat;
                    t.autoDeployAIPersona = typeof t.autoDeployAIPersona === 'string' ? t.autoDeployAIPersona : '';
                    return t;
                }
                // Migrate old {fotoUrl, lastKnownName} → new format
                return {
                    id: String(Date.now()) + Math.random().toString(36).slice(2, 6),
                    alias: null,
                    lastKnownName: t.lastKnownName,
                    lastKnownFoto: t.fotoUrl,
                    bellOn: false,
                    autoDeploy: !!t.autoDeploy,
                    autoDeployName: t.autoDeployName || 'Botnik 1',
                    autoDeployMessage: '',
                    autoDeployKick: false,
                    autoDeployLoyalty: false,
                    autoDeployAIChat: false,
                    autoDeployAIPersona: '',
                    identifiers: [
                        ...(t.fotoUrl ? [{ type: 'foto', value: t.fotoUrl }] : []),
                        ...(t.lastKnownName ? [{ type: 'name', value: t.lastKnownName }] : [])
                    ]
                };
            });
            if (raw.length && !raw[0]?.identifiers) this.saveTracked(); // persist migration

            // One-shot migration: strip known testing-only uuid identifiers that
            // got auto-added/backfilled before they were recognized as the user's
            // own test identities. Without this, every client→server push
            // (updateTrackedStatus, executeMerge, editor save) re-uploads the bad
            // uuid and undoes the server-side cleanup, looping forever.
            //
            // Add more values here later if the user identifies further testing
            // identities. List is intentionally tiny — no false-positive risk.
            const TESTING_UUIDS_TO_STRIP = new Set([
                "ae90f2769-26ba-48f8-9090-bc11061f90c7", // guest: used Ev + SARAHUNA + الشريف
                "a0ef81132-40f6-4346-8d85-55a6628785de", // guest: used SARAHUNA + الشريف
            ]);
            let strippedAny = false;
            const dirtyTracked = [];
            for (const t of this.trackedUsers) {
                if (!Array.isArray(t.identifiers)) continue;
                const before = t.identifiers.length;
                t.identifiers = t.identifiers.filter(
                    i => !(i.type === "uuid" && TESTING_UUIDS_TO_STRIP.has(i.value))
                );
                if (t.identifiers.length !== before) {
                    strippedAny = true;
                    dirtyTracked.push(t);
                }
            }
            if (strippedAny) {
                this.saveTracked();
                // Push cleaned state up so the server stops getting reinfected
                // on the next normal sync push. Fire-and-forget; the periodic
                // syncAutoDeployFromServer will heal any miss.
                for (const t of dirtyTracked) {
                    if (t.autoDeploy) this.syncAutoDeploy(t).catch(() => {});
                }
                console.log("[Bird] one-shot migration: stripped testing uuids from",
                            dirtyTracked.length, "tracked entries");
            }

            // Bell state — master mute persisted, cooldown map in-memory only
            this.bellMuted = GM_getValue("Bird-BellMuted", false);
            this._lastBellAt = new Map();

            // Master auto-deploy toggle — server-owned (shared across all Bird
            // tabs / users). Default true until syncAutoDeployFromServer fills
            // it in. Local-only cache; the real source of truth is bird-server.
            this._masterAutoDeploy = true;

            // Per-room immunity cache. Same source-of-truth pattern as above;
            // populated from /api/auto-deploy on boot and after every toggle.
            this._immuneRooms = new Set();
            document.addEventListener('visibilitychange', () => {
                if (!document.hidden) { this._stopBeepLoop(); this._stopTitleFlash(); this._restoreFavicon(); }
            });

            this.playerStores = {};

            // Chat history — bounded at 100K messages (hard cap in addMessage)
            this._chatHistory = [];
            this._userScrolledUp = false;
            this._chatScroller = this._createVirtualScroller();
            this._chatSearchWorker = this._createChatSearchWorker();

            this.initEvents();
            this.renderTrackedBar();

            // Proxy init
            if (this.proxies === undefined) {
                this.proxies = [];
                GM_setValue("Bird-Proxies", this.proxies);
            }
            if (Array.isArray(this.proxies) && this.proxies.length === 0) {
                this.proxies = [];
                GM_setValue("Bird-Proxies", this.proxies);
            }
            this.updateModeButtons();

            // Proxy validation removed — token server relay handles proxy health
            this.proxyButton.textContent = `Get Proxy (${this.proxies?.length || 0})`;

            // Server-session bootstrap: if the sidecar already has a session running,
            // hide the start screen and resume from its snapshot.
            this._maybeResumeServerSession();

            // Pull the server-owned follow list (non-destructive — never wipes
            // entries this browser doesn't own).
            this.syncAutoDeployFromServer();
        }

        // =================================================================
        // Events
        // =================================================================
        initEvents() {
            this.searchInput.addEventListener("input", this._debounce(() => this.filterPlayers(), 150));
            this.roomInput.addEventListener("input", this._debounce(() => this.filterPlayers(), 150));
            this.msgSearchInput.addEventListener("input", this._debounce(() => this.filterChat(), 150));

            this.modeArabic.addEventListener("click", () => this.startMode('arabic'));
            this.modeEnglish.addEventListener("click", () => this.startMode('english'));
            this.modeAll.addEventListener("click", () => this.startMode('all'));
            this.modeBulgarian.addEventListener("click", () => this.startMode('bulgarian'));
            this.modeKhmer.addEventListener("click", () => this.startMode('khmer'));
            this.modePublic.addEventListener("click", () => this.startMode('public'));

            this.refreshButton.addEventListener("click", () => this.refreshLogic());
            this.runOnServerBtn.addEventListener("click", () => this._toggleServerSession());

            // Mobile tab bar: swap visible panel (CSS handles the visibility via [data-mv])
            const setMobileView = (mv) => {
                this.overlay.dataset.mv = mv;
                this.tabPlayers.setAttribute("aria-selected", mv === "players" ? "true" : "false");
                this.tabChat.setAttribute("aria-selected", mv === "chat" ? "true" : "false");
                // Scroll chat to bottom when entering it (virtual scroller needs a nudge)
                if (mv === "chat" && this.messagesContainer) {
                    requestAnimationFrame(() => {
                        this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
                    });
                }
            };
            this.tabPlayers.addEventListener("click", () => setMobileView("players"));
            this.tabChat.addEventListener("click", () => setMobileView("chat"));

            this.proxyButton.addEventListener('click', () => {
                this.updateModeButtons(true);
                this.proxyButton.disabled = true;
                this.addNewProxy();
            });

            this.proxyAddButton.addEventListener('click', async () => {
                const original = this.proxyAddButton.textContent;
                this.proxyAddButton.disabled = true;
                this.proxyAddButton.textContent = "Adding...";
                try {
                    const r = await fetch('/bird/api/webshare-add?n=20');
                    if (r.status === 404) {
                        this.proxyAddButton.textContent = "Rebuild server";
                        console.error('[proxyAdd] /api/webshare-add returned 404 — running an old icebot-server.exe. Restart start-webshare.bat to pick up the rebuilt exe.');
                        setTimeout(() => { this.proxyAddButton.textContent = original; this.proxyAddButton.disabled = false; }, 3500);
                        return;
                    }
                    const data = await r.json();
                    if (!data.ok) throw new Error('server returned ok=false');
                    if (data.added === 0 && data.message) {
                        this.proxyAddButton.textContent = "No deferred";
                    } else {
                        this.proxyAddButton.textContent = `+${data.added} ok`;
                    }
                    await this.addNewProxy();
                } catch (e) {
                    console.error('[proxyAdd]', e);
                    this.proxyAddButton.textContent = "Server offline";
                }
                setTimeout(() => {
                    this.proxyAddButton.textContent = original;
                    this.proxyAddButton.disabled = false;
                }, 2500);
            });

            this.proxyResetButton.addEventListener('click', async () => {
                const original = this.proxyResetButton.textContent;
                this.proxyResetButton.disabled = true;
                this.proxyResetButton.textContent = "Resetting...";
                try {
                    const r = await fetch('/bird/api/webshare-reset');
                    if (r.status === 404) {
                        this.proxyResetButton.textContent = "Rebuild server";
                        console.error('[proxyReset] /api/webshare-reset returned 404 — restart start-webshare.bat to pick up the rebuilt exe.');
                        setTimeout(() => { this.proxyResetButton.textContent = original; this.proxyResetButton.disabled = false; }, 3500);
                        return;
                    }
                    const data = await r.json();
                    if (!data.ok) throw new Error('server returned ok=false');
                    this.proxies = [];
                    this._proxyFails = {};
                    GM_setValue("Bird-Proxies", []);
                    this.proxyButton.textContent = "Get Proxy (0)";
                    this.updateModeButtons();
                    this.proxyResetButton.textContent = `Reset (${data.moved}→0)`;
                } catch (e) {
                    console.error('[proxyReset]', e);
                    this.proxyResetButton.textContent = "Server offline";
                }
                setTimeout(() => {
                    this.proxyResetButton.textContent = original;
                    this.proxyResetButton.disabled = false;
                }, 2500);
            });

            this.overlay.querySelector("#Bird-resetChat").addEventListener("click", () => {
                this._chatHistory = [];
                this._chatSearchWorker.postMessage({ type: 'clear' });
                this._chatScroller.setVisibleIndices(null);
                this._userScrolledUp = false;
                // Reset clears messages — also drop the author filter (it's pointing
                // at nothing now); rooms filter is preserved since it's a coarse view.
                this.activeAuthorNames = null;
                this.activeAuthorLabel = null;
                this.activeAuthorTrackedId = null;
                this._updateChatTitle();
            });

            this.overlay.querySelector("#Bird-saveChat").addEventListener("click", () => {
                const parsed = this._parseSearchQuery(this.msgSearchInput.value);
                const searchText = parsed.empty ? '' : parsed.term.toLowerCase();
                const exactRe = (parsed.exact && searchText) ? this._buildExactWordRegex(searchText) : null;
                const hasRoomFilter = this.activeRoomFilters.size > 0;
                const authorNames = this.activeAuthorNames;
                const lines = this._chatHistory
                    .filter(m => {
                        if (hasRoomFilter && !this.activeRoomFilters.has(m.room)) return false;
                        if (authorNames && !authorNames.has(m.author)) return false;
                        if (searchText) {
                            const hay = (`${m.author} ${m.text}`).toLowerCase();
                            if (exactRe ? !exactRe.test(hay) : !hay.includes(searchText)) return false;
                        }
                        return true;
                    })
                    .map(m => {
                        const shortRoom = m.room.length > 12 ? m.room.slice(0, 12) + '\u2026' : m.room;
                        return `[${shortRoom}] ${m.author}: ${m.text}`;
                    });
                const msgs = lines.join('\n');
                if (!msgs) return;
                const blob = new Blob([msgs], { type: 'text/plain' });
                const a = document.createElement('a');
                a.href = URL.createObjectURL(blob);
                a.download = `bird-chat-${Date.now()}.txt`;
                a.click();
            });

            this.accountFilterCheckbox.addEventListener("change", () => {
                this.filterChat();
            });

            // Chat backfill limit — persists across reloads. Changing it only
            // affects the NEXT load (existing in-memory chat stays put), so we
            // just persist the choice and let _backfillNewestPage read it on
            // resume / next reload. Value 0 means "All (forever)".
            this._chatLoadLimit = 10000;
            if (this.chatLoadLimitSelect) {
                const stored = localStorage.getItem('bird-chat-load-limit');
                if (stored !== null) {
                    const opt = this.chatLoadLimitSelect.querySelector(`option[value="${CSS.escape(stored)}"]`);
                    if (opt) this.chatLoadLimitSelect.value = stored;
                }
                this._chatLoadLimit = parseInt(this.chatLoadLimitSelect.value, 10) || 0;
                this.chatLoadLimitSelect.addEventListener('change', () => {
                    this._chatLoadLimit = parseInt(this.chatLoadLimitSelect.value, 10) || 0;
                    try { localStorage.setItem('bird-chat-load-limit', this.chatLoadLimitSelect.value); } catch(e) {}
                });
            }

            this.showAllChatBtn.addEventListener("click", () => {
                this.activeRoomFilters.clear();
                this.overlay.querySelectorAll('.Bird-filterRoomBtn.active').forEach(b => b.classList.remove('active'));
                // "Show All" = drop ALL view-narrowing filters (rooms + author).
                this.activeAuthorNames = null;
                this.activeAuthorLabel = null;
                this.activeAuthorTrackedId = null;
                this._updateChatTitle();
                this.filterChat();
            });
        }

        // =================================================================
        // Mode launch & button state
        // =================================================================

        startMode(mode) {
            this.activeMode = mode;
            this.startScreen.style.display = "none";
            this.app.classList.add("Bird-main--active");
            const langs = this.getSelectedLangs();
            const hasPrivate = mode === 'arabic' || mode === 'english' || mode === 'all' || mode === 'bulgarian' || mode === 'khmer';
            if (langs.length > 0 && hasPrivate) {
                if (this._serverSession) {
                    // Server is authoritative — events arrive via SSE; skip local scan.
                    console.log('[Bird] startMode bypassed local scan — server session active');
                } else {
                    this.playerSearchGO(langs);
                    // Start auto-monitor for continuous room discovery
                    this.startAutoMonitor();
                }
            }
            if (mode === 'public') {
                if (this.loadingIndicator) {
                    this.loadingIndicator.textContent = "Connecting to public rooms...";
                }
                this.connectPublicRooms(this.ALL_LANGS);
                setTimeout(() => {
                    if (this.loadingIndicator) {
                        this.loadingIndicator.remove();
                        this.loadingIndicator = null;
                    }
                }, 15000);
            }
            this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
        }

        updateModeButtons(fetching = false) {
            if (fetching) {
                this.modeArabic.disabled = true;
                this.modeEnglish.disabled = true;
                this.modeAll.disabled = true;
                this.modeBulgarian.disabled = true;
                this.modeKhmer.disabled = true;
                this.modePublic.disabled = true;
            } else {
                const hasProxies = this.proxies?.length > 0;
                this.modeArabic.disabled = !hasProxies;
                this.modeEnglish.disabled = !hasProxies;
                this.modeAll.disabled = !hasProxies;
                this.modeBulgarian.disabled = !hasProxies;
                this.modeKhmer.disabled = !hasProxies;
                this.modePublic.disabled = false;
            }
        }

        // =================================================================
        // SMART DIFF-BASED REFRESH
        // =================================================================

        refreshLogic() {
            if (this.isRefreshing) return;
            this.isRefreshing = true;

            // Spinner on button
            this.refreshButton.classList.add('Bird-refreshing');
            this.refreshButton.disabled = true;

            if (this.isPublicMode()) {
                // Public mode: full reconnect (SSE-based, no persistence needed)
                this.refreshCycle++;
                const cycle = this.refreshCycle;
                this.activeEventSources.forEach(es => { try { es.close(); } catch(e){} });
                this.activeEventSources = [];
                this.playersContainer.querySelectorAll('.Bird-room-group').forEach(roomDiv => {
                    roomDiv.setAttribute('data-stale', '1');
                    roomDiv.querySelectorAll('.Bird-card').forEach(card => card.setAttribute('data-stale', '1'));
                });
                if (this.staleCleanupTimer) clearTimeout(this.staleCleanupTimer);
                const langs = this.getSelectedLangs();
                this.connectPublicRooms(langs.length > 0 ? langs : this.ALL_LANGS);
                this.staleCleanupTimer = setTimeout(() => {
                    if (this.refreshCycle !== cycle) return;
                    this.sweepStale();
                    this.isRefreshing = false;
                    this.refreshButton.classList.remove('Bird-refreshing');
                    this.refreshButton.disabled = false;
                }, 25000);
            } else {
                // Private mode: incremental — keep live connections, just scan for new rooms
                // Also clean up dead sockets
                this.activeSockets = this.activeSockets.filter(ws => {
                    if (ws.readyState === WebSocket.CLOSED || ws.readyState === WebSocket.CLOSING) {
                        return false;
                    }
                    return true;
                });

                // Trigger an immediate scan for new rooms
                this.scanForNewRooms().then(() => {
                    // Also refresh proxies
                    return this.refreshProxiesSilent();
                }).finally(() => {
                    this.isRefreshing = false;
                    this.refreshButton.classList.remove('Bird-refreshing');
                    this.refreshButton.disabled = false;
                });

                // Restart auto-monitor if it wasn't running
                this.startAutoMonitor();
            }
        }

        showEmptyState(message) {
            if (this.playersContainer.querySelector('.Bird-empty-state')) return;
            const empty = Object.assign(document.createElement('div'), {
                className: 'Bird-empty-state',
                textContent: message
            });
            this.playersContainer.appendChild(empty);
        }

        hideEmptyState() {
            this.playersContainer.querySelector('.Bird-empty-state')?.remove();
        }

        /**
         * sweepStale — after refresh settles, remove DOM that wasn't reconfirmed.
         * Cards fade out first, then empty rooms collapse.
         */
        sweepStale() {
            // Fade out stale cards
            this.playersContainer.querySelectorAll('.Bird-card[data-stale="1"]').forEach(card => {
                card.classList.add('Bird-card-leaving');
                card.addEventListener('animationend', () => card.remove(), { once: true });
            });

            // After card animations, check for newly-empty rooms
            setTimeout(() => {
                this.playersContainer.querySelectorAll('.Bird-room-group').forEach(roomDiv => {
                    const alive = roomDiv.querySelectorAll('.Bird-card:not(.Bird-card-leaving)');
                    if (alive.length === 0) {
                        roomDiv.classList.add('Bird-room-leaving');
                        roomDiv.addEventListener('animationend', () => roomDiv.remove(), { once: true });
                    } else {
                        roomDiv.removeAttribute('data-stale');
                    }
                });
                this.filterPlayers();
                // Show empty state if no rooms remain
                const aliveRooms = this.playersContainer.querySelectorAll('.Bird-room-group:not(.Bird-room-leaving)');
                if (aliveRooms.length === 0) {
                    this.showEmptyState('No active rooms. Try refreshing.');
                }
            }, 500);
        }

        // =================================================================
        // AUTO-MONITOR — continuous room scanning & proxy refresh
        // =================================================================

        startAutoMonitor() {
            if (this._serverSession) {
                console.log('[Bird] auto-monitor disabled — server session is authoritative');
                return;
            }
            if (this._autoMonitorActive) return;
            this._autoMonitorActive = true;
            console.log('[AutoMonitor] Started — scanning every 20s, proxies every 60s');

            // Room scan every 20 seconds
            this._autoMonitorTimer = setInterval(() => this.scanForNewRooms(), 20000);

            // Proxy refresh every 60 seconds
            this._autoProxyTimer = setInterval(() => this.refreshProxiesSilent(), 60000);

            this._updateAutoMonitorStatus();
        }

        stopAutoMonitor() {
            this._autoMonitorActive = false;
            if (this._autoMonitorTimer) { clearInterval(this._autoMonitorTimer); this._autoMonitorTimer = null; }
            if (this._autoProxyTimer) { clearInterval(this._autoProxyTimer); this._autoProxyTimer = null; }
            console.log('[AutoMonitor] Stopped');
        }

        async scanForNewRooms() {
            if (this._serverSession) return; // server session is authoritative
            if (this._isScanning) return; // prevent overlapping scans
            this._isScanning = true;
            // Per-IP fail counts are scan-local — without this reset they ratchet up
            // monotonically across hours and eventually starve every IP, even when the
            // proxies themselves are perfectly healthy.
            this._proxyFails = {};
            const langs = this.getSelectedLangs();
            if (!langs.length) { this._isScanning = false; return; }

            try {
                const res = await new Promise(resolve => this.monitorRooms(langs, resolve));
                const flatList = [];
                for (const [lang, arr] of Object.entries(res)) {
                    for (const code of arr) flatList.push({ language: Number(lang), code });
                }

                // Filter to only NEW rooms we don't already have a connection to
                const newRooms = flatList.filter(r => !this.connectedRooms.has(r.code));

                if (newRooms.length > 0) {
                    console.log(`[AutoMonitor] Found ${newRooms.length} new room(s) (${flatList.length} total, ${this.connectedRooms.size} already connected)`);
                    this.hideEmptyState();
                    await this._connectNewRooms(newRooms);
                }

                this._lastScanTime = Date.now();
                this._updateAutoMonitorStatus();
            } catch(e) {
                console.error('[AutoMonitor] Scan error:', e);
            }
            this._isScanning = false;
        }

        async _connectNewRooms(rooms) {
            if (!this.proxies?.length) return;

            const distributed = {};
            this.proxies.forEach(p => {
                const ip = typeof p === 'object' ? p.ip : p;
                distributed[ip] = [];
            });

            let i = 0;
            for (const item of rooms) {
                const proxy = this.proxies[i % this.proxies.length];
                const ip = typeof proxy === 'object' ? proxy.ip : proxy;
                const cookie = typeof proxy === 'object' ? proxy.cookie : '';
                distributed[ip].push({ language: item.language, roomcode: item.code, ip, cookie });
                i++;
            }

            const sleep = ms => new Promise(r => setTimeout(r, ms));

            for (const [ip, queue] of Object.entries(distributed)) {
                (async () => {
                    for (const job of queue) {
                        const result = await this.checkDeepProxy(job);
                        this.connectToRoom(
                            { ip: result.info.ip, cookie: result.info.cookie },
                            result.info.roomcode,
                            result.response,
                            result.server
                        );
                        await sleep(2000);
                    }
                })();
            }
        }

        async refreshProxiesSilent() {
            try {
                const r = await fetch('/bird/api/proxies');
                const data = await r.json();
                if (!data.ok || !Array.isArray(data.proxies)) return;
                // Replace, don't union: the server's /api/proxies is the source of
                // truth for *currently healthy* Webshare proxies. Webshare entries
                // flap in/out of healthy state; unioning across refreshes accumulates
                // every IP that was ever healthy until the count balloons (30-40 → 100+
                // over a long session). Replace each refresh so the count tracks the
                // real active pool. We don't need cross-refresh persistence — the next
                // refresh in 60s rebuilds the list from the server snapshot anyway.
                const before = this.proxies?.length || 0;
                this.proxies = data.proxies.map(p => ({ ip: p.ip, cookie: p.cookie || '' }));
                GM_setValue("Bird-Proxies", this.proxies);
                if (this.proxies.length !== before) {
                    console.log(`[AutoMonitor] Proxy refresh: ${before} → ${this.proxies.length} (replaced from server snapshot)`);
                    this.proxyButton.textContent = `Get Proxy (${this.proxies.length})`;
                }
            } catch(e) {
                // Server not available — silent fail, keep using existing proxies
            }
        }

        _updateAutoMonitorStatus() {
            let statusEl = this.overlay.querySelector('#Bird-autoMonitorStatus');
            if (!statusEl) {
                const stats = this.overlay.querySelector('#Bird-stats');
                if (!stats) return;
                statusEl = document.createElement('span');
                statusEl.id = 'Bird-autoMonitorStatus';
                statusEl.style.cssText = 'color:var(--pl-accent-online);font-size:var(--pl-font-size-xs)';
                stats.appendChild(statusEl);
            }
            const ago = this._lastScanTime ? Math.round((Date.now() - this._lastScanTime) / 1000) : 0;
            const connected = this.connectedRooms.size;
            statusEl.textContent = this._autoMonitorActive
                ? `AUTO-MONITOR ON — ${connected} rooms connected${ago ? ` — last scan ${ago}s ago` : ''}`
                : '';
        }

        // =================================================================
        // Tracked Users (multi-identifier with merge)
        // =================================================================

        saveTracked() { GM_setValue("Bird-TrackedUsers", this.trackedUsers); this._trackedByFoto = null; this._trackedByName = null; this._trackedByUUID = null; }

        async syncAutoDeploy(t) {
            try {
                if (t.autoDeploy) {
                    await fetch('/bird/api/auto-deploy/upsert', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            trackedID: t.id,
                            name: t.autoDeployName || 'Botnik 1',
                            identifiers: t.identifiers || [],
                            enabled: true,
                            message: t.autoDeployMessage || '',
                            kick: !!t.autoDeployKick,
                            loyalty: !!t.autoDeployLoyalty,
                            aiChat: !!t.autoDeployAIChat,
                            aiPersona: t.autoDeployAIPersona || '',
                        }),
                    });
                } else {
                    await fetch('/bird/api/auto-deploy/delete?id=' + encodeURIComponent(t.id), { method: 'DELETE' });
                }
            } catch (e) { console.warn('[Bird] auto-deploy sync failed:', e); }
        }

        // Non-destructive boot sync. The server's auto-deploy registry is the
        // source of truth for the follow list — it is shared across all Bird
        // users and the 24/7 watcher reads it. This Bird page only ever touches
        // its OWN entries: trackedID is globally unique per browser, so a
        // server entry created by another user never matches a local pill and
        // is left untouched. Nothing here deletes — that is what stopped one
        // user's page load from wiping every other user's follows.
        // Best-effort POST to flip the server-side master gate. Updates the
        // local cache + re-renders the tracked bar synchronously so the button
        // doesn't lag the click; the server's reply is authoritative and
        // corrects the cache if it disagrees.
        async setMasterAutoDeploy(enabled) {
            const want = !!enabled;
            this._masterAutoDeploy = want;
            this.renderTrackedBar();
            try {
                const r = await fetch('/bird/api/auto-deploy/master?enabled=' + (want ? 'true' : 'false'), { method: 'POST' });
                if (!r.ok) throw new Error('HTTP ' + r.status);
                const j = await r.json();
                if (typeof j.enabled === 'boolean' && j.enabled !== this._masterAutoDeploy) {
                    this._masterAutoDeploy = j.enabled;
                    this.renderTrackedBar();
                }
            } catch (e) {
                console.warn('[Bird] master auto-deploy toggle failed:', e);
                // Roll back optimistic flip on failure.
                this._masterAutoDeploy = !want;
                this.renderTrackedBar();
            }
        }

        // Toggle immunity for one room. Optimistic local update + POST/DELETE;
        // server reply is authoritative and corrects the cache on disagreement.
        async setRoomImmune(roomCode, immune) {
            if (!roomCode) return;
            const want = !!immune;
            if (want) this._immuneRooms.add(roomCode);
            else this._immuneRooms.delete(roomCode);
            this.renderTrackedBar();
            this._refreshImmuneRoomDecorations();
            try {
                const url = '/bird/api/auto-deploy/immune-rooms?room=' + encodeURIComponent(roomCode) +
                            '&immune=' + (want ? 'true' : 'false');
                const r = await fetch(url, { method: want ? 'POST' : 'DELETE' });
                if (!r.ok) throw new Error('HTTP ' + r.status);
                const j = await r.json();
                if (Array.isArray(j.rooms)) {
                    this._immuneRooms = new Set(j.rooms);
                    this.renderTrackedBar();
                    this._refreshImmuneRoomDecorations();
                }
            } catch (e) {
                console.warn('[Bird] immune-room toggle failed:', e);
                // Roll back the optimistic flip on failure.
                if (want) this._immuneRooms.delete(roomCode);
                else this._immuneRooms.add(roomCode);
                this.renderTrackedBar();
                this._refreshImmuneRoomDecorations();
            }
        }

        // Repaint shield buttons + room outlines after the immune set changes
        // — cheaper than rerendering every room card.
        _refreshImmuneRoomDecorations() {
            const groups = this.playersContainer?.querySelectorAll('.Bird-room-group') || [];
            for (const g of groups) {
                const code = g.getAttribute('data-room-code');
                if (!code) continue;
                const on = this._immuneRooms.has(code);
                g.classList.toggle('immune', on);
                const shield = g.querySelector('.Bird-immuneShield');
                if (shield) {
                    shield.classList.toggle('on', on);
                    shield.title = on
                        ? 'Room is immune to auto-deploy — click to allow bots again'
                        : 'Make room immune to auto-deploy (no bots, no loops, no recovery refire)';
                }
            }
        }

        openImmuneRoomsPanel() {
            // Reuse the edit-overlay pattern.
            this.overlay.querySelector('.Bird-immuneOverlay')?.remove();
            const ov = document.createElement('div');
            ov.className = 'Bird-immuneOverlay';
            const panel = document.createElement('div');
            panel.className = 'Bird-immunePanel';

            const title = document.createElement('h3');
            title.textContent = 'Auto-deploy immune rooms';
            panel.appendChild(title);

            const hint = Object.assign(document.createElement('div'), {
                style: 'color:var(--pl-text-secondary);font-size:var(--pl-font-size-xs)',
                textContent: 'Rooms here never get auto-deploy bots, even if tracked players join them.'
            });
            panel.appendChild(hint);

            const list = document.createElement('div');
            list.style.cssText = 'display:flex;flex-direction:column;gap:6px;margin-top:var(--pl-space-2)';
            const renderList = () => {
                list.innerHTML = '';
                if (this._immuneRooms.size === 0) {
                    const e = document.createElement('div');
                    e.className = 'empty';
                    e.textContent = 'No immune rooms yet.';
                    list.appendChild(e);
                    return;
                }
                const sorted = [...this._immuneRooms].sort();
                for (const code of sorted) {
                    const row = document.createElement('div');
                    row.className = 'row';
                    const codeSpan = Object.assign(document.createElement('span'), { className: 'code', textContent: code });
                    const removeBtn = Object.assign(document.createElement('button'), {
                        type: 'button', textContent: '×', title: 'Remove from immune list'
                    });
                    removeBtn.addEventListener('click', async () => {
                        await this.setRoomImmune(code, false);
                        renderList();
                    });
                    row.append(codeSpan, removeBtn);
                    list.appendChild(row);
                }
            };
            renderList();
            panel.appendChild(list);

            const addRow = document.createElement('div');
            addRow.className = 'add';
            const input = Object.assign(document.createElement('input'), {
                type: 'text',
                placeholder: 'Room code, e.g. 49k5w6 — paste URL too',
                autocomplete: 'off',
                spellcheck: false,
            });
            const addBtn = Object.assign(document.createElement('button'), { type: 'button', textContent: 'Add' });
            const submit = async () => {
                let code = input.value.trim();
                if (!code) return;
                // Accept full gartic URLs and pluck the room slug from the end.
                const m = code.match(/gartic\.io\/([a-zA-Z0-9_-]+)/);
                if (m) code = m[1];
                if (this._immuneRooms.has(code)) {
                    input.value = '';
                    return;
                }
                input.value = '';
                await this.setRoomImmune(code, true);
                renderList();
            };
            addBtn.addEventListener('click', submit);
            input.addEventListener('keydown', e => { if (e.key === 'Enter') submit(); });
            addRow.append(input, addBtn);
            panel.appendChild(addRow);

            const closeRow = document.createElement('div');
            closeRow.className = 'closeRow';
            const closeBtn = Object.assign(document.createElement('button'), { type: 'button', textContent: 'Close' });
            closeBtn.addEventListener('click', () => ov.remove());
            closeRow.appendChild(closeBtn);
            panel.appendChild(closeRow);

            ov.appendChild(panel);
            ov.addEventListener('click', e => { if (e.target === ov) ov.remove(); });
            this.overlay.appendChild(ov);
            input.focus();
        }

        async syncAutoDeployFromServer() {
            try {
                const r = await fetch('/bird/api/auto-deploy');
                if (!r.ok) return;
                const payload = await r.json();
                const entries = payload.entries || [];
                // masterEnabled may be missing on a very old server build; treat
                // an absent field as "on" so we don't show a misleading OFF pill.
                const master = (payload.masterEnabled === false) ? false : true;
                if (this._masterAutoDeploy !== master) {
                    this._masterAutoDeploy = master;
                    this.renderTrackedBar();
                }
                const serverImmune = Array.isArray(payload.immuneRooms) ? payload.immuneRooms : [];
                const incoming = new Set(serverImmune);
                // Only rerender if the cached set actually drifted.
                let drift = incoming.size !== this._immuneRooms.size;
                if (!drift) {
                    for (const c of incoming) { if (!this._immuneRooms.has(c)) { drift = true; break; } }
                }
                if (drift) {
                    this._immuneRooms = incoming;
                    this.renderTrackedBar();
                    this._refreshImmuneRoomDecorations();
                }
                const byID = new Map(entries.map(e => [e.trackedID, e]));
                let changed = false;
                for (const t of this.trackedUsers) {
                    const e = byID.get(t.id);
                    if (e) {
                        // Server has this follow — reflect server truth onto the pill.
                        if (!t.autoDeploy) { t.autoDeploy = true; changed = true; }
                        const name = e.name || 'Botnik 1';
                        if (t.autoDeployName !== name) { t.autoDeployName = name; changed = true; }
                        const msg = e.message || '';
                        if (t.autoDeployMessage !== msg) { t.autoDeployMessage = msg; changed = true; }
                        const kick = !!e.kick;
                        if (t.autoDeployKick !== kick) { t.autoDeployKick = kick; changed = true; }
                        const loyalty = !!e.loyalty;
                        if (t.autoDeployLoyalty !== loyalty) { t.autoDeployLoyalty = loyalty; changed = true; }
                        const aiChat = !!e.aiChat;
                        if (t.autoDeployAIChat !== aiChat) { t.autoDeployAIChat = aiChat; changed = true; }
                        const aiPersona = e.aiPersona || '';
                        if (t.autoDeployAIPersona !== aiPersona) { t.autoDeployAIPersona = aiPersona; changed = true; }
                        // Mirror server identifiers DOWN into the local entry —
                        // union, never clobber. The boot-time chat backfill writes
                        // uuid identifiers directly onto the server registry; this
                        // is the only path the client uses to learn about them
                        // (so the editor + index see them too). Manual identifiers
                        // the user added locally are preserved either way.
                        const serverIdents = Array.isArray(e.identifiers) ? e.identifiers : [];
                        if (!Array.isArray(t.identifiers)) t.identifiers = [];
                        const haveKey = new Set(t.identifiers.map(i => i.type + '' + i.value));
                        for (const sid of serverIdents) {
                            if (!sid || !sid.type || !sid.value) continue;
                            const k = sid.type + '' + sid.value;
                            if (haveKey.has(k)) continue;
                            t.identifiers.push({ type: sid.type, value: sid.value });
                            haveKey.add(k);
                            changed = true;
                        }
                    } else if (t.autoDeploy) {
                        // Pill says follow-on but the server has no entry — a
                        // pre-fix clobber or a failed upsert. Heal by re-pushing;
                        // never turn the pill off, never delete.
                        await this.syncAutoDeploy(t);
                    }
                }
                if (changed) { this.saveTracked(); this.renderTrackedBar(); }
            } catch (e) { console.warn('[Bird] auto-deploy server sync failed:', e); }
        }

        isGoogleFoto(url) {
            return url && url.includes('googleusercontent.com/');
        }

        // Rebuild tracked lookup maps (call after any trackedUsers mutation)
        _rebuildTrackedIndex() {
            this._trackedByFoto = new Map();
            this._trackedByName = new Map();
            this._trackedByUUID = new Map();
            for (const t of this.trackedUsers) {
                for (const id of t.identifiers) {
                    if (id.type === 'foto') this._trackedByFoto.set(id.value, t);
                    else if (id.type === 'name') this._trackedByName.set(id.value, t);
                    else if (id.type === 'uuid') this._trackedByUUID.set(id.value, t);
                }
            }
        }

        // Find tracked entry that matches a given player id, avatar, or name.
        // Lookup priority: id (deterministic — survives nick+avatar change) →
        // foto → name. O(1) via Maps.
        findTracked(avatar, name, id) {
            if (!this._trackedByFoto) this._rebuildTrackedIndex();
            return (id && this._trackedByUUID.get(String(id))) ||
                   (avatar && this._trackedByFoto.get(avatar)) ||
                   (name && this._trackedByName.get(name)) ||
                   null;
        }

        // Check if a specific player id, avatar, or name is tracked
        isTrackedPlayer(avatar, name, id) {
            return !!this.findTracked(avatar, name, id);
        }

        trackUser(avatar, name) {
            if (this.findTracked(avatar, name)) return;
            const ids = [];
            if (this.isGoogleFoto(avatar)) ids.push({ type: 'foto', value: avatar });
            if (name) ids.push({ type: 'name', value: name });
            if (ids.length === 0) return;
            this.trackedUsers.push({
                id: String(Date.now()) + Math.random().toString(36).slice(2, 6),
                alias: null,
                lastKnownName: name,
                lastKnownFoto: this.isGoogleFoto(avatar) ? avatar : null,
                bellOn: false,
                identifiers: ids
            });
            this.saveTracked();
            this.renderTrackedBar();
        }

        untrackUser(trackedId) {
            this.trackedUsers = this.trackedUsers.filter(t => t.id !== trackedId);
            this.saveTracked();
            fetch('/bird/api/auto-deploy/delete?id=' + encodeURIComponent(trackedId), { method: 'DELETE' }).catch(() => {});
            this.trackedOnline.delete(trackedId);
            this._lastBellAt.delete(trackedId);
            // Drop the chat author filter if it was pointing at this person.
            if (this.activeAuthorTrackedId === trackedId) this.clearAuthorFilter();
            this.renderTrackedBar();
            // Update star buttons on visible cards
            this.refreshTrackStars();
        }

        updateTrackedStatus(avatar, name, room, id) {
            const tracked = this.findTracked(avatar, name, id);
            if (!tracked) return;
            let changed = false;
            let identifierAdded = false;
            if (name && tracked.lastKnownName !== name) { tracked.lastKnownName = name; changed = true; }
            if (this.isGoogleFoto(avatar) && tracked.lastKnownFoto !== avatar) { tracked.lastKnownFoto = avatar; changed = true; }
            // Auto-add identifiers we discover (e.g. they tracked by name, now we see the foto)
            if (this.isGoogleFoto(avatar) && !tracked.identifiers.some(idn => idn.type === 'foto' && idn.value === avatar)) {
                tracked.identifiers.push({ type: 'foto', value: avatar }); changed = true; identifierAdded = true;
            }
            if (name && !tracked.identifiers.some(idn => idn.type === 'name' && idn.value === name)) {
                tracked.identifiers.push({ type: 'name', value: name }); changed = true; identifierAdded = true;
                // Keep an active chat filter in sync — if we're filtering on this
                // tracked person and they just adopted a new nickname, fold it in
                // so their future messages still match.
                if (this.activeAuthorTrackedId === tracked.id && this.activeAuthorNames) {
                    this.activeAuthorNames.add(name);
                    this._scheduleOnce('filterChat', this.filterChat);
                }
            }
            // UUID auto-merge: only when a real id is available (numeric account
            // id or guest UUID — gartic's persistent per-identity handle). Survives
            // simultaneous nick + foto change. Sanity bounds: length ∈ [5,64].
            // Lenient + log: if the record already has uuid identifiers and the new
            // uuid matches none of them, we still merge (could be a legit second
            // device for the same person) but warn so impostor activity is auditable.
            const idStr = id == null ? '' : String(id);
            if (idStr.length >= 5 && idStr.length <= 64 &&
                !tracked.identifiers.some(idn => idn.type === 'uuid' && idn.value === idStr)) {
                const existingUUIDs = tracked.identifiers.filter(idn => idn.type === 'uuid').map(idn => idn.value);
                if (existingUUIDs.length > 0) {
                    console.warn('[Bird] uuid divergence on tracked', tracked.id,
                        '— existing:', existingUUIDs, 'new:', idStr,
                        '(merging — could be new device or impostor sharing nick/avatar)');
                }
                tracked.identifiers.push({ type: 'uuid', value: idStr }); changed = true; identifierAdded = true;
            }
            if (changed) this.saveTracked();
            // Push auto-discovered identifiers to bird-server. Without this,
            // the server's Match keeps the original (smaller) identifier set
            // and misses on later rejoins under a new name — the user has to
            // open the edit panel and re-save just to make the deploy fire.
            // Gated on autoDeploy so we don't churn the server for tracked
            // people the user is only watching, not auto-deploying. The
            // upsert handler runs the rescan hook so the deploy fires for
            // anyone already in a watched room without waiting for a rejoin.
            if (identifierAdded && tracked.autoDeploy) {
                this.syncAutoDeploy(tracked);
            }
            const prev = this.trackedOnline.get(tracked.id);
            const isFirstSighting = !prev;
            const isRoomChange = prev && prev.room !== room;
            this.trackedOnline.set(tracked.id, { name, room, avatar });
            this._scheduleOnce('renderTrackedBar', this.renderTrackedBar);
            if (tracked.bellOn && (isFirstSighting || isRoomChange)) {
                this.maybeRing(tracked, name, room);
            }
        }

        removeTrackedStatus(avatar, name) {
            const tracked = this.findTracked(avatar, name);
            if (tracked && this.trackedOnline.delete(tracked.id)) {
                const id = tracked.id;
                setTimeout(() => {
                    if (!this.trackedOnline.has(id)) this._lastBellAt.delete(id);
                }, 60_000);
                this.renderTrackedBar();
            }
        }

        refreshTrackStars() {
            this.overlay.querySelectorAll('.Bird-trackBtn').forEach(btn => {
                const foto = btn.dataset.foto;
                const name = btn.dataset.trackname;
                btn.classList.toggle('tracked', this.isTrackedPlayer(foto, name));
            });
        }

        // ---- Merge ----
        startMerge(sourceId) {
            this.mergeSource = sourceId;
            this.renderTrackedBar();
        }

        cancelMerge() {
            this.mergeSource = null;
            this.renderTrackedBar();
        }

        executeMerge(targetId) {
            const src = this.trackedUsers.find(t => t.id === this.mergeSource);
            const tgt = this.trackedUsers.find(t => t.id === targetId);
            if (!src || !tgt || src === tgt) { this.cancelMerge(); return; }

            // Prompt for custom alias
            const defaultAlias = src.alias || tgt.alias || src.lastKnownName || tgt.lastKnownName;
            const alias = prompt('Name for the merged player:', defaultAlias);
            if (alias === null) { this.cancelMerge(); return; }

            // Merge identifiers (deduplicate)
            for (const id of src.identifiers) {
                if (!tgt.identifiers.some(x => x.type === id.type && x.value === id.value)) {
                    tgt.identifiers.push(id);
                }
            }
            tgt.alias = alias || tgt.alias;
            tgt.lastKnownFoto = tgt.lastKnownFoto || src.lastKnownFoto;
            tgt.lastKnownName = tgt.lastKnownName || src.lastKnownName;

            // Merge auto-deploy: any toggle ON in either side survives, and
            // any non-empty string field falls back to the source if target's
            // is blank. Without this, merging into a target with no auto-deploy
            // silently drops the source's autoDeploy + message/kick/loyalty/
            // aiChat/persona settings — the user's reported "reset" bug.
            const srcHadAD = !!src.autoDeploy;
            if (srcHadAD || tgt.autoDeploy) {
                tgt.autoDeploy = true;
                tgt.autoDeployName      = tgt.autoDeployName      || src.autoDeployName      || 'Botnik 1';
                tgt.autoDeployMessage   = tgt.autoDeployMessage   || src.autoDeployMessage   || '';
                tgt.autoDeployKick      = !!(tgt.autoDeployKick    || src.autoDeployKick);
                tgt.autoDeployLoyalty   = !!(tgt.autoDeployLoyalty || src.autoDeployLoyalty);
                tgt.autoDeployAIChat    = !!(tgt.autoDeployAIChat  || src.autoDeployAIChat);
                tgt.autoDeployAIPersona = tgt.autoDeployAIPersona || src.autoDeployAIPersona || '';
            }

            // Transfer online status
            const srcOnline = this.trackedOnline.get(src.id);
            if (srcOnline && !this.trackedOnline.has(tgt.id)) {
                this.trackedOnline.set(tgt.id, srcOnline);
            }
            this.trackedOnline.delete(src.id);

            // Remove source
            this.trackedUsers = this.trackedUsers.filter(t => t.id !== src.id);
            this._lastBellAt.delete(src.id);
            this.mergeSource = null;
            // If the chat author filter was pointing at either side of the merge,
            // re-target it at the surviving (target) entry so the filter follows
            // the merged identity rather than silently going stale.
            if (this.activeAuthorTrackedId === src.id || this.activeAuthorTrackedId === tgt.id) {
                const names = this._collectTrackedNames(tgt);
                if (names.size > 0) {
                    const label = tgt.alias || tgt.lastKnownName || 'Tracked';
                    this.setAuthorFilter(tgt.id, names, label);
                } else {
                    this.clearAuthorFilter();
                }
            }
            this.saveTracked();
            // Server-side reconcile: drop the source's now-orphaned entry so it
            // doesn't double-fire against the identifiers we just moved over,
            // and push the merged target so its identifiers + settings match
            // the local pill. syncAutoDeploy handles both upsert and delete
            // based on tgt.autoDeploy, which we just set above.
            if (srcHadAD) {
                fetch('/bird/api/auto-deploy/delete?id=' + encodeURIComponent(src.id), { method: 'DELETE' }).catch(() => {});
            }
            this.syncAutoDeploy(tgt);
            this.renderTrackedBar();
            this.refreshTrackStars();
        }

        // =================================================================
        // Bell — per-tracked alert + master mute
        // =================================================================

        toggleBell(trackedId) {
            const t = this.trackedUsers.find(x => x.id === trackedId);
            if (!t) return;
            t.bellOn = !t.bellOn;
            this.saveTracked();
            if (t.bellOn) this._ensureAudioContext(); // first user gesture unlocks audio
            this.renderTrackedBar();
        }

        setBellMuted(v) {
            this.bellMuted = !!v;
            GM_setValue("Bird-BellMuted", this.bellMuted);
            if (this.bellMuted) { this._stopBeepLoop(); this._stopTitleFlash(); this._restoreFavicon(); }
            this.renderTrackedBar();
        }

        requestNotificationPermission() {
            if (typeof Notification === 'undefined') return;
            if (Notification.permission === 'granted' || Notification.permission === 'denied') return;
            Notification.requestPermission().catch(() => {});
        }

        maybeRing(tracked, name, room) {
            if (this.bellMuted) return;
            const now = Date.now();
            const last = this._lastBellAt.get(tracked.id) || 0;
            if (now - last < 30_000) return;
            this._lastBellAt.set(tracked.id, now);

            const label = tracked.alias || name;
            this._playBeep();
            if (document.hidden) {
                this._startTitleFlash(label, room);
                this._paintFaviconDot();
                this._startBeepLoop();
            }
            if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
                try {
                    new Notification('Bird: tracked player joined', {
                        body: `${label} → ${room}`,
                        silent: true,
                        tag: tracked.id
                    });
                } catch {}
            }

            // Pulse the matching pill so the user can locate it after refocusing
            const pill = this.trackedBar.querySelector(`[data-tracked-id="${CSS.escape(tracked.id)}"]`);
            if (pill) { pill.classList.remove('bell-flash'); void pill.offsetWidth; pill.classList.add('bell-flash'); }
        }

        _ensureAudioContext() {
            if (this._audioCtx) return this._audioCtx;
            try {
                const Ctor = window.AudioContext || window.webkitAudioContext;
                this._audioCtx = new Ctor();
            } catch { this._audioCtx = null; }
            return this._audioCtx;
        }

        _playBeep() {
            const ctx = this._ensureAudioContext();
            if (!ctx) return;
            if (ctx.state === 'suspended') ctx.resume().catch(() => {});
            const now = ctx.currentTime;
            const make = (freq, start, duration, peak) => {
                const osc = ctx.createOscillator();
                const gain = ctx.createGain();
                osc.type = 'sine';
                osc.frequency.value = freq;
                gain.gain.setValueAtTime(0, now + start);
                gain.gain.linearRampToValueAtTime(peak, now + start + 0.025);
                gain.gain.exponentialRampToValueAtTime(0.0001, now + start + duration);
                osc.connect(gain).connect(ctx.destination);
                osc.start(now + start);
                osc.stop(now + start + duration + 0.02);
            };
            make(880, 0,    0.18, 0.18);
            make(660, 0.22, 0.22, 0.18);
        }

        _startBeepLoop() {
            if (this._beepLoop) return;
            this._beepLoop = setInterval(() => {
                if (this.bellMuted || !document.hidden) { this._stopBeepLoop(); return; }
                this._playBeep();
            }, 3000);
        }

        _stopBeepLoop() {
            if (this._beepLoop) { clearInterval(this._beepLoop); this._beepLoop = null; }
        }

        _startTitleFlash(name, room) {
            if (!this._origTitle) this._origTitle = document.title;
            this._flashLabel = `🔔 ${name} → ${room} • Bird`;
            if (this._flashTimer) return;
            this._flashTimer = setInterval(() => {
                document.title = (document.title === this._flashLabel) ? this._origTitle : this._flashLabel;
            }, 1100);
        }

        _stopTitleFlash() {
            if (this._flashTimer) { clearInterval(this._flashTimer); this._flashTimer = null; }
            if (this._origTitle) { document.title = this._origTitle; this._origTitle = null; }
        }

        _paintFaviconDot() {
            if (this._faviconActive) return;
            let link = document.querySelector('link[rel~="icon"]');
            if (!link) {
                link = Object.assign(document.createElement('link'), { rel: 'icon' });
                document.head.appendChild(link);
            }
            if (this._origFavicon == null) this._origFavicon = link.getAttribute('href') || '';
            const draw = (baseImg) => {
                const c = document.createElement('canvas');
                c.width = 32; c.height = 32;
                const g = c.getContext('2d');
                if (baseImg) { try { g.drawImage(baseImg, 0, 0, 32, 32); } catch {} }
                g.beginPath(); g.arc(24, 24, 8, 0, Math.PI * 2);
                g.fillStyle = '#ef4444'; g.fill();
                g.lineWidth = 2; g.strokeStyle = '#fff'; g.stroke();
                try { link.href = c.toDataURL('image/png'); } catch {}
                this._faviconActive = true;
            };
            if (this._origFavicon) {
                const img = new Image();
                img.crossOrigin = 'anonymous';
                img.onload = () => draw(img);
                img.onerror = () => draw(null);
                img.src = this._origFavicon;
            } else { draw(null); }
        }

        _restoreFavicon() {
            if (!this._faviconActive) return;
            const link = document.querySelector('link[rel~="icon"]');
            if (link && this._origFavicon != null) link.href = this._origFavicon;
            this._faviconActive = false;
            this._origFavicon = null;
        }

        // ---- Edit Panel ----
        openEditPanel(trackedId) {
            const t = this.trackedUsers.find(x => x.id === trackedId);
            if (!t) return;
            this.overlay.querySelector('.Bird-editOverlay')?.remove();

            const ov = document.createElement('div');
            ov.className = 'Bird-editOverlay';

            const panel = document.createElement('div');
            panel.className = 'Bird-editPanel';

            // Header
            const h3 = document.createElement('h3');
            if (t.lastKnownFoto) {
                const img = Object.assign(document.createElement('img'), { src: t.lastKnownFoto });
                h3.appendChild(img);
            } else {
                const noAvatar = Object.assign(document.createElement('span'), {
                    className: 'Bird-editNoAvatar', textContent: '?'
                });
                h3.appendChild(noAvatar);
            }
            h3.appendChild(document.createTextNode(' Edit Tracked Player'));
            panel.appendChild(h3);

            // Alias field
            const aliasField = document.createElement('div');
            aliasField.className = 'Bird-editField';
            const aliasLabel = Object.assign(document.createElement('label'), { textContent: 'Custom Alias' });
            aliasLabel.setAttribute('for', 'plEditAlias');
            const aliasInput = Object.assign(document.createElement('input'), {
                type: 'text', id: 'plEditAlias',
                placeholder: t.lastKnownName || 'Enter alias...',
                value: t.alias || ''
            });
            aliasField.append(aliasLabel, aliasInput);
            panel.appendChild(aliasField);

            // Identifiers
            const idsWrap = document.createElement('div');
            idsWrap.className = 'Bird-editIds';
            const idsLabel = Object.assign(document.createElement('label'), {
                textContent: `Identifiers (${t.identifiers.length})`
            });
            idsWrap.appendChild(idsLabel);

            t.identifiers.forEach((id, i) => {
                const row = document.createElement('div');
                row.className = 'Bird-editId';
                row.dataset.idx = i;

                const typeIcon = id.type === 'foto' ? '\uD83D\uDCF7' :
                                 id.type === 'name' ? '\uD83D\uDC64' :
                                 '\uD83C\uDD94'; // uuid \u2192 \uD83C\uDD94
                const typeSpan = Object.assign(document.createElement('span'), {
                    className: 'type',
                    textContent: `${typeIcon} ${id.type}`
                });
                const displayVal = id.type === 'foto'
                    ? id.value.split('/').pop()
                    : id.type === 'uuid' && id.value.length > 12
                        ? id.value.slice(0, 8) + '\u2026'
                        : id.value;
                const valueSpan = Object.assign(document.createElement('span'), {
                    className: 'value',
                    textContent: displayVal,
                    title: id.value
                });
                row.append(typeSpan, valueSpan);

                if (t.identifiers.length > 1) {
                    const delBtn = Object.assign(document.createElement('button'), {
                        className: 'delId',
                        textContent: '\u00d7',
                        title: 'Remove this identifier',
                        type: 'button'
                    });
                    delBtn.setAttribute('aria-label', `Remove identifier: ${id.value}`);
                    delBtn.addEventListener('click', () => {
                        t.identifiers.splice(i, 1);
                        this.saveTracked();
                        ov.remove();
                        this.openEditPanel(trackedId);
                    });
                    row.appendChild(delBtn);
                }
                idsWrap.appendChild(row);
            });
            panel.appendChild(idsWrap);

            // Add identifier field
            const addField = document.createElement('div');
            addField.className = 'Bird-editField';
            const addLabel = Object.assign(document.createElement('label'), { textContent: 'Add Identifier' });
            const addRow = Object.assign(document.createElement('div'), { style: 'display:flex;gap:6px' });
            const addInput = Object.assign(document.createElement('input'), {
                type: 'text', id: 'plEditNewId',
                placeholder: 'Username or Google foto URL...',
                style: 'flex:1'
            });
            const addBtn = Object.assign(document.createElement('button'), {
                id: 'plEditAddId', type: 'button', textContent: '+ Add'
            });
            addBtn.addEventListener('click', () => {
                const val = addInput.value.trim();
                if (!val) return;
                let type;
                if (val.includes('googleusercontent.com/')) {
                    type = 'foto';
                } else if (/^[0-9a-f]{8,}(-[0-9a-f]+){0,4}$/i.test(val) || /^\d{5,}$/.test(val)) {
                    // Hex UUID (with optional dashes) or pure-digit account id.
                    type = 'uuid';
                } else {
                    type = 'name';
                }
                if (t.identifiers.some(x => x.type === type && x.value === val)) return;
                t.identifiers.push({ type, value: val });
                this.saveTracked();
                ov.remove();
                this.openEditPanel(trackedId);
            });
            addRow.append(addInput, addBtn);
            addField.append(addLabel, addRow);
            panel.appendChild(addField);

            // Auto-deploy section
            const adField = document.createElement('div');
            adField.className = 'Bird-editField';
            const adLabel = document.createElement('label');
            adLabel.textContent = 'Auto-deploy';

            const adToggleRow = document.createElement('div');
            adToggleRow.style.cssText = 'display:flex;align-items:center;gap:8px;margin-bottom:6px';
            const adCheckbox = Object.assign(document.createElement('input'), {
                type: 'checkbox',
                id: 'plEditAutoDeploy',
                checked: !!t.autoDeploy,
            });
            const adCheckLabel = Object.assign(document.createElement('label'), {
                htmlFor: 'plEditAutoDeploy',
                textContent: 'Auto-deploy bots when seen in a room',
                style: 'cursor:pointer;user-select:none',
            });
            adToggleRow.append(adCheckbox, adCheckLabel);

            const adNameRow = document.createElement('div');
            adNameRow.style.cssText = 'display:flex;gap:6px;align-items:center';
            const adNameLabel = Object.assign(document.createElement('label'), {
                textContent: 'Bot name:',
                htmlFor: 'plEditAutoDeployName',
                style: 'font-size:var(--pl-font-size-xs)',
            });
            const adNameInput = Object.assign(document.createElement('input'), {
                type: 'text',
                id: 'plEditAutoDeployName',
                placeholder: 'Botnik 1',
                value: t.autoDeployName || 'Botnik 1',
                style: 'flex:1',
            });
            adNameRow.append(adNameLabel, adNameInput);

            const adMsgRow = document.createElement('div');
            adMsgRow.style.cssText = 'display:flex;gap:6px;align-items:center;margin-top:6px';
            const adMsgLabel = Object.assign(document.createElement('label'), {
                textContent: 'Message:',
                htmlFor: 'plEditAutoDeployMessage',
                style: 'font-size:var(--pl-font-size-xs)',
            });
            const adMsgInput = Object.assign(document.createElement('input'), {
                type: 'text',
                id: 'plEditAutoDeployMessage',
                placeholder: 'Message bots repeat (lobby + answers)',
                value: t.autoDeployMessage || '',
                style: 'flex:1',
            });
            adMsgRow.append(adMsgLabel, adMsgInput);

            const adKickRow = document.createElement('div');
            adKickRow.style.cssText = 'display:flex;align-items:center;gap:8px;margin-top:6px';
            const adKickCheckbox = Object.assign(document.createElement('input'), {
                type: 'checkbox',
                id: 'plEditAutoDeployKick',
                checked: !!t.autoDeployKick,
            });
            const adKickLabel = Object.assign(document.createElement('label'), {
                htmlFor: 'plEditAutoDeployKick',
                textContent: 'Kick this player every 5 s',
                style: 'cursor:pointer;user-select:none',
            });
            adKickRow.append(adKickCheckbox, adKickLabel);

            const adLoyaltyRow = document.createElement('div');
            adLoyaltyRow.style.cssText = 'display:flex;align-items:center;gap:8px;margin-top:6px';
            const adLoyaltyCheckbox = Object.assign(document.createElement('input'), {
                type: 'checkbox',
                id: 'plEditAutoDeployLoyalty',
                checked: !!t.autoDeployLoyalty,
            });
            const adLoyaltyLabel = Object.assign(document.createElement('label'), {
                htmlFor: 'plEditAutoDeployLoyalty',
                textContent: "Loyalty — bots mirror this player's chat & kicks",
                style: 'cursor:pointer;user-select:none',
            });
            adLoyaltyRow.append(adLoyaltyCheckbox, adLoyaltyLabel);

            const adAIChatRow = document.createElement('div');
            adAIChatRow.style.cssText = 'display:flex;align-items:center;gap:8px;margin-top:6px';
            const adAIChatCheckbox = Object.assign(document.createElement('input'), {
                type: 'checkbox',
                id: 'plEditAutoDeployAIChat',
                checked: !!t.autoDeployAIChat,
            });
            const adAIChatLabel = Object.assign(document.createElement('label'), {
                htmlFor: 'plEditAutoDeployAIChat',
                textContent: 'AI chat — bots reply with AI + vote-kick whoever this player kicks',
                style: 'cursor:pointer;user-select:none',
            });
            adAIChatRow.append(adAIChatCheckbox, adAIChatLabel);

            const adAIPersonaRow = document.createElement('div');
            adAIPersonaRow.style.cssText = 'display:flex;align-items:center;gap:6px;margin-top:6px';
            const adAIPersonaLabel = Object.assign(document.createElement('label'), {
                textContent: 'Persona:',
                htmlFor: 'plEditAutoDeployAIPersona',
                style: 'font-size:var(--pl-font-size-xs)',
            });
            const adAIPersonaInput = Object.assign(document.createElement('input'), {
                type: 'text',
                id: 'plEditAutoDeployAIPersona',
                placeholder: 'persona, e.g. "a sarcastic friend" (optional)',
                value: t.autoDeployAIPersona || '',
                style: 'flex:1',
            });
            adAIPersonaRow.append(adAIPersonaLabel, adAIPersonaInput);

            adField.append(adLabel, adToggleRow, adNameRow, adMsgRow, adKickRow, adLoyaltyRow, adAIChatRow, adAIPersonaRow);
            panel.appendChild(adField);

            // Show-this-person's-chat — filters the chat panel to messages whose
            // author matches any nickname we've seen this tracked person use.
            // Toggles off when already active for the same tracked id.
            const chatFilterField = document.createElement('div');
            chatFilterField.className = 'Bird-editField';
            const chatFilterBtn = Object.assign(document.createElement('button'), {
                type: 'button',
                style: 'width:100%;padding:8px 0;border:none;border-radius:var(--pl-radius-sm);font-weight:700;font-size:var(--pl-font-size-sm);cursor:pointer;font-family:var(--pl-font);transition:all var(--pl-transition-fast)',
            });
            const isActiveHere = this.activeAuthorTrackedId === trackedId;
            chatFilterBtn.textContent = isActiveHere
                ? "Hide this person's chat"
                : "Show this person's chat";
            chatFilterBtn.style.background = isActiveHere
                ? 'var(--pl-accent-warning)'
                : 'var(--pl-accent-tracked)';
            chatFilterBtn.style.color = 'var(--pl-text-inverse)';
            chatFilterBtn.addEventListener('click', () => {
                if (this.activeAuthorTrackedId === trackedId) {
                    this.clearAuthorFilter();
                    ov.remove();
                    return;
                }
                const names = this._collectTrackedNames(t);
                if (names.size === 0) {
                    const warn = Object.assign(document.createElement('div'), {
                        className: 'Bird-mergeHint',
                        textContent: "No nicknames recorded for this person yet — can't filter chat.",
                    });
                    chatFilterField.appendChild(warn);
                    setTimeout(() => warn.remove(), 3000);
                    return;
                }
                const label = t.alias || t.lastKnownName || 'Tracked';
                this.setAuthorFilter(trackedId, names, label);
                // Switch to the chat tab on mobile (no-op on desktop where both panels show).
                if (this.tabChat) this.tabChat.click();
                ov.remove();
            });
            chatFilterField.appendChild(chatFilterBtn);
            panel.appendChild(chatFilterField);

            // Action buttons
            const actions = document.createElement('div');
            actions.className = 'Bird-editActions';
            const saveBtn = Object.assign(document.createElement('button'), { className: 'save', textContent: 'Save', type: 'button' });
            const mergeBtn = Object.assign(document.createElement('button'), { className: 'merge', textContent: 'Merge', type: 'button' });
            const deleteBtn = Object.assign(document.createElement('button'), { className: 'delete', textContent: 'Delete', type: 'button' });
            const cancelBtn = Object.assign(document.createElement('button'), { className: 'cancel', textContent: 'Cancel', type: 'button' });

            saveBtn.addEventListener('click', () => {
                t.alias = aliasInput.value.trim() || null;
                t.autoDeploy = !!adCheckbox.checked;
                t.autoDeployName = adNameInput.value.trim() || 'Botnik 1';
                t.autoDeployMessage = adMsgInput.value.trim();
                t.autoDeployKick = !!adKickCheckbox.checked;
                t.autoDeployLoyalty = !!adLoyaltyCheckbox.checked;
                t.autoDeployAIChat = !!adAIChatCheckbox.checked;
                t.autoDeployAIPersona = adAIPersonaInput.value.trim();
                this.saveTracked();
                this.syncAutoDeploy(t);
                this.renderTrackedBar();
                this.refreshTrackStars();
                ov.remove();
            });
            mergeBtn.addEventListener('click', () => {
                if (this.trackedUsers.length < 2) {
                    // Inline warning instead of alert()
                    const warn = Object.assign(document.createElement('div'), {
                        className: 'Bird-mergeHint',
                        textContent: 'Need at least 2 tracked players to merge.'
                    });
                    actions.before(warn);
                    setTimeout(() => warn.remove(), 3000);
                    return;
                }
                this.startMerge(trackedId); ov.remove();
            });
            deleteBtn.addEventListener('click', () => { this.untrackUser(trackedId); ov.remove(); });
            cancelBtn.addEventListener('click', () => ov.remove());

            actions.append(saveBtn, mergeBtn, deleteBtn, cancelBtn);
            panel.appendChild(actions);

            ov.appendChild(panel);
            ov.addEventListener('click', e => { if (e.target === ov) ov.remove(); });
            this.overlay.appendChild(ov);
            aliasInput.focus();
        }

        // ---- Render ----
        renderTrackedBar() {
            const isMerging = this.mergeSource !== null;
            this.trackedBar.innerHTML = '';

            if (isMerging) {
                const hint = document.createElement('div');
                hint.className = 'Bird-mergeHint';
                hint.textContent = 'Click another player to merge — or click here to cancel';
                hint.addEventListener('click', () => this.cancelMerge());
                this.trackedBar.appendChild(hint);
            } else {
                const label = document.createElement('span');
                label.className = 'Bird-trackedLabel';
                label.textContent = 'TRACKED';
                this.trackedBar.appendChild(label);

                const mute = document.createElement('span');
                mute.className = `Bird-masterMute${this.bellMuted ? ' muted' : ''}`;
                mute.textContent = this.bellMuted ? '🔇' : '🔊';
                mute.title = this.bellMuted ? 'All bells muted — click to unmute' : 'Mute all bells';
                mute.setAttribute('role', 'button');
                mute.setAttribute('aria-label', this.bellMuted ? 'Unmute all bells' : 'Mute all bells');
                mute.addEventListener('click', () => this.setBellMuted(!this.bellMuted));
                this.trackedBar.appendChild(mute);

                // Master auto-deploy gate. Per-user checkboxes stay set; this
                // toggle is the higher-level switch that blocks every deploy/
                // active-loop/recovery refire when off, and resumes them via
                // a server-side rescan when flipped back on.
                const adMaster = document.createElement('span');
                const adOn = !!this._masterAutoDeploy;
                adMaster.className = `Bird-masterAD${adOn ? '' : ' off'}`;
                adMaster.textContent = adOn ? 'AUTO-DEPLOY: ON' : 'AUTO-DEPLOY: OFF';
                adMaster.title = adOn
                    ? 'Global auto-deploy is ON — click to disable for everyone (per-user settings stay saved)'
                    : 'Global auto-deploy is OFF — per-user settings stay saved but never fire; click to re-enable';
                adMaster.setAttribute('role', 'button');
                adMaster.setAttribute('aria-pressed', adOn ? 'true' : 'false');
                adMaster.setAttribute('aria-label', adOn ? 'Disable global auto-deploy' : 'Enable global auto-deploy');
                adMaster.addEventListener('click', () => this.setMasterAutoDeploy(!this._masterAutoDeploy));
                this.trackedBar.appendChild(adMaster);

                const immunePill = document.createElement('span');
                const immuneCount = this._immuneRooms.size;
                immunePill.className = `Bird-immunePill${immuneCount > 0 ? ' has' : ''}`;
                immunePill.textContent = immuneCount > 0
                    ? `🛡️ IMMUNE: ${immuneCount}`
                    : '🛡️ IMMUNE: 0';
                immunePill.title = 'Manage rooms that should never receive auto-deploy bots';
                immunePill.setAttribute('role', 'button');
                immunePill.setAttribute('aria-label', 'Manage auto-deploy immune rooms');
                immunePill.addEventListener('click', () => this.openImmuneRoomsPanel());
                this.trackedBar.appendChild(immunePill);
            }

            if (this.trackedUsers.length === 0) return;

            for (const t of this.trackedUsers) {
                const online = this.trackedOnline.get(t.id);
                const displayName = t.alias || online?.name || t.lastKnownName || '???';
                const el = document.createElement('div');
                el.dataset.trackedId = t.id;
                const isMergeSrc = isMerging && t.id === this.mergeSource;
                el.className = `Bird-trackedUser ${online ? 'online' : 'offline'}${isMerging && !isMergeSrc ? ' merge-target' : ''}`;
                if (isMergeSrc) el.style.opacity = '0.5';

                // Buttons container
                const btnsDiv = document.createElement('div');
                btnsDiv.className = 'Bird-trackedBtns';

                const bellBtn = Object.assign(document.createElement('button'), {
                    className: `Bird-trackedBtn bell${t.bellOn ? ' on' : ''}`,
                    type: 'button',
                    textContent: t.bellOn ? '\ud83d\udd14' : '\ud83d\udd15'
                });
                bellBtn.title = t.bellOn
                    ? 'Ringing on join \u2014 click to silence (Shift-click for system notifications)'
                    : 'Ring when this player joins (Shift-click for system notifications)';
                bellBtn.setAttribute('aria-label', `${t.bellOn ? 'Disable' : 'Enable'} ring for ${displayName}`);

                const editBtn = Object.assign(document.createElement('button'), {
                    className: 'Bird-trackedBtn edit', type: 'button', textContent: '\u270E'
                });
                editBtn.setAttribute('aria-label', `Edit tracked player ${displayName}`);
                editBtn.title = 'Edit';

                const removeBtn = Object.assign(document.createElement('button'), {
                    className: 'Bird-trackedBtn remove', type: 'button', textContent: '\u00d7'
                });
                removeBtn.setAttribute('aria-label', `Remove tracked player ${displayName}`);
                removeBtn.title = 'Remove';

                btnsDiv.append(bellBtn, editBtn, removeBtn);
                el.appendChild(btnsDiv);

                if (t.autoDeploy) {
                    const rocket = document.createElement('span');
                    rocket.className = 'Bird-trackedAutoDeploy';
                    rocket.textContent = '🚀';
                    rocket.title = 'Auto-deploy on';
                    rocket.style.cssText = 'position:absolute;top:-4px;left:-4px;font-size:12px;line-height:1;pointer-events:none';
                    el.appendChild(rocket);
                }

                // Avatar
                if (t.lastKnownFoto) {
                    const img = Object.assign(document.createElement('img'), {
                        className: 'Bird-trackedAvatar', src: t.lastKnownFoto
                    });
                    el.appendChild(img);
                } else {
                    const noAvatar = Object.assign(document.createElement('div'), {
                        className: 'Bird-trackedNoAvatar', textContent: '?'
                    });
                    el.appendChild(noAvatar);
                }

                // Name
                const hasFoto = t.identifiers.some(id => id.type === 'foto');
                const hasName = t.identifiers.some(id => id.type === 'name');
                const hasUUID = t.identifiers.some(id => id.type === 'uuid');
                let idIcons = '';
                if (hasFoto) idIcons += '\uD83D\uDCF7'; // \uD83D\uDCF7
                if (hasName) idIcons += '\uD83D\uDC64'; // \uD83D\uDC64
                if (hasUUID) idIcons += '\uD83C\uDD94'; // \uD83C\uDD94
                if (!idIcons) idIcons = '\uD83D\uDC64'; // fallback person icon
                const nameSpan = Object.assign(document.createElement('span'), {
                    className: 'Bird-trackedName', textContent: displayName
                });
                nameSpan.title = `${displayName} ${idIcons}`;
                el.appendChild(nameSpan);

                // Online room indicator
                if (online) {
                    const roomSpan = Object.assign(document.createElement('span'), {
                        className: 'Bird-trackedRoom', textContent: online.room
                    });
                    el.appendChild(roomSpan);
                }

                // Event listeners
                if (isMerging && !isMergeSrc) {
                    el.addEventListener('click', () => this.executeMerge(t.id));
                } else if (!isMerging) {
                    if (online) {
                        el.addEventListener('click', () => {
                            const roomDiv = document.getElementById(`room-group-${online.room}`);
                            if (!roomDiv) return;

                            const container = this.playersContainer;
                            const allRooms = container.querySelectorAll('.Bird-room-group');

                            // Force ALL rooms to render so layout positions are real, not estimated from contain-intrinsic-size placeholders
                            allRooms.forEach(r => r.style.contentVisibility = 'visible');
                            void container.offsetHeight; // force synchronous reflow

                            // Relative scroll: getBoundingClientRect gives where the room IS right now on screen,
                            // then scrollBy moves by the delta — works regardless of current scroll position
                            const containerRect = container.getBoundingClientRect();
                            const roomRect = roomDiv.getBoundingClientRect();
                            const delta = (roomRect.top + roomRect.height / 2) - (containerRect.top + containerRect.height / 2);
                            container.scrollBy({ top: delta, behavior: 'smooth' });

                            // Dim all other rooms via inline styles (!important) — CSS classes get ignored by content-visibility skip-rendering
                            allRooms.forEach(r => {
                                if (r !== roomDiv) {
                                    r.style.setProperty('opacity', '0.3', 'important');
                                }
                            });

                            // Glow on target
                            roomDiv.classList.remove('Bird-room-glow');
                            void roomDiv.offsetWidth;
                            roomDiv.classList.add('Bird-room-glow');
                            roomDiv.addEventListener('animationend', () => roomDiv.classList.remove('Bird-room-glow'), { once: true });

                            // Restore after scroll + visual highlight settle
                            setTimeout(() => {
                                allRooms.forEach(r => {
                                    // Fade back smoothly — the base transition (opacity 0.4s) on .Bird-room-group handles the animation
                                    r.style.contentVisibility = '';
                                    if (r !== roomDiv) r.style.removeProperty('opacity');
                                });
                            }, 2000);
                        });
                    }
                    removeBtn.addEventListener('click', (e) => {
                        e.stopPropagation();
                        this.untrackUser(t.id);
                    });
                    editBtn.addEventListener('click', (e) => {
                        e.stopPropagation();
                        this.openEditPanel(t.id);
                    });
                    bellBtn.addEventListener('click', (e) => {
                        e.stopPropagation();
                        if (e.shiftKey) { this.requestNotificationPermission(); return; }
                        this.toggleBell(t.id);
                    });
                }

                this.trackedBar.appendChild(el);
            }
        }

        // =================================================================
        // DIFF-AWARE addPlayer
        // =================================================================

        // addPlayer renders (or refreshes) one player's card.
        //
        // id (optional, but always present on the server-session path): when supplied,
        // the DOM card is keyed by data-player-id, so two players with identical nicks
        // each get their own card. Without an id we fall back to name-keying for the
        // legacy local-WS / public-room paths. The reason this matters: gartic bot
        // rooms routinely have 5–6 players sharing one nick — name-only keying caused
        // them to share a single DOM card, then a single player_left removed everyone
        // visually while the data store still tracked the remaining IDs.
        addPlayer(avatar, name, room, id) {
            this.hideEmptyState();
            // Clear loading indicator on first real data
            if (this.loadingIndicator) {
                this.loadingIndicator.remove();
                this.loadingIndicator = null;
            }

            let roomDiv = document.getElementById(`room-group-${room}`);
            const isNewRoom = !roomDiv;

            // --- Room creation / reuse ---
            if (!roomDiv) {
                roomDiv = document.createElement("div");
                roomDiv.className = "Bird-room-group";
                roomDiv.id = `room-group-${room}`;
                roomDiv.setAttribute('data-room-code', room);
                const headerDiv = document.createElement('div');
                headerDiv.className = 'Bird-room-header';

                const roomLabel = Object.assign(document.createElement('span'), {
                    className: 'Bird-room-label',
                    textContent: `Room: ${room}`
                });

                const copyBtn = Object.assign(document.createElement('button'), {
                    className: 'Bird-headerBtn Bird-copyBtn',
                    type: 'button',
                    textContent: '\uD83D\uDCCB',
                    title: 'Copy Viewer Link'
                });
                copyBtn.setAttribute('aria-label', `Copy viewer link for room ${room}`);

                const filterBtn = Object.assign(document.createElement('button'), {
                    className: 'Bird-headerBtn Bird-filterRoomBtn',
                    type: 'button',
                    textContent: '\uD83D\uDCAC',
                    title: 'Filter chat to this room'
                });
                filterBtn.setAttribute('aria-label', `Filter chat to room ${room}`);

                // Auto-deploy immunity shield. Yellow + outline when on. Clicking
                // toggles the server-side flag, which (a) blocks future deploys
                // and (b) stops any in-flight per-room loops for this room.
                const shieldBtn = document.createElement('span');
                const isImmune = this._immuneRooms.has(room);
                shieldBtn.className = `Bird-immuneShield${isImmune ? ' on' : ''}`;
                shieldBtn.textContent = '🛡️';
                shieldBtn.setAttribute('role', 'button');
                shieldBtn.setAttribute('aria-pressed', isImmune ? 'true' : 'false');
                shieldBtn.setAttribute('aria-label', `Toggle auto-deploy immunity for room ${room}`);
                shieldBtn.title = isImmune
                    ? 'Room is immune to auto-deploy — click to allow bots again'
                    : 'Make room immune to auto-deploy (no bots, no loops, no recovery refire)';
                shieldBtn.addEventListener('click', (e) => {
                    e.stopPropagation();
                    this.setRoomImmune(room, !this._immuneRooms.has(room));
                });
                if (isImmune) roomDiv.classList.add('immune');

                headerDiv.append(roomLabel, copyBtn, filterBtn, shieldBtn);

                const fvBtn = document.createElement('button');
                fvBtn.textContent = '⚡ Fast View';
                fvBtn.title = `Open ${room} in Fast Viewer (new tab)`;
                fvBtn.setAttribute('aria-label', `Fast View for room ${room}`);
                fvBtn.style.cssText = 'margin-left:6px;padding:3px 8px;border:none;border-radius:6px;' +
                    'background:linear-gradient(135deg,#F59E0B,#FBBF24);color:#0F172A;font-size:11px;' +
                    'font-weight:600;cursor:pointer;';
                fvBtn.addEventListener('click', e => {
                    e.stopPropagation();
                    // Pre-seed FastView via localStorage with what Bird already
                    // knows about this room — players from playerStores + the
                    // last ~50 chat lines from _chatHistory — so the new tab
                    // can render instantly instead of blocking on its own WS
                    // connect + Event 5 round-trip.
                    try {
                        const store = this.playerStores?.[room];
                        const players = store ? [...store.map.entries()].map(([id, p]) => ({
                            id: Number(id), nick: p.name, foto: p.foto,
                            avatar: p.avatar, pts: 0
                        })) : [];
                        // Capture all chat for this room. Cap at 5000 messages OR 4 MB JSON
                        // so a long-running session never blows the localStorage quota.
                        const MAX_CHAT_COUNT = 5000;
                        const MAX_PAYLOAD_BYTES = 4 * 1024 * 1024;
                        const all = (this._chatHistory || [])
                            .filter(m => m.room === room)
                            .map(m => ({
                                authorId: 0, nick: m.author, text: m.text,
                                isSystem: false, ts: Date.now()
                            }));
                        let chat = all.length > MAX_CHAT_COUNT ? all.slice(-MAX_CHAT_COUNT) : all;
                        let payload = JSON.stringify({ players, chat, ts: Date.now() });
                        // If still too big, halve the chat slice until it fits.
                        while (payload.length > MAX_PAYLOAD_BYTES && chat.length > 100) {
                            chat = chat.slice(Math.floor(chat.length / 2));
                            payload = JSON.stringify({ players, chat, ts: Date.now() });
                        }
                        try {
                            localStorage.setItem(`bird-fv-snapshot:${room}`, payload);
                        } catch (err) {
                            // QuotaExceeded — fall back to last 50 lines (the previous behavior).
                            const fallback = all.slice(-50);
                            try {
                                localStorage.setItem(`bird-fv-snapshot:${room}`,
                                    JSON.stringify({ players, chat: fallback, ts: Date.now() }));
                            } catch (_) { /* nothing more we can do */ }
                        }
                    } catch (err) { /* localStorage full or disabled — ignore */ }
                    window.open(`https://gartic.io/live#fv=${room}`, '_blank', 'noopener,noreferrer');
                });
                headerDiv.appendChild(fvBtn);

                const roomListDiv = document.createElement('div');
                roomListDiv.className = 'Bird-room-list';

                roomDiv.append(headerDiv, roomListDiv);

                // During refresh → new rooms go to top with green accent
                if (this.isRefreshing) {
                    roomDiv.classList.add('Bird-room-new');
                    const badge = document.createElement('span');
                    badge.className = 'Bird-newBadge';
                    badge.textContent = 'NEW';
                    roomDiv.querySelector('.Bird-room-header').appendChild(badge);
                    this.playersContainer.prepend(roomDiv);

                    // Fade accent after 10s
                    setTimeout(() => {
                        roomDiv.classList.remove('Bird-room-new');
                        badge.classList.add('fading');
                        setTimeout(() => badge.remove(), 1600);
                    }, 10000);
                } else {
                    this.playersContainer.appendChild(roomDiv);
                }

                // Bind room buttons
                copyBtn.addEventListener("click", function() {
                    navigator.clipboard.writeText(`https://gartic.io/${room}/viewer`);
                    const orig = this.textContent;
                    this.textContent = "\u2705";
                    setTimeout(() => this.textContent = orig, 1000);
                });
                filterBtn.addEventListener("click", () => {
                    if (this.activeRoomFilters.has(room)) {
                        this.activeRoomFilters.delete(room);
                        filterBtn.classList.remove('active');
                    } else {
                        this.activeRoomFilters.add(room);
                        filterBtn.classList.add('active');
                    }
                    this._updateChatTitle();
                    this.filterChat();
                });
            } else {
                // Room already exists — it survived! Un-stale it.
                roomDiv.setAttribute('data-stale', '0');
                roomDiv.classList.remove('Bird-room-leaving');
            }

            // --- Player card diff within room ---
            const roomList = roomDiv.querySelector('.Bird-room-list');

            // Use CSS.escape for names with special chars
            let safeSelector;
            try { safeSelector = CSS.escape(name); } catch(e) { safeSelector = name.replace(/[^\w-]/g, '\\$&'); }

            // Prefer id-keyed lookup when caller supplied one (server-session,
            // local-WS event 5/23, public-room SSE) — name-only would collide
            // on identical-nick players. Name fallback preserves any older path
            // that doesn't track IDs.
            let existingCard = null;
            if (id != null && id !== '') {
                const safeId = CSS.escape(String(id));
                existingCard = roomList.querySelector(`.Bird-card[data-player-id="${safeId}"]`);
            }
            if (!existingCard) {
                existingCard = roomList.querySelector(`.Bird-card[data-player-name="${safeSelector}"]:not([data-player-id])`);
            }

            if (existingCard) {
                // Player confirmed — un-stale, update avatar/name if they changed.
                existingCard.setAttribute('data-stale', '0');
                existingCard.classList.remove('Bird-card-leaving');
                const img = existingCard.querySelector('.Bird-avatar');
                if (img && img.src !== avatar) img.src = avatar;
                // Refresh name attr + visible label in case the same id changed nick.
                if (existingCard.getAttribute('data-player-name') !== name) {
                    existingCard.setAttribute('data-player-name', name);
                    const nameEl = existingCard.querySelector('.Bird-name');
                    if (nameEl) nameEl.textContent = name;
                }
                if (existingCard.getAttribute('data-player-foto') !== avatar) {
                    existingCard.setAttribute('data-player-foto', avatar);
                }
            } else {
                // Brand new player — create with entrance animation
                const card = Object.assign(document.createElement("div"), { className: "Bird-card" });
                card.setAttribute('data-player-name', name);
                card.setAttribute('data-player-foto', avatar);
                if (id != null && id !== '') card.setAttribute('data-player-id', String(id));

                if (this.isRefreshing || !isNewRoom) {
                    card.classList.add('Bird-card-entering');
                    card.addEventListener('animationend', () => card.classList.remove('Bird-card-entering'), { once: true });
                }

                const avatarImg = Object.assign(document.createElement("img"), { className: "Bird-avatar", src: avatar });
                avatarImg.addEventListener('click', (e) => { e.stopPropagation(); window.open(avatar, '_blank'); });
                card.append(
                    avatarImg,
                    Object.assign(document.createElement("div"), { className: "Bird-name", textContent: name })
                );

                // Track star — works for all players (by foto and/or name)
                const star = document.createElement('button');
                star.type = 'button';
                star.className = `Bird-trackBtn${this.isTrackedPlayer(avatar, name) ? ' tracked' : ''}`;
                star.dataset.foto = avatar || '';
                star.dataset.trackname = name || '';
                star.textContent = '⭐';
                star.title = 'Track this user';
                star.setAttribute('aria-label', `Track player ${name}`);
                star.addEventListener('click', (e) => {
                    e.stopPropagation();
                    const existing = this.findTracked(avatar, name);
                    if (existing) {
                        this.untrackUser(existing.id);
                        star.classList.remove('tracked');
                    } else {
                        this.trackUser(avatar, name);
                        // Seed trackedOnline immediately so the new pill renders
                        // as "in <room>" instead of absent. Without this the
                        // pill only flips to online on the next render pass
                        // that touches this player — i.e. after a refresh.
                        this.updateTrackedStatus(avatar, name, room, id);
                        star.classList.add('tracked');
                    }
                });
                card.appendChild(star);

                roomList.appendChild(card);
            }

            // Update tracked user status
            this.updateTrackedStatus(avatar, name, room, id);

            this._scheduleOnce('filterPlayers', this.filterPlayers);
        }

        /**
         * removePlayer — gracefully removes a single player from a room.
         * Used both for real-time "player left" events and stale cleanup.
         *
         * id (optional): when supplied, the card is located by data-player-id so
         * identical-nick players each get removed individually. Without id we
         * fall back to name-keyed lookup for older paths. The name argument is
         * still consulted for removeTrackedStatus; for id-only callers we read
         * it back off the card after the lookup succeeds.
         */
        removePlayer(name, roomCode, id) {
            const roomDiv = document.getElementById(`room-group-${roomCode}`);
            if (!roomDiv) return;
            const roomList = roomDiv.querySelector('.Bird-room-list');

            let card = null;
            if (id != null && id !== '') {
                const safeId = CSS.escape(String(id));
                card = roomList.querySelector(`.Bird-card[data-player-id="${safeId}"]`);
            }
            if (!card && name) {
                let safeSelector;
                try { safeSelector = CSS.escape(name); } catch(e) { safeSelector = name.replace(/[^\w-]/g, '\\$&'); }
                card = roomList.querySelector(`.Bird-card[data-player-name="${safeSelector}"]:not([data-player-id])`);
            }
            if (!card) return;

            // Clear tracked status if this player was tracked
            const foto = card.getAttribute('data-player-foto');
            const cardName = name || card.getAttribute('data-player-name');
            this.removeTrackedStatus(foto, cardName);

            card.classList.add('Bird-card-leaving');
            card.addEventListener('animationend', () => {
                card.remove();
                // If room is now empty and we're not mid-refresh, remove room too
                const remaining = roomList.querySelectorAll('.Bird-card:not(.Bird-card-leaving)');
                if (remaining.length === 0 && !this.isRefreshing) {
                    roomDiv.classList.add('Bird-room-leaving');
                    roomDiv.addEventListener('animationend', () => roomDiv.remove(), { once: true });
                }
                this._scheduleOnce('filterPlayers', this.filterPlayers);
            }, { once: true });
        }

        // =================================================================
        // Chat & Filters
        // =================================================================

        getRoomColor(room) {
            if (!this.roomColors.has(room)) {
                this.roomColors.set(room, this.roomColorPalette[this.roomColors.size % this.roomColorPalette.length]);
            }
            return this.roomColors.get(room);
        }

        // Virtual scroller with a fixed pool of 80 reusable <div> nodes.
        // Bounded DOM, O(1) window math with ROW_HEIGHT=29, absolute-positioned
        // render zone so re-fills don't disturb scrollTop.
        _createVirtualScroller() {
            const POOL_SIZE = 80;
            const ROW_HEIGHT = 29;
            const BUFFER = 15;
            const container = this.messagesContainer;
            container.innerHTML = '';
            const scrollContent = document.createElement('div');
            scrollContent.className = 'pl-scroll-content';
            scrollContent.id = 'pl-scroll-content';
            const renderZone = document.createElement('div');
            renderZone.className = 'pl-render-zone';
            renderZone.id = 'pl-render-zone';
            scrollContent.appendChild(renderZone);
            container.appendChild(scrollContent);

            const pool = [];
            for (let i = 0; i < POOL_SIZE; i++) {
                const msg = document.createElement('div');
                msg.className = 'Bird-msg';
                const badge = document.createElement('span');
                badge.className = 'pl-badge';
                const authorSpan = document.createElement('span');
                authorSpan.className = 'pl-author';
                const textSpan = document.createElement('span');
                textSpan.className = 'pl-text';
                msg.appendChild(badge);
                msg.appendChild(authorSpan);
                msg.appendChild(textSpan);
                msg.style.display = 'none';
                renderZone.appendChild(msg);
                pool.push(msg);
            }

            const self = this;
            const scroller = {
                container, scrollContent, renderZone, pool,
                POOL_SIZE, ROW_HEIGHT, BUFFER,
                visibleIndices: null,
                renderPending: false,
                getItemCount() {
                    return this.visibleIndices ? this.visibleIndices.length : self._chatHistory.length;
                },
                getDataIndex(i) {
                    return this.visibleIndices ? this.visibleIndices[i] : i;
                },
                fillNode(node, data) {
                    const { author, text, isAccount, room } = data;
                    node.className = `Bird-msg ${isAccount ? 'Bird-msg-account' : 'Bird-msg-guest'}`;
                    node.dataset.room = room;
                    const badge = node.children[0];
                    const authorSpan = node.children[1];
                    const textSpan = node.children[2];
                    const color = self.getRoomColor(room);
                    const shortRoom = room.length > 12 ? room.slice(0, 12) + '\u2026' : room;
                    const safeText = text.length > 2000 ? text.slice(0, 2000) + '...' : text;
                    badge.style.background = color;
                    badge.textContent = shortRoom;
                    authorSpan.textContent = ` ${author}: `;
                    textSpan.textContent = safeText;
                },
                render() {
                    const total = this.getItemCount();
                    this.scrollContent.style.height = (total * this.ROW_HEIGHT) + 'px';
                    const scrollTop = this.container.scrollTop;
                    const vh = this.container.clientHeight;
                    let start = Math.max(0, Math.floor(scrollTop / this.ROW_HEIGHT) - this.BUFFER);
                    let end = Math.min(total, Math.ceil((scrollTop + vh) / this.ROW_HEIGHT) + this.BUFFER);
                    if (end - start > this.POOL_SIZE) end = start + this.POOL_SIZE;
                    this.renderZone.style.top = (start * this.ROW_HEIGHT) + 'px';
                    const count = Math.max(0, end - start);
                    for (let i = 0; i < this.POOL_SIZE; i++) {
                        const node = this.pool[i];
                        if (i < count) {
                            const dataIdx = this.getDataIndex(start + i);
                            const data = self._chatHistory[dataIdx];
                            if (data) {
                                this.fillNode(node, data);
                                node.style.display = '';
                                continue;
                            }
                        }
                        node.style.display = 'none';
                    }
                },
                scheduleRender() {
                    if (this.renderPending) return;
                    this.renderPending = true;
                    requestAnimationFrame(() => {
                        this.renderPending = false;
                        this.render();
                    });
                },
                setVisibleIndices(indices) {
                    this.visibleIndices = indices;
                    this.scheduleRender();
                }
            };

            // Single scroll handler drives both userScrolledUp and re-render.
            container.addEventListener('scroll', () => {
                const atBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 40;
                self._userScrolledUp = !atBottom;
                scroller.scheduleRender();
            }, { passive: true });

            return scroller;
        }

        // Trigram inverted index + set-intersection search, off the main thread.
        _createChatSearchWorker() {
            const code = `
                const trigramIndex = new Map();
                const roomIndex = new Map();
                const accountIndex = new Set();
                function addToIndex(idx, author, text, room, isAccount) {
                    const str = ((author || '') + ' ' + (text || '') + ' ' + (room || '')).toLowerCase();
                    for (let i = 0; i <= str.length - 3; i++) {
                        const tri = str.slice(i, i + 3);
                        let set = trigramIndex.get(tri);
                        if (!set) { set = new Set(); trigramIndex.set(tri, set); }
                        set.add(idx);
                    }
                    if (room) {
                        let set = roomIndex.get(room);
                        if (!set) { set = new Set(); roomIndex.set(room, set); }
                        set.add(idx);
                    }
                    if (isAccount) accountIndex.add(idx);
                }
                function search(q, rooms, accountsOnly) {
                    q = (q || '').toLowerCase();
                    if (q.length < 3 && (!rooms || rooms.length === 0) && !accountsOnly) return null;
                    let candidates = null;
                    if (q.length >= 3) {
                        for (let i = 0; i <= q.length - 3; i++) {
                            const tri = q.slice(i, i + 3);
                            const set = trigramIndex.get(tri);
                            if (!set || set.size === 0) return [];
                            if (!candidates) {
                                candidates = new Set(set);
                            } else {
                                for (const idx of candidates) if (!set.has(idx)) candidates.delete(idx);
                                if (candidates.size === 0) return [];
                            }
                        }
                    }
                    if (rooms && rooms.length > 0) {
                        const roomCandidates = new Set();
                        for (const r of rooms) {
                            const set = roomIndex.get(r);
                            if (set) for (const idx of set) roomCandidates.add(idx);
                        }
                        if (!candidates) candidates = roomCandidates;
                        else {
                            for (const idx of candidates) if (!roomCandidates.has(idx)) candidates.delete(idx);
                        }
                        if (candidates.size === 0) return [];
                    }
                    if (accountsOnly) {
                        if (!candidates) candidates = new Set(accountIndex);
                        else {
                            for (const idx of candidates) if (!accountIndex.has(idx)) candidates.delete(idx);
                        }
                    }
                    if (!candidates) return null;
                    const arr = [...candidates];
                    arr.sort((a, b) => a - b);
                    return arr;
                }
                self.onmessage = (e) => {
                    const msg = e.data;
                    if (msg.type === 'index') {
                        addToIndex(msg.idx, msg.author, msg.text, msg.room, msg.isAccount);
                    } else if (msg.type === 'rebuild') {
                        trigramIndex.clear();
                        roomIndex.clear();
                        accountIndex.clear();
                        for (let i = 0; i < msg.history.length; i++) {
                            const h = msg.history[i];
                            addToIndex(i, h.author, h.text, h.room, h.isAccount);
                        }
                    } else if (msg.type === 'clear') {
                        trigramIndex.clear();
                        roomIndex.clear();
                        accountIndex.clear();
                    } else if (msg.type === 'search') {
                        const result = search(msg.q, msg.rooms, msg.accountsOnly);
                        postMessage({ type: 'result', searchId: msg.searchId, result });
                    }
                };
            `;
            const blob = new Blob([code], { type: 'application/javascript' });
            const worker = new Worker(URL.createObjectURL(blob));
            this._pendingSearches = new Map();
            this._searchId = 0;
            worker.onmessage = (e) => {
                if (e.data.type === 'result') {
                    const resolver = this._pendingSearches.get(e.data.searchId);
                    if (resolver) {
                        this._pendingSearches.delete(e.data.searchId);
                        resolver(e.data.result);
                    }
                }
            };
            return worker;
        }

        _searchChat(q, rooms, accountsOnly) {
            return new Promise((resolve) => {
                const id = ++this._searchId;
                this._pendingSearches.set(id, resolve);
                this._chatSearchWorker.postMessage({
                    type: 'search', searchId: id, q, rooms, accountsOnly
                });
            });
        }

        addMessage(author, text, isAccount, room, foto) {
            // No runtime cap — the user-chosen Load limit (or "All forever") is the
            // only ceiling, applied at backfill time in _backfillNewestPage.
            const idx = this._chatHistory.length;
            this._chatHistory.push({ author, text, isAccount, room, foto });
            this._chatSearchWorker.postMessage({ type: 'index', idx, author, text, room, isAccount });

            // Throttled header update — every 50 msgs, plus first 10 for responsiveness.
            const len = this._chatHistory.length;
            if (len <= 10 || len % 50 === 0) this._updateChatTitle();

            this._onMessageAdded();
        }

        // Single source of truth for the chat panel's title — reflects every
        // active filter (rooms + author) plus the total in-memory count.
        _chatHeaderText() {
            const parts = [];
            if (this.activeRoomFilters.size > 0) parts.push([...this.activeRoomFilters].join(', '));
            if (this.activeAuthorLabel) parts.push(`from ${this.activeAuthorLabel}`);
            const base = parts.length ? `Chat (${parts.join(' · ')})` : 'Chat';
            return `${base} (${this._chatHistory.length.toLocaleString()})`;
        }
        _updateChatTitle() {
            if (this.chatTitle) this.chatTitle.textContent = this._chatHeaderText();
        }

        // Collect every nickname we know this tracked user has gone by.
        // Used by the editor's "Show this person's chat" button.
        _collectTrackedNames(t) {
            const s = new Set();
            for (const id of (t.identifiers || [])) {
                if (id.type === 'name' && id.value) s.add(id.value);
            }
            if (t.lastKnownName) s.add(t.lastKnownName);
            return s;
        }

        // Apply an author filter (chat shows only messages where author ∈ names).
        // Composes with room/search/accountsOnly filters (all AND together).
        setAuthorFilter(trackedId, names, label) {
            this.activeAuthorNames = new Set(names);
            this.activeAuthorLabel = label;
            this.activeAuthorTrackedId = trackedId;
            this._updateChatTitle();
            this.filterChat();
        }
        clearAuthorFilter() {
            if (!this.activeAuthorNames) return;
            this.activeAuthorNames = null;
            this.activeAuthorLabel = null;
            this.activeAuthorTrackedId = null;
            this._updateChatTitle();
            this.filterChat();
        }

        _onMessageAdded() {
            const hasSearch = this.msgSearchInput.value.trim().length > 0;
            const hasRoomFilter = this.activeRoomFilters.size > 0;
            const hasAccountsOnly = this.accountFilterCheckbox?.checked || false;
            const hasAuthorFilter = !!this.activeAuthorNames;

            if (hasSearch || hasRoomFilter || hasAccountsOnly || hasAuthorFilter) {
                this._scheduleOnce('filterChat', this.filterChat);
            } else {
                this._chatScroller.setVisibleIndices(null);
            }

            // Auto-scroll only when user hasn't scrolled up to read history.
            if (!this._userScrolledUp) {
                this._scheduleOnce('chatAutoScroll', () => {
                    const el = this.messagesContainer;
                    el.scrollTop = el.scrollHeight;
                });
            }
        }

        async filterChat() {
            const parsed = this._parseSearchQuery(this.msgSearchInput.value);
            const q = parsed.empty ? '' : parsed.term;
            const rooms = this.activeRoomFilters.size > 0 ? [...this.activeRoomFilters] : null;
            const accountsOnly = this.accountFilterCheckbox?.checked || false;
            const authorNames = this.activeAuthorNames; // Set<string> or null

            if (!q && !rooms && !accountsOnly && !authorNames) {
                this._chatScroller.setVisibleIndices(null);
                return;
            }

            const qLower = q.toLowerCase();
            const exactRe = (parsed.exact && qLower) ? this._buildExactWordRegex(qLower) : null;

            // Author-only path (no search, no room, no accounts) and sub-3-char-search
            // path both linear-scan _chatHistory — the worker's trigram index can't
            // help with exact author matching anyway.
            if ((q.length > 0 && q.length < 3) || (!q && authorNames && !rooms && !accountsOnly)) {
                const indices = [];
                const hist = this._chatHistory;
                for (let i = 0; i < hist.length; i++) {
                    const m = hist[i];
                    if (q) {
                        const hay = (`${m.author} ${m.text} ${m.room || ''}`).toLowerCase();
                        if (exactRe ? !exactRe.test(hay) : !hay.includes(qLower)) continue;
                    }
                    if (rooms && !rooms.includes(m.room)) continue;
                    if (accountsOnly && !m.isAccount) continue;
                    if (authorNames && !authorNames.has(m.author)) continue;
                    indices.push(i);
                }
                this._chatScroller.setVisibleIndices(indices);
                return;
            }

            const searchId = this._searchId + 1;
            let result = await this._searchChat(q, rooms, accountsOnly);
            // If a newer search fired while we were waiting, drop this stale result.
            if (searchId !== this._searchId) return;
            // Exact-mode post-filter: trigrams narrow candidates, regex enforces word boundaries.
            if (exactRe && Array.isArray(result)) {
                const hist = this._chatHistory;
                result = result.filter(i => {
                    const m = hist[i];
                    if (!m) return false;
                    return exactRe.test((`${m.author} ${m.text} ${m.room || ''}`).toLowerCase());
                });
            }
            // Author post-filter — applied after the worker narrows by search/room/account.
            if (authorNames) {
                const hist = this._chatHistory;
                if (result === null) {
                    // Worker returned "no filter" but we DO want to narrow — fall back
                    // to a linear scan of the full history with the author predicate.
                    const indices = [];
                    for (let i = 0; i < hist.length; i++) {
                        if (authorNames.has(hist[i].author)) indices.push(i);
                    }
                    result = indices;
                } else {
                    result = result.filter(i => {
                        const m = hist[i];
                        return m && authorNames.has(m.author);
                    });
                }
            }
            this._chatScroller.setVisibleIndices(result === null ? null : result);
        }

        filterPlayers() {
            const parsed = this._parseSearchQuery(this.searchInput.value);
            const q = parsed.empty ? '' : parsed.term.toLowerCase();
            const exactRe = (parsed.exact && q) ? this._buildExactWordRegex(q) : null;
            const r = this.roomInput.value.toLowerCase();
            let visiblePlayerTotal = 0;
            let visibleRoomTotal = 0;
            const roomNodes = this.playersContainer.children;
            for (let i = 0; i < roomNodes.length; i++) {
                const roomDiv = roomNodes[i];
                const cls = roomDiv.classList;
                if (!cls || !cls.contains('Bird-room-group') || cls.contains('Bird-room-leaving')) continue;
                const roomCode = (roomDiv.dataset.roomCode || '').toLowerCase();
                let visiblePlayerCount = 0;
                const roomList = roomDiv.querySelector('.Bird-room-list');
                if (roomList) {
                    const cards = roomList.children;
                    for (let j = 0; j < cards.length; j++) {
                        const card = cards[j];
                        const cc = card.classList;
                        if (!cc || !cc.contains('Bird-card') || cc.contains('Bird-card-leaving')) continue;
                        const name = (card.dataset.playerName || '').toLowerCase();
                        const nameMatch = !q || (exactRe ? exactRe.test(name) : name.includes(q));
                        const match = nameMatch && roomCode.includes(r);
                        card.style.display = match ? "flex" : "none";
                        if (match) visiblePlayerCount++;
                    }
                }
                if (visiblePlayerCount > 0 && roomCode.includes(r)) {
                    roomDiv.style.display = "block";
                    visibleRoomTotal++;
                    visiblePlayerTotal += visiblePlayerCount;
                } else {
                    roomDiv.style.display = "none";
                }
            }
            this.activePlayersSpan.textContent = `Active Players: ${visiblePlayerTotal}`;
            this.activeRoomsSpan.textContent = `Active Rooms: ${visibleRoomTotal}` + (this._totalRoomsEverSeen > visibleRoomTotal ? ` (${this._totalRoomsEverSeen} total seen)` : '');
        }

        // =================================================================
        // Bird Backend
        // =================================================================

        /**
         * getSelectedLangs — returns array of language codes based on checkboxes
         */
        getSelectedLangs() {
            const langs = new Set();
            if (this.activeMode === 'arabic') langs.add(19);
            if (this.activeMode === 'english') langs.add(2);
            if (this.activeMode === 'bulgarian') langs.add(21);
            if (this.activeMode === 'khmer') langs.add(48);
            if (this.activeMode === 'all') this.ALL_LANGS.forEach(l => langs.add(l));
            return [...langs];
        }

        /** Whether public room watching is enabled */
        isPublicMode() {
            return this.activeMode === 'public';
        }

        // === Server-side persistent session (browser becomes a viewer) ===

        async _maybeResumeServerSession() {
            let state;
            try {
                const r = await fetch('/bird/api/session/state');
                state = await r.json();
            } catch(e) { return; }
            if (!state || !state.running) return;

            console.log('[Bird] resuming server session', state.id);
            this._serverSession = { id: state.id, langs: state.langs };
            this.activeMode = 'server-session';
            this.startScreen.style.display = 'none';
            this.app.classList.add('Bird-main--active');

            for (const room of (state.rooms || [])) {
                this.connectedRooms.add(room.code);
                if (!this.playerStores[room.code]) {
                    this.playerStores[room.code] = this.createPlayerStore();
                }
                const plst = this.playerStores[room.code];
                for (const id in (room.players || {})) {
                    const p = room.players[id];
                    plst.add({ id: p.id, nick: p.nick, foto: p.foto, avatar: p.avatar });
                    this.addPlayer(
                        p.foto || `https://gartic.io/static/images/avatar/svg/${p.avatar}.svg`,
                        p.nick,
                        room.code,
                        p.id
                    );
                }
            }
            if (this.runOnServerBtn) {
                this.runOnServerBtn.textContent = 'Stop server session';
                this.runOnServerBtn.style.background = '#dc2626';
            }
            // First page: the newest PAGE messages, appended in order so they paint
            // at the bottom instantly. We subscribe to live events RIGHT after this
            // so incoming chat/player events flow into the bottom in real time while
            // the older-page backfill streams in above (no live-event "burst" at the
            // end of backfill anymore).
            const backfillState = await this._backfillNewestPage(state.totalChats);
            this._subscribeServerEvents();
            // Resume seeded players from a snapshot taken BEFORE we subscribed.
            // One debounced resync immediately closes any gap from events that
            // fired in the tiny window before _subscribeServerEvents().
            this._scheduleServerResync('resume');
            // Older pages run in the background; their prepends compose correctly
            // with live appends because each prepend bumps scrollTop by exactly
            // the height of the new rows.
            this._backfillOlderPages(backfillState);
        }

        // Load chat history from the server, newest-page first.
        //
        // Split in two so live events can subscribe between them:
        //   1) _backfillNewestPage  — awaited. Appends the tail (newest PAGE) so
        //      recent chat paints instantly at the bottom. Returns the state
        //      needed to resume the loop (target/loaded/from/maxPages).
        //   2) _backfillOlderPages  — fired without await. Prepends older
        //      PAGE-message chunks above what's already rendered, in the
        //      background, while live messages keep arriving at the bottom.
        //
        // Net effect: recent messages are visible immediately; older history
        // streams in above them in PAGE-message chunks until the user-selected
        // Load limit is hit (or the file is exhausted for "All (forever)");
        // live chat lands at the bottom in real time the whole way through.
        async _backfillNewestPage(totalChats) {
            const PAGE = 20000;
            if (typeof totalChats !== 'number' || totalChats <= 0) {
                return { target: 0, loaded: 0, from: 0, maxPages: 0 };
            }
            const limitRaw = (this.chatLoadLimitSelect && this.chatLoadLimitSelect.value) || '10000';
            const userLimit = parseInt(limitRaw, 10) || 0; // 0 means "all"
            const target = userLimit > 0 ? Math.min(userLimit, totalChats) : totalChats;
            const pageSize = Math.min(PAGE, target);
            const from = Math.max(0, totalChats - pageSize);
            let loaded = 0;
            try {
                const r = await fetch(`/bird/api/session/chat?from=${from}&limit=${pageSize}`);
                const data = await r.json();
                const msgs = data.messages || [];
                for (const m of msgs) {
                    this.addMessage(m.author, m.text, m.isAccount, m.room, m.foto);
                }
                loaded = msgs.length;
            } catch (e) { /* fall through; older-pages loop will still try */ }
            const maxPages = Math.ceil(target / PAGE) + 5;
            const nextFrom = from === 0 ? 0 : Math.max(0, from - Math.min(PAGE, target - loaded));
            return { target, loaded, from: nextFrom, firstFrom: from, maxPages };
        }

        // Background loop — prepends successively older pages above
        // what _backfillNewestPage already painted. Runs without awaiting at the
        // caller so live SSE events (handled in parallel) can land at the bottom
        // while history streams in above.
        async _backfillOlderPages(state) {
            if (!state || state.target <= 0 || state.loaded >= state.target) return;
            if (state.firstFrom === 0) return; // newest page was also the oldest
            const PAGE = 20000;
            const CONCURRENCY = 4;
            const { target, loaded: initialLoaded } = state;
            const remaining = target - initialLoaded;
            if (remaining <= 0) return;

            // Pre-compute page descriptors: descending order so the FIRST entry
            // is the page immediately older than what newest-page already painted.
            // Each tuple is { from, size, order } where `order` is the slot index
            // (0 = oldest, N-1 = closest to newest-already-rendered). We prepend
            // in ascending `order` so the oldest chunk lands at the very top.
            const pages = [];
            let nextFrom = state.firstFrom;
            let toFetch = remaining;
            while (toFetch > 0 && nextFrom > 0) {
                const size = Math.min(PAGE, toFetch);
                const from = Math.max(0, nextFrom - size);
                pages.push({ from, size });
                toFetch -= size;
                nextFrom = from;
                if (from === 0) break;
            }
            if (pages.length === 0) return;

            // pages is in newest→oldest order. _prependMessages unshifts to
            // _chatHistory, so the last prepend ends up at the very top. To
            // place oldest at the top, we must prepend in pages-array order:
            // pages[0] (newest of the older) first, pages[N-1] (oldest) last.
            const results = new Array(pages.length);
            let cursor = 0;
            let didPrepend = false;
            let nextToPrepend = 0;

            const worker = async () => {
                while (true) {
                    const myIdx = cursor++;
                    if (myIdx >= pages.length) return;
                    const { from, size } = pages[myIdx];
                    try {
                        const r = await fetch(`/bird/api/session/chat?from=${from}&limit=${size}`);
                        const data = await r.json();
                        results[myIdx] = data.messages || [];
                    } catch (e) {
                        results[myIdx] = null; // signal failure for this slot
                    }
                    // Flush any contiguous-from-newest-older results that are ready.
                    while (nextToPrepend < pages.length && results[nextToPrepend] !== undefined) {
                        const msgs = results[nextToPrepend];
                        results[nextToPrepend] = undefined; // free
                        nextToPrepend++;
                        if (msgs && msgs.length > 0) {
                            this._prependMessages(msgs, { deferIndex: true });
                            didPrepend = true;
                        }
                    }
                }
            };
            try {
                const workers = [];
                for (let i = 0; i < Math.min(CONCURRENCY, pages.length); i++) {
                    workers.push(worker());
                }
                await Promise.all(workers);
            } finally {
                // One rebuild at the end (or after any abort) — search results
                // for backfilled messages stay stale during the load, then snap
                // to fully indexed when the loop exits.
                if (didPrepend) {
                    this._chatSearchWorker.postMessage({ type: 'rebuild', history: this._chatHistory });
                }
            }
        }

        // Insert a chunk of older messages at the front of _chatHistory while
        // keeping the user visually pinned (whether they're at the bottom or
        // mid-scroll reading older history). Used by _backfillOlderPages to
        // stream pages above already-rendered recent messages.
        _prependMessages(msgs, opts) {
            if (!msgs || msgs.length === 0) return;
            const objs = msgs.map(m => ({
                author: m.author, text: m.text, isAccount: m.isAccount,
                room: m.room, foto: m.foto,
            }));
            const M = objs.length;
            const rh = this._chatScroller.ROW_HEIGHT;
            const el = this.messagesContainer;

            this._chatHistory.unshift(...objs);

            // Every existing entry's index just shifted by M — rebuild the
            // worker's trigram index from the new full history. Skipped when
            // a batched backfill is calling us; it issues ONE rebuild at the
            // end. Without this guard each 500-msg page ships the entire
            // growing history via postMessage, turning backfill into O(N²)
            // work that consistently stalls the browser around 300–400k msgs.
            if (!opts || !opts.deferIndex) {
                this._chatSearchWorker.postMessage({ type: 'rebuild', history: this._chatHistory });
            }

            // Active filter's visibleIndices reference the pre-prepend array.
            const hasSearch = this.msgSearchInput.value.trim().length > 0;
            const hasRoomFilter = this.activeRoomFilters.size > 0;
            const hasAccountsOnly = this.accountFilterCheckbox?.checked || false;
            const hasAuthorFilter = !!this.activeAuthorNames;
            const hasFilter = hasSearch || hasRoomFilter || hasAccountsOnly || hasAuthorFilter;
            if (hasFilter) this._chatScroller.visibleIndices = null;

            // Grow scrollContent height synchronously BEFORE bumping scrollTop —
            // otherwise the browser clamps the bump against the old (smaller) max.
            // render() will reassert this same value on the next animation frame.
            this._chatScroller.scrollContent.style.height = (this._chatHistory.length * rh) + 'px';
            el.scrollTop = el.scrollTop + M * rh;

            this._updateChatTitle();

            if (hasFilter) {
                this._scheduleOnce('filterChat', this.filterChat);
            } else {
                this._chatScroller.scheduleRender();
            }
        }

        _subscribeServerEvents() {
            if (this._serverEventSource) {
                try { this._serverEventSource.close(); } catch(e){}
            }
            const es = new EventSource('/bird/api/session/events');
            this._serverEventSource = es;
            es.addEventListener('chat', e => {
                try {
                    const d = JSON.parse(e.data);
                    this.addMessage(d.author, d.text, d.isAccount, d.room, d.foto);
                    // A live chat with a resolved author means the server has that
                    // player in r.Players right now. If we don't, their player_joined
                    // SSE event was dropped (slow-consumer select-default in emit, or
                    // the resume snapshot/subscribe race). Schedule a resync so the
                    // roster catches up in <2s instead of waiting up to 60s.
                    if (d.authorId && d.author && d.room) {
                        const plst = this.playerStores[d.room];
                        if (!plst || !plst.map.has(String(d.authorId))) {
                            this._scheduleServerResync('chat-author-missing');
                        }
                    }
                } catch(err) {}
            });
            es.addEventListener('player_joined', e => {
                try {
                    const d = JSON.parse(e.data);
                    const p = d.player;
                    if (!this.playerStores[d.room]) this.playerStores[d.room] = this.createPlayerStore();
                    this.playerStores[d.room].add({ id: p.id, nick: p.nick, foto: p.foto, avatar: p.avatar });
                    this.addPlayer(p.foto || `https://gartic.io/static/images/avatar/svg/${p.avatar}.svg`, p.nick, d.room, p.id);
                } catch(err) {}
            });
            es.addEventListener('player_left', e => {
                try {
                    const d = JSON.parse(e.data);
                    const plst = this.playerStores[d.room];
                    if (!plst) return;
                    const name = plst.getName(d.playerId);
                    plst.remove(d.playerId);
                    // Pass the id so identical-nick players are removed individually;
                    // name is still passed for tracked-status cleanup.
                    this.removePlayer(name, d.room, d.playerId);
                } catch(err) {}
            });
            es.addEventListener('room_added', e => {
                try {
                    const d = JSON.parse(e.data);
                    this.connectedRooms.add(d.code);
                    if (!this.playerStores[d.code]) this.playerStores[d.code] = this.createPlayerStore();
                } catch(err) {}
            });
            es.addEventListener('room_removed', e => {
                try {
                    const d = JSON.parse(e.data);
                    this.connectedRooms.delete(d.code);
                    // remove DOM card if any (id pattern set in addPlayer at line 1953-1954)
                    const card = document.getElementById(`room-group-${d.code}`);
                    if (card) card.remove();
                } catch(err) {}
            });
            es.onerror = () => {
                // SSE dropped. EventSource auto-reconnects on TRANSIENT errors
                // (readyState=CONNECTING). But if the server responds non-2xx
                // (e.g. restart while we were idle, network-level 502 from the
                // relay's proxy), the browser puts the EventSource into CLOSED
                // and stops retrying entirely — silent permanent loss of updates.
                // Detect CLOSED and recreate so the stream resumes; always
                // resync afterwards to backfill anything we missed during the gap.
                if (es.readyState === EventSource.CLOSED && this._serverEventSource === es) {
                    // Recreate after a short backoff. _subscribeServerEvents closes the
                    // old reference and reinstalls handlers; we just need the kick.
                    if (this._serverESRetryTimer) clearTimeout(this._serverESRetryTimer);
                    this._serverESRetryTimer = setTimeout(() => {
                        this._serverESRetryTimer = null;
                        if (this._serverSession) {
                            this._subscribeServerEvents();
                            this._scheduleServerResync('sse-recreated');
                        }
                    }, 2000);
                    return;
                }
                this._scheduleServerResync('sse-error');
            };
            // Defense-in-depth periodic resync. Even without an SSE error, slow
            // consumers can drop events (subBufferSize=256 with select-default).
            // 15s instead of 60s: the snapshot fetch is cheap (no per-message work,
            // just JSON of current rooms+players), and 60s was visible enough that
            // users had to reload to see correct rooms. With a 15s ceiling, the
            // worst-case drift window is short enough that the UI is never far
            // off ground truth.
            if (this._serverResyncTimer) clearInterval(this._serverResyncTimer);
            this._serverResyncTimer = setInterval(() => {
                if (this._serverSession) this._resyncServerState('periodic').catch(()=>{});
            }, 15000);
        }

        // Schedule a resync after a short delay so we don't hammer the server when
        // SSE flaps repeatedly (e.g. proxy hiccup, laptop wake). Coalesces multiple
        // error events into one resync call.
        _scheduleServerResync(reason) {
            if (this._serverResyncPending) return;
            this._serverResyncPending = true;
            setTimeout(() => {
                this._serverResyncPending = false;
                if (this._serverSession) this._resyncServerState(reason).catch(()=>{});
            }, 1500);
        }

        // Reconcile local state against the server snapshot — snapshot is the
        // authoritative source, both for the data store AND the rendered DOM.
        // We don't trust plst.map as a proxy for "is this card rendered"; the
        // DOM is queried directly so a card missing for any reason (event drop,
        // animation timing, previous name-keyed dedup leaving plst out of sync
        // with rendered cards) gets corrected here. Idempotent.
        async _resyncServerState(reason) {
            let state;
            try {
                const r = await fetch('/bird/api/session/state');
                state = await r.json();
            } catch(e) { return; }
            if (!state || !state.running) return;

            const snapshotRooms = new Map();   // code -> room
            for (const room of (state.rooms || [])) {
                snapshotRooms.set(room.code, room);
            }

            let addedPlayers = 0, removedPlayers = 0, addedRooms = 0, removedRooms = 0;

            // Drop local rooms that aren't in the snapshot.
            for (const code of [...this.connectedRooms]) {
                if (!snapshotRooms.has(code)) {
                    this.connectedRooms.delete(code);
                    delete this.playerStores[code];
                    const card = document.getElementById(`room-group-${code}`);
                    if (card) card.remove();
                    removedRooms++;
                }
            }

            // For each snapshot room, reconcile both data store and DOM against
            // the snapshot's player set. Cards are looked up by data-player-id so
            // identical-nick players are tracked individually.
            for (const [code, room] of snapshotRooms) {
                if (!this.connectedRooms.has(code)) {
                    this.connectedRooms.add(code);
                    addedRooms++;
                }
                if (!this.playerStores[code]) this.playerStores[code] = this.createPlayerStore();
                const plst = this.playerStores[code];

                const snapshotIDs = new Set();
                for (const id in (room.players || {})) snapshotIDs.add(String(id));

                // Walk snapshot: ensure each id has a data store entry AND a DOM card.
                // We must check the DOM separately from plst.map because the two have
                // historically diverged (name-keyed cards collapsed multi-id players).
                const roomDiv = document.getElementById(`room-group-${code}`);
                const roomList = roomDiv ? roomDiv.querySelector('.Bird-room-list') : null;
                for (const id of snapshotIDs) {
                    const p = room.players[id];
                    plst.add({ id: p.id, nick: p.nick, foto: p.foto, avatar: p.avatar });
                    let hasCard = false;
                    if (roomList) {
                        const safeId = CSS.escape(String(id));
                        hasCard = !!roomList.querySelector(`.Bird-card[data-player-id="${safeId}"]:not(.Bird-card-leaving)`);
                    }
                    if (!hasCard) {
                        this.addPlayer(
                            p.foto || `https://gartic.io/static/images/avatar/svg/${p.avatar}.svg`,
                            p.nick,
                            code,
                            p.id
                        );
                        addedPlayers++;
                    }
                }

                // Drop data-store entries the snapshot says are gone.
                for (const id of [...plst.map.keys()]) {
                    if (snapshotIDs.has(id)) continue;
                    const name = plst.getName(id);
                    plst.remove(id);
                    this.removePlayer(name, code, id);
                    removedPlayers++;
                }

                // Drop any DOM cards for IDs not in snapshot (defensive — covers
                // cards whose plst entry was already removed in a prior pass but
                // whose DOM card lingered, e.g. animation interrupted).
                if (roomList) {
                    const stray = roomList.querySelectorAll('.Bird-card[data-player-id]');
                    for (const card of stray) {
                        const cid = card.getAttribute('data-player-id');
                        if (cid && !snapshotIDs.has(cid) && !card.classList.contains('Bird-card-leaving')) {
                            const nm = card.getAttribute('data-player-name') || '';
                            this.removePlayer(nm, code, cid);
                            removedPlayers++;
                        }
                    }
                }
            }

            if (addedPlayers || removedPlayers || addedRooms || removedRooms) {
                console.log(`[Bird] resync(${reason}): rooms +${addedRooms}/-${removedRooms}, players +${addedPlayers}/-${removedPlayers}`);
            }
        }

        async _toggleServerSession() {
            if (this._serverSession) {
                // Stopping
                const secret = this._getAdminSecret();
                if (!secret) return;
                try {
                    const r = await fetch('/bird/api/session/stop', {
                        method: 'POST',
                        headers: { 'X-Auth': secret }
                    });
                    if (r.status === 401) {
                        try { localStorage.removeItem('relay_admin_secret'); } catch(e){}
                        alert('Wrong secret. Cleared cache — click again to re-enter.');
                        return;
                    }
                    if (!r.ok) {
                        alert('Stop failed: HTTP ' + r.status);
                        return;
                    }
                } catch(e) { alert('Stop failed: ' + e); return; }
                this._serverSession = null;
                if (this._serverESRetryTimer) {
                    clearTimeout(this._serverESRetryTimer);
                    this._serverESRetryTimer = null;
                }
                if (this._serverEventSource) {
                    try { this._serverEventSource.close(); } catch(e){}
                    this._serverEventSource = null;
                }
                if (this._serverResyncTimer) {
                    clearInterval(this._serverResyncTimer);
                    this._serverResyncTimer = null;
                }
                this.runOnServerBtn.textContent = 'Run on server';
                this.runOnServerBtn.style.background = '';
                // Return UI to start screen
                this.app.classList.remove('Bird-main--active');
                this.startScreen.style.display = 'flex';
                return;
            }

            // Starting
            const langs = this.getSelectedLangs();
            if (!langs || langs.length === 0) {
                alert('Select a language first (use the start screen language buttons).');
                return;
            }
            const secret = this._getAdminSecret();
            if (!secret) return;

            let r;
            try {
                r = await fetch('/bird/api/session/start', {
                    method: 'POST',
                    headers: { 'X-Auth': secret, 'Content-Type': 'application/json' },
                    body: JSON.stringify({ langs })
                });
            } catch(e) { alert('Start failed: ' + e); return; }
            if (r.status === 401) {
                try { localStorage.removeItem('relay_admin_secret'); } catch(e){}
                alert('Wrong secret. Cleared cache — click again to re-enter.');
                return;
            }
            if (r.status === 409) {
                alert('A session is already running on the server. Reload to attach.');
                return;
            }
            if (!r.ok) { alert('Start failed: HTTP ' + r.status); return; }

            const data = await r.json();
            this._serverSession = { id: data.id, langs };
            // Tear down any local-mode timers if user toggled server-on mid-session.
            if (this._autoMonitorActive) this.stopAutoMonitor();
            this.runOnServerBtn.textContent = 'Stop server session';
            this.runOnServerBtn.style.background = '#dc2626';
            // Hide start screen, show app, subscribe
            this.startScreen.style.display = 'none';
            this.app.classList.add('Bird-main--active');
            this._subscribeServerEvents();
        }

        _getAdminSecret() {
            let s = '';
            try { s = localStorage.getItem('relay_admin_secret') || ''; } catch(e){}
            if (!s) {
                s = prompt('Relay admin secret:');
                if (!s) return null;
                try { localStorage.setItem('relay_admin_secret', s); } catch(e){}
            }
            return s;
        }

        createPlayerStore() {
            return {
                map: new Map(),
                add(p) { this.map.set(String(p.id), { name: p.nick, isAccount: !!p.foto, foto: p.foto || null, avatar: p.avatar ?? null }); },
                getName(id) { return this.map.get(String(id))?.name ?? null; },
                getAvatar(id) { const e = this.map.get(String(id)); return e?.foto || (e?.avatar != null ? `https://gartic.io/static/images/avatar/svg/${e.avatar}.svg` : null); },
                getFoto(id) { return this.map.get(String(id))?.foto ?? null; },
                isAccount(id) { return this.map.get(String(id))?.isAccount ?? false; },
                remove(id) { this.map.delete(String(id)); },
            };
        }

        async monitorRooms(languagecodes, callback) {
            const langs = Array.isArray(languagecodes) ? languagecodes : [Number(languagecodes)];
            console.log('[monitorRooms] Starting with', langs.length, 'languages (direct fetch, no proxy)');
            const results = {};
            for (const lang of langs) {
                try {
                    const r = await fetch(`/bird/api/rooms?lang=${encodeURIComponent(lang)}`);
                    if (!r.ok) {
                        console.log(`[monitorRooms] lang=${lang} HTTP ${r.status}`);
                        continue;
                    }
                    const text = await r.text();
                    console.log(`[monitorRooms] lang=${lang} status=${r.status} len=${text.length} snippet="${text.substring(0, 200)}"`);
                    if (!text) continue;
                    const data = JSON.parse(text).filter(x => x.quant > 0);
                    const rooms = data.map(x => x.code);
                    if (rooms.length) results[lang] = rooms;
                } catch (e) {
                    console.log(`[monitorRooms] Lang ${lang} error:`, e.message || e);
                }
                await new Promise(r => setTimeout(r, 500));
            }
            console.log('[monitorRooms] Results:', JSON.stringify(results));
            callback?.(results);
        }

        async checkDeepProxy(yua) {
            const info = { ip: yua.ip, cookie: yua.cookie, roomcode: yua.roomcode, language: yua.language };
            try {
                // Fetch server info directly from gartic.io (bypasses proxy Cloudflare block)
                const r = await fetch(`/bird/api/server-check?room=${encodeURIComponent(yua.roomcode)}`);
                const text = await r.text();
                // Extract server name (e.g. "server03") from the response
                const serverMatch = text.match(/server0[1-7]/);
                return { info, response: text, server: serverMatch ? serverMatch[0] : null };
            } catch(e) {
                return { info, response: '', server: null };
            }
        }

        connectToRoom(proxy, roomCode, serverResponse, serverName) {
            if (this._serverSession) return; // server owns the WS
            const servers = { "server07": "c", "server06": "Y", "server05": "U", "server04": "Q", "server03": "M", "server02": "I", "server01": "E" };
            const roomid = roomCode.substring(2);
            const ip = typeof proxy === 'object' ? proxy.ip : proxy;

            if (this.connectedRooms.has(roomCode)) return;

            if (!this._proxyFails) this._proxyFails = {};
            if ((this._proxyFails[ip] || 0) >= 3) return;

            this.connectedRooms.add(roomCode);
            this._totalRoomsEverSeen++;

            if (!this.playerStores[roomCode]) {
                this.playerStores[roomCode] = this.createPlayerStore();
            }
            const plst = this.playerStores[roomCode];

            let serverCodes;
            if (serverName && servers[serverName]) {
                serverCodes = [servers[serverName]];
            } else if (serverResponse && typeof serverResponse === 'string') {
                serverCodes = [servers["server06"]];
                for (const [name, code] of Object.entries(servers)) {
                    if (serverResponse.includes(name)) { serverCodes = [code]; break; }
                }
            } else {
                serverCodes = [servers["server06"]];
            }

            serverCodes.forEach(scode => {
                const targetB64 = `d3NzOi8vc2VydmVyMD${scode}uZ2FydGljLmlvL3NvY2tldC5pby8/RUlPPTMmdHJhbnNwb3J0PXdlYnNvY2tldA==`;
                const wsUrl = `wss://${location.host}/bird/relay?ip=${encodeURIComponent(ip)}&target=${encodeURIComponent(targetB64)}`;
                const websocket = new WebSocket(wsUrl);
                websocket._lastFrameAt = Date.now();
                websocket._roomCode = roomCode;
                this.activeSockets.push(websocket);

                websocket.onopen = () => {
                    if (this._proxyFails) this._proxyFails[ip] = 0;
                    websocket._lastFrameAt = Date.now();
                    websocket._openedAt = Date.now();
                    websocket.send(`42[12,{"v":20000,"platform":0,"sala":"${roomid}"}]`);
                    websocket.send(`42[46,0]`);
                    // Per-room BroadcastChannel — Fast Viewer tabs subscribe to this and
                    // receive every raw socket.io frame, so they don't need their own relay WS.
                    try {
                        websocket._fvChannel = new BroadcastChannel('bird-fv:' + roomCode);
                    } catch (err) {
                        websocket._fvChannel = null;
                    }
                };

                // Shared onmessage — uses e.target so pong always goes to the right socket
                const onMsg = e => {
                    try {
                        // Stamp activity FIRST — server pings count as life signs.
                        e.target._lastFrameAt = Date.now();
                        // Engine.IO pong: server sends "2" (ping), we must reply "3" (pong)
                        if (e.data === "2") { e.target.send("3"); return; }
                        // Re-broadcast every non-ping frame so FV tabs can consume them.
                        // FV's parser expects raw strings — pass through unchanged.
                        try { e.target._fvChannel?.postMessage({ type: 'wsMessage', data: e.data }); } catch (_) {}
                        if (!e.data.startsWith("42")) return;
                        const data = JSON.parse(e.data.slice(2));

                        // Room joined — full player list. Treat as authoritative
                        // for the room: anyone in plst but missing here left during
                        // a silent stretch (zombie reconnect, server resync, etc.).
                        if (data[0] === "5") {
                            e.target._joined = true;
                            const incomingIds = new Set(data[5].map(p => String(p.id)));
                            const toRemove = [];
                            for (const [pid, entry] of plst.map) {
                                if (!incomingIds.has(pid)) toRemove.push([pid, entry?.name]);
                            }
                            for (const [pid, name] of toRemove) {
                                plst.remove(pid);
                                if (name) this.removePlayer(name, roomCode);
                            }
                            for (const player of data[5]) {
                                plst.add({ id: player.id, nick: player.nick, foto: player.foto, avatar: player.avatar });
                                this.addPlayer(
                                    player.foto ?? `https://gartic.io/static/images/avatar/svg/${player.avatar}.svg`,
                                    player.nick,
                                    roomCode
                                );
                            }
                        }

                        // Room error — close and unmark so auto-monitor can retry
                        if (data[0] === "6" && data[1] == 6) {
                            this.connectedRooms.delete(roomCode);
                            e.target.close();
                        }

                        // Chat message (event 11) or answer/guess (event 13) — rendered identically
                        if (data[0] === "11" || data[0] === "13") {
                            this.addMessage(plst.getName(data[1]), data[2], plst.isAccount(data[1]), roomCode, plst.getAvatar(data[1]));
                        }

                        // Player joined live — data[1] is a player object {id, nick, avatar, foto}
                        if (data[0] === "23") {
                            const player = data[1];
                            player.foto ||= `https://gartic.io/static/images/avatar/svg/${player.avatar}.svg`;
                            plst.add({ id: player.id, nick: player.nick, foto: player.foto, avatar: player.avatar });
                            this.addPlayer(
                                player.foto,
                                player.nick,
                                roomCode
                            );
                        }

                        // Player left live
                        if (data[0] === "24") {
                            const leavingName = plst.getName(data[1]);
                            plst.remove(data[1]);
                            if (leavingName) this.removePlayer(leavingName, roomCode);
                        }
                    } catch (err) { /* silent */ }
                };

                // Reconnection factory — retry with a DIFFERENT proxy IP on failure
                const makeOnClose = (currentWs, retryCount = 0) => () => {
                    try { currentWs._fvChannel?.close(); currentWs._fvChannel = null; } catch (_) {}
                    const idx = this.activeSockets.indexOf(currentWs);
                    if (idx !== -1) this.activeSockets.splice(idx, 1);
                    if (!this._proxyFails) this._proxyFails = {};
                    this._proxyFails[ip] = (this._proxyFails[ip] || 0) + 1;
                    if (retryCount >= 2) {
                        this.connectedRooms.delete(roomCode);
                        return;
                    }
                    const candidates = this.proxies.filter(p => {
                        const altIp = typeof p === 'object' ? p.ip : p;
                        return altIp !== ip && (this._proxyFails[altIp] || 0) < 3;
                    });
                    const altProxy = candidates.length ? candidates[Math.floor(Math.random() * candidates.length)] : null;
                    if (!altProxy) {
                        this.connectedRooms.delete(roomCode);
                        return;
                    }
                    const altIp = typeof altProxy === 'object' ? altProxy.ip : altProxy;
                    const altRelayUrl = `wss://${location.host}/bird/relay?ip=${encodeURIComponent(altIp)}&target=${encodeURIComponent(targetB64)}`;
                    const delay = 3000 * (retryCount + 1);
                    setTimeout(() => {
                        try {
                            const newWs = new WebSocket(altRelayUrl);
                            newWs._lastFrameAt = Date.now();
                            newWs._roomCode = roomCode;
                            this.activeSockets.push(newWs);
                            newWs.onopen = () => {
                                if (this._proxyFails) this._proxyFails[altIp] = 0;
                                newWs._lastFrameAt = Date.now();
                                newWs._openedAt = Date.now();
                                newWs.send(`42[12,{"v":20000,"platform":0,"sala":"${roomid}"}]`);
                                newWs.send(`42[46,0]`);
                                try {
                                    newWs._fvChannel = new BroadcastChannel('bird-fv:' + roomCode);
                                } catch (err) {
                                    newWs._fvChannel = null;
                                }
                            };
                            newWs.onmessage = onMsg;
                            newWs.onerror = () => {};
                            newWs.onclose = makeOnClose(newWs, retryCount + 1);
                        } catch(e) { /* insufficient resources, stop */ }
                    }, delay);
                };

                websocket.onmessage = onMsg;
                websocket.onerror = () => {};
                websocket.onclose = makeOnClose(websocket);
            });
        }

        /**
         * connectPublicRooms — connects to public rooms via local Node.js relay (localhost:8080)
         * Step 1: POST /startScan with all jobs (avoids URL length limits)
         * Step 2: GET /scanStream as EventSource for results
         */
        connectPublicRooms(langs) {
            const LANG_NAMES = {1:"Portuguese",2:"English",3:"Spanish",4:"French",6:"Italian",7:"Russian",9:"Chinese-TW",10:"Polish",11:"Czech",12:"Thai",13:"Vietnamese",14:"German",15:"Japanese",16:"Chinese-CN",17:"Chinese-HK",18:"Dutch",19:"Arabic",20:"Korean",21:"Bulgarian",22:"Slovak",23:"Azerbaijani",24:"Swedish",25:"Amharic",26:"Afrikaans",27:"Belarusian",28:"Bengali",29:"Bosnian",30:"Catalan",31:"Danish",32:"Greek",33:"Estonian",34:"Persian",35:"Finnish",36:"Faroese",37:"Irish",38:"Galician",39:"Gujarati",40:"Hebrew",41:"Hindi",42:"Armenian",43:"Croatian",44:"Hungarian",45:"Indonesian",46:"Icelandic",47:"Georgian",48:"Khmer",49:"Kazakh",50:"Luxembourgish",51:"Lao",52:"Latvian",53:"Macedonian",54:"Mongolian",55:"Malay",56:"Maltese",57:"Burmese",58:"Romanian",59:"Slovenian",60:"Serbian",61:"Albanian",62:"Turkmen",63:"Ukrainian",64:"Yoruba",65:"Norwegian",66:"Kurdish",67:"Esperanto",68:"Lithuanian"};
            const RELAY = '/bird';

            this.activeEventSources.forEach(es => { try { es.close(); } catch(e){} });
            this.activeEventSources = [];

            const jobs = langs.map((lang, idx) => {
                const proxy = this.proxies[idx % this.proxies.length];
                return {
                    lang: lang,
                    proxyIp: typeof proxy === 'object' ? proxy.ip : proxy,
                    cookie: `__cpc=${typeof proxy === 'object' ? proxy.cookie : ''}`
                };
            });

            console.log(`[Public] Starting scan for ${langs.length} languages via relay...`);

            // Step 1: POST jobs to relay
            fetch(`${RELAY}/startScan`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ jobs, secb: this.secb })
            }).then(r => r.json()).then(data => {
                if (!data.ok) {
                    console.log('[Public] Relay error:', data.error);
                    return;
                }
                console.log(`[Public] Scan queued (${data.langCount} langs), connecting SSE...`);

                // Step 2: Connect SSE stream (tiny URL, no data in query)
                const es = new EventSource(`${RELAY}/scanStream`);
                this.activeEventSources.push(es);

                const roomStores = {};

                es.addEventListener('connected', (e) => {
                    try {
                        const d = JSON.parse(e.data);
                        const langName = LANG_NAMES[d.lang] || `Lang-${d.lang}`;
                        const roomCode = `public-${langName} ${d.roomLabel || 'Room'}`;
                        // Filter out our own bot from the player list
                        const players = (d.players || []).filter(p => p.id !== d.botId);
                        console.log(`[Public] ${roomCode} on ${d.server} — ${players.length} players`);

                        if (!this.playerStores[roomCode]) this.playerStores[roomCode] = this.createPlayerStore();
                        const plst = this.playerStores[roomCode];
                        if (d.roomKey) roomStores[d.roomKey] = { plst, roomCode, botId: d.botId };

                        for (const player of players) {
                            plst.add({ id: player.id, nick: player.nick, foto: player.foto, avatar: player.avatar });
                            this.addPlayer(
                                player.foto ?? `https://gartic.io/static/images/avatar/svg/${player.avatar}.svg`,
                                player.nick, roomCode
                            );
                        }
                        if (this.loadingIndicator) { this.loadingIndicator.remove(); this.loadingIndicator = null; }
                    } catch (err) { console.log('[Public] parse error', err); }
                });

                es.addEventListener('player_joined', (e) => {
                    try {
                        const d = JSON.parse(e.data);
                        const s = d.roomKey && roomStores[d.roomKey]; if (!s) return;
                        if (d.id === s.botId) return; // skip our own bot
                        s.plst.add({ id: d.id, nick: d.nick, foto: d.foto || null, avatar: d.avatar });
                        this.addPlayer(d.foto ?? `https://gartic.io/static/images/avatar/svg/${d.avatar}.svg`, d.nick, s.roomCode);
                    } catch (err) {}
                });

                es.addEventListener('player_left', (e) => {
                    try {
                        const d = JSON.parse(e.data);
                        const s = d.roomKey && roomStores[d.roomKey]; if (!s) return;
                        const n = s.plst.getName(d.id); s.plst.remove(d.id);
                        if (n) this.removePlayer(n, s.roomCode);
                    } catch (err) {}
                });

                es.addEventListener('chat', (e) => {
                    try {
                        const d = JSON.parse(e.data);
                        const s = d.roomKey && roomStores[d.roomKey]; if (!s) return;
                        this.addMessage(s.plst.getName(d.senderId), d.message, s.plst.isAccount(d.senderId), s.roomCode, s.plst.getAvatar(d.senderId));
                    } catch (err) {}
                });

                es.addEventListener('scan_complete', (e) => {
                    try { const d = JSON.parse(e.data); const ln = LANG_NAMES[d.lang] || `Lang-${d.lang}`; console.log(`[Public] ${ln}: ${d.roomCount} rooms`); } catch(e){}
                });

                es.addEventListener('all_done', (e) => {
                    try { const d = JSON.parse(e.data); console.log(`[Public] Done — ${d.totalRooms} rooms / ${d.totalLangs} langs`); } catch(e){}
                });

                es.addEventListener('error', (e) => {
                    if (e.data) { try { console.log('[Public] Error:', JSON.parse(e.data).message); } catch(e){} }
                });

                es.addEventListener('room_disconnected', (e) => {
                    try { const d = JSON.parse(e.data); console.log(`[Public] ${LANG_NAMES[d.lang] || d.lang} ${d.roomLabel}: disconnected`); } catch(e){}
                });

                es.onerror = () => { if (es.readyState === EventSource.CLOSED) console.log('[Public] SSE closed'); };

            }).catch(err => {
                console.log('[Public] Failed to reach relay:', err.message || err);
                console.log('[Public] Is "node public-relay.js" running?');
            });
        }


        async playerSearchGO(languagecodes) {
            if (this._serverSession) return; // server session is authoritative
            try {
                this.monitorRooms.call(this, languagecodes, (res) => {
                    const flatList = [];
                    for (const [lang, arr] of Object.entries(res)) {
                        for (const code of arr) {
                            flatList.push({ language: Number(lang), code });
                        }
                    }

                    if (flatList.length === 0) {
                        if (this.loadingIndicator) {
                            this.loadingIndicator.textContent = "No active rooms found. Try refreshing.";
                        }
                        if (this.isRefreshing) {
                            setTimeout(() => {
                                this.sweepStale();
                                this.isRefreshing = false;
                                this.refreshButton.classList.remove('Bird-refreshing');
                                this.refreshButton.disabled = false;
                            }, 1000);
                        }
                        return;
                    }

                    if (this.loadingIndicator) {
                        this.loadingIndicator.textContent = `Found ${flatList.length} rooms. Connecting...`;
                        setTimeout(() => {
                            if (this.loadingIndicator) {
                                this.loadingIndicator.remove();
                                this.loadingIndicator = null;
                            }
                        }, 3000);
                    }

                    const distributed = {};
                    this.proxies.forEach(p => {
                        const ip = typeof p === 'object' ? p.ip : p;
                        distributed[ip] = [];
                    });
                    let i = 0;
                    for (const item of flatList) {
                        const proxy = this.proxies[i % this.proxies.length];
                        const ip = typeof proxy === 'object' ? proxy.ip : proxy;
                        const cookie = typeof proxy === 'object' ? proxy.cookie : '';
                        distributed[ip].push({ language: item.language, roomcode: item.code, ip, cookie });
                        i++;
                    }

                    const sleep = ms => new Promise(r => setTimeout(r, ms));

                    for (const [ip, queue] of Object.entries(distributed)) {
                        (async () => {
                            for (const job of queue) {
                                const result = await this.checkDeepProxy(job);
                                this.connectToRoom(
                                    { ip: result.info.ip, cookie: result.info.cookie },
                                    result.info.roomcode,
                                    result.response,
                                    result.server
                                );
                                await sleep(2000);
                            }
                        })();
                    }
                });
            } catch (e) {
                console.error("playerSearchGO error:", e);
            }
        }

        // =================================================================
        // Proxy Management
        // =================================================================

        async addNewProxy() {
            // Webshare-only: the local Go server (localhost:8080) is the single
            // source of proxies. There is no fallback — if the server is down
            // or has no healthy Webshare proxies loaded, we surface that clearly
            // so the user can start it. CroxyProxy iframe scraping is gone.
            try {
                this.proxyButton.textContent = "Fetching from server...";
                this._proxyFails = {}; // reset health tracking

                const SERVER = '/bird';
                let data;
                try {
                    const r = await fetch(`${SERVER}/api/proxies`);
                    data = await r.json();
                } catch (e) {
                    console.error('[addNewProxy] Local server unreachable:', e);
                    this.proxyButton.textContent = "Server offline — start it on :8080";
                    setTimeout(() => { this.proxyButton.textContent = `Get Proxy (${this.proxies?.length || 0})`; }, 4000);
                    this.proxyButton.disabled = false;
                    return;
                }

                if (!data?.ok || !data.proxies?.length) {
                    this.proxyButton.textContent = "No Webshare proxies loaded";
                    setTimeout(() => { this.proxyButton.textContent = `Get Proxy (${this.proxies?.length || 0})`; }, 4000);
                    this.proxyButton.disabled = false;
                    return;
                }

                this.proxies = data.proxies.map(p => ({ ip: p.ip, cookie: p.cookie || '' }));
                GM_setValue("Bird-Proxies", this.proxies);
                this.proxyButton.textContent = `Get Proxy (${this.proxies.length})`;
                this.proxyButton.disabled = false;
                this.updateModeButtons();
                console.log(`[addNewProxy] Got ${this.proxies.length} Webshare proxies from server`);
            } catch (error) {
                console.error('[addNewProxy] error:', error);
                this.proxyButton.disabled = false;
                this.updateModeButtons();
                this.proxyButton.textContent = `Get Proxy (${this.proxies?.length || 0})`;
            }
        }

        // initProxies is a no-op now: proxies come exclusively from the local
        // Go server via /api/proxies (Webshare). The historical CroxyProxy
        // bootstrap list and GitHub ProxyModuleList fetch were removed.
        async initProxies() { /* intentionally empty */ }
    }

    // Boot
    (async () => {
        const overlay = document.querySelector('#BirdOverlay');
        new Bird(overlay);
    })();
})();