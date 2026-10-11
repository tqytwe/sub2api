"""Native compiled Vue/Pinia/Axios with held loopback HTTP responses.

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


server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
origin = f'http://127.0.0.1:{server.server_port}'
reports = []
try:
    with sync_playwright() as pw:
        browser = pw.chromium.launch(executable_path='/usr/bin/chromium', headless=True,
                                     args=['--no-sandbox', '--disable-dev-shm-usage'])
        for kind in ['login', 'login2FA', 'register']:
            for transition in ['account', 'same-account', 'logout']:
                for outcome in [200, 401, 503]:
                    release.clear(); received.clear(); state.clear()
                    state.update(outcome=outcome, tokens={})
                    width = 360 if transition == 'same-account' else 1280
                    context = browser.new_context(viewport=dict(width=width, height=900))
                    context.route('**/*', lambda route: route.continue_() if urlparse(route.request.url).netloc == urlparse(origin).netloc else route.abort())
                    context.add_init_script('window.__APP_CONFIG__=' + json.dumps(SETTINGS))
                    page = context.new_page(); errors = []
                    page.on('pageerror', lambda e: errors.append(str(e)))
                    page.goto(origin + '/login?lang=zh', wait_until='networkidle')
                    page.wait_for_function("document.querySelector('#app')?.__vue_app__?.config.globalProperties.$pinia?._s.has('auth') === true")
                    call(page, 'login', '1-initial')
                    initial = snapshot(page)
                    assert initial['userID'] == 1 and initial['authenticated']
                    call(page, kind, '1-old', hold=True)
                    deadline = time.monotonic() + 5
                    while not received.is_set() and time.monotonic() < deadline:
                        page.wait_for_timeout(20)  # Pump Playwright routing callbacks while HTTP starts.
                    assert received.is_set(), 'Old HTTP request was not observed'
                    if transition == 'logout':
                        page.evaluate(f'async () => {{ await ({AUTH}).logout(); }}')
                    else:
                        call(page, 'login', '2-current' if transition == 'account' else '1-current')
                    current = snapshot(page)
                    expected_owner = None if transition == 'logout' else 2 if transition == 'account' else 1
                    assert current['userID'] == expected_owner
                    if transition == 'logout':
                        assert not current['authenticated'] and not current['token']
                        assert all(current['stored'][key] is None for key in ['auth_token', 'refresh_token', 'token_expires_at', 'auth_user'])
                    else:
                        expected_label = f'{expected_owner}-current'
                        assert current['authenticated']
                        assert current['token'] == current['stored']['auth_token'] == f'fixture-access-{expected_label}'
                        assert current['stored']['refresh_token'] == f'fixture-refresh-{expected_label}'
                        assert json.loads(current['stored']['auth_user'])['id'] == expected_owner
                    assert current['stored']['auth_session_id'] != initial['stored']['auth_session_id']
                    assert not (current['stored']['auth_session_id'] or '').startswith('pending:')
                    release.set()
                    page.wait_for_function('window.oldDone === true')
                    assert page.evaluate('Boolean(window.oldError)'), (kind, transition, outcome)
                    assert snapshot(page) == current, (kind, transition, outcome)
                    assert not errors, errors
                    assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
                    if outcome == 401:
                        page.screenshot(path=str(OUT / f'{kind}-{transition}-{width}.png'), full_page=True, animations='disabled')
                    reports.append(dict(kind=kind, transition=transition, old_http_status=outcome,
                                        viewport=width, old_rejected=True, session_unchanged=True,
                                        expected_userID=expected_owner, transition_verified=True,
                                        js_errors=errors, overflow=False, manual_restore=False))
                    context.unroute_all(behavior='wait')
                    context.close()
        browser.close()
finally:
    release.set()
    server.shutdown()
report = dict(dist_index_sha256=hashlib.sha256((DIST / 'index.html').read_bytes()).hexdigest(),
              scope='Native built store actions over loopback HTTP; no form/TOTP/JWT/production acceptance', cases=reports)
(OUT / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(dict(passed=len(reports), dist_index_sha256=report['dist_index_sha256'])))
