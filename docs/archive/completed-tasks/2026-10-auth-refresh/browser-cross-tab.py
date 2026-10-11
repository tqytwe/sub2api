"""Actual shared-context browser tabs and loopback HTTP; all accounts are fixtures.

Observe base/head separately before changing production source. No manual checkAuth,
store restoration or simulated storage events after either tab starts.
"""
import argparse
import hashlib
import json
import threading
import time
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright, expect

parser = argparse.ArgumentParser()
parser.add_argument('--dist', required=True)
parser.add_argument('--out', required=True)
parser.add_argument('--cases', default='reading,peer-reading,account,account-no-event,logout,relogin,pending-account,pending-account-no-event')
parser.add_argument('--width', type=int, default=1280)
parser.add_argument('--theme', default='light')
parser.add_argument('--language', default='zh')
parser.add_argument('--assert-safe', action='store_true')
parser.add_argument('--trace-locks', action='store_true')
parser.add_argument('--login-write-gap', action='store_true', help='Amplify the native login write gap and release the old HTTP response within it')
args = parser.parse_args()
DIST, OUT = Path(args.dist).resolve(), Path(args.out).resolve()
OUT.mkdir(parents=True, exist_ok=True)
AUTH = "document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth')"
SETTINGS = dict(site_name='极速蹬 · TEST FIXTURE', play_quiz_enabled=True, play_blindbox_enabled=True, registration_enabled=True)
state = {}
release = threading.Event()


def user(owner):
    return dict(id=owner, email=f'fixture-{owner}@example.test', username=f'Fixture {owner}', role='user', status='active', balance=owner, concurrency=5)


def pair(owner, suffix, lifetime=3600):
    access, refresh = f'fixture-{owner}-access-{suffix}', f'fixture-{owner}-refresh-{suffix}'
    state['tokens'][access] = owner
    state['refreshes'][refresh] = owner
    return dict(access_token=access, refresh_token=refresh, expires_in=lifetime, token_type='Bearer')


class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=str(DIST), **kw)

    def log_message(self, *a):
        pass

    def reply(self, status, data):
        raw = json.dumps(dict(code=0 if status == 200 else status, data=data)).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(raw)))
        self.end_headers()
        try:
            self.wfile.write(raw)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def owner(self):
        return state['tokens'].get(self.headers.get('Authorization', '').removeprefix('Bearer '))

    def do_GET(self):
        path = urlparse(self.path).path
        if path.startswith('/api/'):
            if path.endswith('/auth/me'):
                state['profiles'].append(self.owner())
                return self.reply(200 if self.owner() else 401, user(self.owner()))
            if path.endswith('/quiz/today'):
                state['quiz_reads'] += 1
                data = dict(enabled=True, coupon_pool_ready=True, already_submitted=False,
                            server_date='2026-10-11', reward_per_correct=.1,
                            questions=[dict(id=i, prompt=f'Fixture question {i}', options=['A', 'B']) for i in [1, 2]])
            elif path.endswith('/blindbox/status'):
                pool = dict(version='cross-tab-fixture', cost=.5, rtp_cap=.9, tiers=[dict(amount=.2, weight=10000)])
                data = dict(enabled=True, coupon_pool_ready=True, coupon_prizes=[], cost_amount=.5, pool=pool,
                            current_pool=pool, coupon_weight_bp=0, redeem_code_weight_bp=0, balance_weight_bp=10000,
                            pool_version='cross-tab-fixture', daily_limit=2, effective_limit=2, opens_today=0,
                            can_open=True, growth_eligibility=dict(reward_mode='redeemable'),
                            growth_governance_available=True, server_date='2026-10-11')
            elif path.endswith('/settings/public'):
                data = SETTINGS
            elif path.endswith('/blindbox/recent') or 'subscriptions' in path:
                data = []
            elif 'announcements' in path:
                data = dict(items=[], total=0)
            else:
                data = {}
            return self.reply(200, data)
        if not Path(self.translate_path(self.path)).is_file():
            self.path = '/index.html'
        super().do_GET()

    def do_POST(self):
        raw = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        body = json.loads(raw) if raw else {}
        if self.path.endswith('/auth/login'):
            owner = 2 if body['email'] == 'fixture-2@example.test' else 1
            state['logins'] += 1
            if args.login_write_gap and state['logins'] == 2:
                threading.Timer(.3, release.set).start()
            return self.reply(200, dict(pair(owner, f"login-{state['logins']}", state['lifetime']), user=user(owner)))
        if self.path.endswith('/auth/logout'):
            return self.reply(200, {})
        if self.path.endswith('/auth/refresh'):
            state['renewals'] += 1
            state['refresh_requests'].append(dict(token=body['refresh_token'], source='api-client' if self.headers.get('Authorization') else 'coordinator'))
            owner = state['refreshes'].get(body['refresh_token'])
            return self.reply(200, pair(owner, f"renewed-{state['renewals']}"))
        if self.path.endswith('/blindbox/open'):
            owner = self.owner()
            state['posts'].append(dict(owner=owner, key=self.headers.get('Idempotency-Key')))
            if state['hold']:
                release.wait(30)
            return self.reply(200 if owner else 401, dict(cost_amount=.5, reward_amount=.2, net_amount=-.3,
                               reward_type='balance', opens_today=1, pool_version='cross-tab-fixture', server_date='2026-10-11'))
        self.reply(404, {})


