"""Native compiled Vue/Pinia/Axios with one-time Storage exceptions.

Uses only synthetic credentials. No storage seeding, manual restore, mocked
Axios errors or production calls. Store actions enter the actual built code;
this is session adoption evidence, not form/TOTP/JWT or production acceptance.
"""
import argparse
import hashlib
import json
import threading
import time
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright

parser = argparse.ArgumentParser()
parser.add_argument('--dist', required=True)
parser.add_argument('--out', required=True)
parser.add_argument('--assert-clean', action='store_true')
args = parser.parse_args()
DIST, OUT = Path(args.dist).resolve(), Path(args.out).resolve()
OUT.mkdir(parents=True, exist_ok=True)
AUTH = "document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth')"
SETTINGS = dict(site_name='极速蹬 · TEST FIXTURE', registration_enabled=True)
release, received = threading.Event(), threading.Event()
state = {}


def user(owner):
    return dict(id=owner, email=f'fixture-{owner}@example.test', username=f'Fixture {owner}',
                role='user', status='active', balance=owner, concurrency=5)


class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=str(DIST), **kw)

    def log_message(self, *a):
        pass

    def reply(self, status, data):
        raw = json.dumps(dict(code=0 if status == 200 else status, data=data,
                              message=f'fixture-error-{status}')).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(raw)))
        self.end_headers()
        try:
            self.wfile.write(raw)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def do_GET(self):
        path = urlparse(self.path).path
        if path.startswith('/api/'):
            if path.endswith('/settings/public'):
                data = SETTINGS
            elif path.endswith('/auth/me'):
                data = user(state['tokens'].get(self.headers.get('Authorization', '').removeprefix('Bearer '), 1))
            elif 'announcements' in path:
                data = dict(items=[], total=0)
            elif 'subscriptions' in path:
                data = []
            else:
                data = {}
            return self.reply(200, data)
        if not Path(self.translate_path(self.path)).is_file():
            self.path = '/index.html'
        super().do_GET()

    def do_POST(self):
        raw = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        body = json.loads(raw) if raw else {}
        if self.path.endswith(('/auth/login', '/auth/login/2fa', '/auth/register')):
            label = str(body.get('email', body.get('temp_token', ''))).split('@')[0]
            status = 200
            if label == '1-old':
                received.set()
                if not release.wait(15):
                    return self.reply(504, {})
                status = state['outcome']
            owner = int(label.split('-')[0])
            access = f'fixture-access-{label}'
            state['tokens'][access] = owner
            return self.reply(status, dict(user=user(owner), access_token=access,
                              refresh_token=f'fixture-refresh-{label}', expires_in=3600, token_type='Bearer'))
        return self.reply(200, {})


def call(page, kind, label, hold=False):
    expr = (f"auth.login2FA({json.dumps(label)}, '123456')" if kind == 'login2FA' else
            f"auth.{kind}({{email:{json.dumps(label + '@example.test')},password:'synthetic-fixture-only'}})")
    if hold:
        page.evaluate(f"() => {{const auth={AUTH}; window.oldDone=false; window.oldError=null; "
                      f"{expr}.then(()=>{{window.oldDone=true}},e=>{{window.oldError=e.code||e.status||e.message;window.oldDone=true}});}}")
    else:
        page.evaluate(f"async () => {{const auth={AUTH}; await {expr};}}")


def snapshot(page):
    return page.evaluate(f"() => {{const auth={AUTH};return {{token:auth.token,userID:auth.user?.id??null,"
                         "authenticated:auth.isAuthenticated,stored:Object.fromEntries(['auth_token','refresh_token',"
                         "'token_expires_at','auth_user','auth_session_id'].map(k=>[k,localStorage.getItem(k)]))}}")


def attempt(page, kind, label):
    expr = (f"auth.login2FA({json.dumps(label)}, '123456')" if kind == 'login2FA' else
            f"auth.{kind}({{email:{json.dumps(label + '@example.test')},password:'synthetic-fixture-only'}})")
    return page.evaluate(f"async () => {{const auth={AUTH}; try {{await {expr};return null}}"
                         "catch(e){return {name:e.name,message:e.message,code:e.code}}}")


def clean(s):
    return (not s['authenticated'] and s['token'] is None and s['userID'] is None and
            all(s['stored'][k] is None for k in ['auth_token', 'refresh_token', 'token_expires_at', 'auth_user']) and
            not (s['stored']['auth_session_id'] or '').startswith('pending:'))