def snapshot(page):
    page.wait_for_function("document.querySelector('#app')?.__vue_app__?.config.globalProperties.$pinia._s.get('auth')")
    return page.evaluate('({user:(' + AUTH + ')?.user?.id ?? null, token:(' + AUTH + ')?.token ?? null,'
                         'storedUser:JSON.parse(localStorage.getItem("auth_user") || "null")?.id ?? null,'
                         'storedToken:localStorage.getItem("auth_token")})')


def login(page, owner):
    page.wait_for_function("document.querySelector('#app')?.__vue_app__?.config.globalProperties.$pinia._s.get('auth')")
    page.evaluate('async owner => { await (' + AUTH + ').login({email:`fixture-${owner}@example.test`,password:"synthetic-fixture-only"}) }', owner)


server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
origin = f'http://127.0.0.1:{server.server_port}'
reports = []
try:
    with sync_playwright() as pw:
        browser = pw.chromium.launch(executable_path='/usr/bin/chromium', headless=True, args=['--no-sandbox', '--disable-dev-shm-usage'])
        for case in args.cases.split(','):
            pending_case = case == 'relogin' or case.startswith('pending-account')
            state.clear()
            state.update(tokens={}, refreshes={}, profiles=[], logins=0, renewals=0, refresh_requests=[], posts=[], quiz_reads=0,
                         lifetime=135 if case == 'peer-reading' else 126 if case == 'reading' else 3600, hold=pending_case)
            release.clear()
            errors = []
            context = browser.new_context(viewport=dict(width=args.width, height=900), reduced_motion='reduce')
            context.on('page', lambda page: page.on('pageerror', lambda error: errors.append(str(error))))
            context.route('**/*', lambda route: route.continue_() if urlparse(route.request.url).netloc == urlparse(origin).netloc else route.abort())
            context.add_init_script('window.__APP_CONFIG__=' + json.dumps(SETTINGS) +
                                    ';if(location.origin===' + json.dumps(origin) + ')localStorage.setItem("theme",' + json.dumps(args.theme) + ')')
            if args.trace_locks:
                # Diagnostics delegate native locking and preserve the callback Promise.
                # This option is excluded from final uninstrumented acceptance runs.
                context.add_init_script("""(() => {
                  if (!navigator.locks) return;
                  window.__lockTrace = [];
                  const record = (event, extra = {}) => window.__lockTrace.push({
                    event, at: Date.now(), access: localStorage.getItem('auth_token'),
                    refresh: localStorage.getItem('refresh_token'),
                    expiry: localStorage.getItem('token_expires_at'),
                    session: localStorage.getItem('auth_session_id'), ...extra
                  });
                  const original = navigator.locks.request.bind(navigator.locks);
                  navigator.locks.request = (name, ...args) => {
                    record('request', {name});
                    const callback = args.pop();
                    return original(name, ...args, async lock => {
                      record('acquire', {name});
                      try { return await callback(lock); }
                      finally { record('release', {name}); }
                    });
                  };
                  addEventListener('storage', event => record('storage', {key: event.key}));
                })()""")
            a = context.new_page()
            if case.endswith('-no-event'):
                a.add_init_script("window.addEventListener('storage', event => event.stopImmediatePropagation(), true)")
            a.goto(origin + '/login', wait_until='networkidle')
            login(a, 1)
            a.goto(origin + ('/quiz-quest' if case in ['reading', 'peer-reading'] else '/blindbox') + '?lang=' + args.language, wait_until='networkidle')
            initial = snapshot(a)
            report = dict(case=case, initial=initial, width=args.width, theme=args.theme, language=args.language,
                          web_locks=a.evaluate('({secure:isSecureContext, available:!!navigator.locks, request:String(navigator.locks?.request)})'))
            if case in ['reading', 'peer-reading']:
                expect(a.locator('.quiz-question-card')).to_have_count(2)
                for question in a.locator('.quiz-question-card').all():
                    question.locator('label').first.click()
                a.screenshot(path=str(OUT / 'reading-before.png'), full_page=True)
                observer = a
                if case == 'peer-reading':
                    # Both tabs use native timers/Web Locks; whichever wins, the peer
                    # must retain its own selected answers without manual restoration.
                    observer = context.new_page()
                    observer.goto(origin + '/quiz-quest?lang=' + args.language, wait_until='networkidle')
                    expect(observer.locator('.quiz-question-card')).to_have_count(2)
                    for question in observer.locator('.quiz-question-card').all():
                        question.locator('label').first.click()
                observer.wait_for_function('localStorage.getItem("auth_token").includes("renewed")', timeout=20000)
                a.wait_for_timeout(300)
                report.update(selected=a.locator('input:checked').count(), after=snapshot(a), quiz_reads=state['quiz_reads'])
                report['safe'] = report['selected'] == 2 and report['after']['token'] == report['after']['storedToken']
                if case == 'peer-reading':
                    report['peer_selected'] = observer.locator('input:checked').count()
                    report['peer_after'] = snapshot(observer)
                    report['renewals'] = state['renewals']
                    report['refresh_requests'] = state['refresh_requests'].copy()
                    if args.trace_locks:
                        report['lock_traces'] = [p.evaluate('window.__lockTrace') for p in [a, observer]]
                    report['safe'] = report['safe'] and report['peer_selected'] == 2 and report['peer_after']['token'] == report['peer_after']['storedToken'] and report['renewals'] == 1
                a.screenshot(path=str(OUT / 'reading-after.png'), full_page=True)
            else:
                b = context.new_page()
                b.goto(origin + '/blindbox', wait_until='networkidle')
                snapshot(b)
                if pending_case:
                    a.locator('.play-action-panel button.play-btn-primary').click()
                    deadline = time.monotonic() + 5
                    while not state['posts'] and time.monotonic() < deadline:
                        a.wait_for_timeout(50)
                    assert len(state['posts']) == 1
                    report['pending_key_before'] = a.evaluate('sessionStorage.getItem("blindbox.pending-open:1")')
                if args.login_write_gap:
                    # Preserve native storage writes/events; widen the proven cross-renderer
                    # scheduling window so an old real HTTP result arrives before commit.
                    b.evaluate('''() => { const original=Storage.prototype.setItem;
                      Storage.prototype.setItem=function(key,value) {
                        const result=original.call(this,key,value);
                        if(key==='auth_token' && String(value).includes('login-2')) {
                          const until=performance.now()+1500; while(performance.now()<until) {}
                        }
                        return result;
                      }; }''')
                if case != 'relogin':
                    b.evaluate('(' + AUTH + ').logout()')
                if case != 'logout':
                    login(b, 2 if 'account' in case else 1)
                a.wait_for_timeout(300)
                report['immediate'] = snapshot(a)
                print(json.dumps(dict(phase='peer-transition', **report)), flush=True)
                if case == 'account':
                    # Observe the real existing 60s profile timer, without manually repairing auth.
                    a.wait_for_timeout(61000)
                    report['after_timer'] = snapshot(a)
                if pending_case:
                    release.set()
                    a.wait_for_timeout(800)
                    report['old_result_visible'] = a.get_by_role('dialog').count() > 0
                    report['pending_key_after'] = a.evaluate('sessionStorage.getItem("blindbox.pending-open:1")')
                    report['posts'] = state['posts'].copy()
                    report['safe'] = not report['old_result_visible'] and report['pending_key_after'] == report['pending_key_before'] and len(report['posts']) == 1
                else:
                    report['before_new_request'] = snapshot(a)
                    button = a.locator('.play-action-panel button.play-btn-primary')
                    if button.count() and button.is_enabled():
                        button.click()
                        a.wait_for_timeout(500)
                    report['posts'] = state['posts'].copy()
                    report['safe'] = not report['posts'] or (case.startswith('account') and report['before_new_request']['user'] == 2 and all(p['owner'] == 2 for p in report['posts']))
                a.screenshot(path=str(OUT / f'{case}-after.png'), full_page=True)
            report['profiles'] = state['profiles'].copy()
            report['js_errors'] = errors.copy()
            report['overflow'] = a.evaluate('document.documentElement.scrollWidth > innerWidth')
            report['safe'] = report['safe'] and not errors and not report['overflow']
            reports.append(report)
            (OUT / 'report.json').write_text(json.dumps(dict(index_sha256=hashlib.sha256((DIST / 'index.html').read_bytes()).hexdigest(), reports=reports), indent=2) + '\n')
            print(json.dumps(report), flush=True)
            context.close()
        browser.close()
finally:
    release.set()
    server.shutdown()
result = dict(index_sha256=hashlib.sha256((DIST / 'index.html').read_bytes()).hexdigest(), reports=reports)
(OUT / 'report.json').write_text(json.dumps(result, indent=2) + '\n')
if args.assert_safe:
    assert all(r['safe'] for r in reports), 'Native cross-tab/answer-preservation contract failed'