def recovered(s):
    return (s['authenticated'] and s['userID'] == 3 and s['token'] == 'fixture-access-3-recovery' and
            s['stored']['auth_token'] == s['token'] and s['stored']['refresh_token'] == 'fixture-refresh-3-recovery' and
            json.loads(s['stored']['auth_user'] or '{}').get('id') == 3 and
            not (s['stored']['auth_session_id'] or '').startswith('pending:'))


server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
origin = f'http://127.0.0.1:{server.server_port}'
reports = []
try:
    with sync_playwright() as pw:
        browser = pw.chromium.launch(executable_path='/usr/bin/chromium', headless=True,
                                     args=['--no-sandbox', '--disable-dev-shm-usage'])
        for kind in ['login', 'login2FA', 'register']:
            for phase in ['api', 'store', 'fresh']:
                state.clear(); state.update(outcome=200, tokens={})
                width = 360 if phase == 'store' else 1280
                context = browser.new_context(viewport=dict(width=width, height=900))
                context.route('**/*', lambda route: route.continue_() if urlparse(route.request.url).netloc == urlparse(origin).netloc else route.abort())
                context.add_init_script('window.__APP_CONFIG__=' + json.dumps(SETTINGS))
                page = context.new_page(); errors = []; loads = []
                page.on('pageerror', lambda e: errors.append(str(e)))
                page.on('load', lambda: loads.append(page.url))
                page.goto(origin + '/login?lang=zh', wait_until='networkidle')
                page.wait_for_function("document.querySelector('#app')?.__vue_app__?.config.globalProperties.$pinia?._s.has('auth') === true")
                if phase != 'fresh':
                    call(page, 'login', '1-initial')
                    assert snapshot(page)['userID'] == 1
                page.evaluate('''nth => {
                  const original=Storage.prototype.setItem;
                  let seen=0; window.fixtureFaults=0;
                  Storage.prototype.setItem=function(key,value){
                    if(this===localStorage && key==='auth_user' && ++seen===nth){
                      window.fixtureFaults++;
                      throw new DOMException('one-time fixture Storage failure','QuotaExceededError');
                    }
                    return original.call(this,key,value);
                  };
                }''', 2 if phase == 'store' else 1)
                error = attempt(page, kind, '2-failed')
                after_failure = snapshot(page)
                faults = page.evaluate('window.fixtureFaults')
                assert faults == 1 and error and error['name'] == 'QuotaExceededError'
                page.screenshot(path=str(OUT / f'{kind}-{phase}-failure.png'), full_page=True, animations='disabled')
                # Leave the one-time throwing wrapper installed during recovery.
                # No manual checkAuth, storage restoration or reload is performed.
                recovery_error = attempt(page, 'login', '3-recovery')
                page.wait_for_timeout(500)
                page.wait_for_function("document.querySelector('#app')?.__vue_app__?.config.globalProperties.$pinia?._s.has('auth') === true")
                after_recovery = snapshot(page)
                page.screenshot(path=str(OUT / f'{kind}-{phase}-recovery.png'), full_page=True, animations='disabled')
                overflow = page.evaluate('document.documentElement.scrollWidth > innerWidth')
                record = dict(kind=kind, phase=phase, viewport=width, fault_count=faults,
                              error=error, after_failure=after_failure, cleanup_correct=clean(after_failure),
                              recovery_error=recovery_error, after_recovery=after_recovery,
                              recovery_correct=recovered(after_recovery), page_loads=len(loads),
                              js_errors=errors, overflow=overflow, manual_restore=False)
                reports.append(record)
                print(json.dumps(record, ensure_ascii=False), flush=True)
                context.unroute_all(behavior='wait'); context.close()
        browser.close()
finally:
    release.set(); server.shutdown()
report = dict(dist_index_sha256=hashlib.sha256((DIST / 'index.html').read_bytes()).hexdigest(),
              scope='Actual built store actions, native Storage wrapper throwing exactly once and loopback HTTP; no production/TOTP/form acceptance', cases=reports)
(OUT / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(dict(total=len(reports), cleanup_pass=sum(r['cleanup_correct'] for r in reports),
                     recovery_pass=sum(r['recovery_correct'] for r in reports), index_sha256=report['dist_index_sha256'])))
if args.assert_clean:
    assert all(r['cleanup_correct'] and r['recovery_correct'] and not r['recovery_error'] and
               not r['js_errors'] and not r['overflow'] for r in reports), 'Storage failure/recovery invariant violated'
