"""Native auth rotation in actual built Vue resources, using loopback HTTP fixtures.

Only initial session bootstrap writes fixture credentials. No post-refresh checkAuth,
reload, manually assigned store token or manual balance refresh is used.
This is frontend evidence, not production JWT or browser-to-PG acceptance.
"""
import argparse
import hashlib
import json
import threading
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[5]
OUT = ROOT / 'docs/archive/completed-tasks/2026-10-interactive-auth/root-storage/browser-refresh-evidence'
OUT.mkdir(parents=True, exist_ok=True)
parser = argparse.ArgumentParser()
parser.add_argument('--dist', required=True)
args = parser.parse_args()
DIST = Path(args.dist).resolve()
assert (DIST / 'index.html').is_file()
SETTINGS = dict(site_name='极速蹬 · TEST FIXTURE', play_quiz_enabled=True, play_blindbox_enabled=True, registration_enabled=True)
USER = dict(id=50, email='fixture@example.test', username='Fixture', role='user', status='active', balance=1, concurrency=5)
state = {}
release_post = threading.Event()
AUTH = "document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth')"


def quiz():
    return dict(enabled=True, coupon_pool_ready=True, already_submitted=state['commits'] > 0,
                server_date='2026-10-11', reward_per_correct=.1,
                questions=[] if state['commits'] else [dict(id=i, prompt=f'Fixture question {i}', options=['A', 'B']) for i in [1, 2]],
                previous_score=2, previous_total=2, previous_reward=.2, previous_reward_type='balance')


def blindbox():
    pool = dict(version='auth-fixture', cost=.5, rtp_cap=.9, tiers=[dict(amount=.2, weight=10000)])
    return dict(enabled=True, coupon_pool_ready=True, coupon_prizes=[], cost_amount=.5, pool=pool,
                current_pool=pool, coupon_weight_bp=0, redeem_code_weight_bp=0, balance_weight_bp=10000,
                pool_version='auth-fixture', daily_limit=2, effective_limit=2, opens_today=state['commits'],
                can_open=state['commits'] < 2, growth_eligibility=dict(reward_mode='redeemable'),
                growth_governance_available=True, server_date='2026-10-11')


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

    def do_GET(self):
        path = urlparse(self.path).path
        if path.startswith('/api/'):
            if path.endswith('/quiz/today'):
                data = quiz()
            elif path.endswith('/blindbox/status'):
                data = blindbox()
            elif path.endswith('/blindbox/recent') or 'subscriptions' in path:
                data = []
            elif path.endswith('/auth/me'):
                data = dict(USER, balance=state['balance'])
            elif path.endswith('/settings/public'):
                data = SETTINGS
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
        if self.path.endswith('/auth/refresh'):
            assert body['refresh_token'] == 'fixture-refresh-initial'
            state['refreshes'] += 1
            return self.reply(200, dict(access_token='fixture-renewed', refresh_token='fixture-refresh-renewed', expires_in=3600, token_type='Bearer'))
        if self.path.endswith('/auth/logout'):
            return self.reply(200, {})
        if self.path.endswith('/quiz/submit') or self.path.endswith('/blindbox/open'):
            state['posts'] += 1
            state['keys'].append(self.headers.get('Idempotency-Key'))
            if state['mode'] == '401' and self.headers.get('Authorization') == 'Bearer fixture-initial':
                return self.reply(401, {})
            assert self.headers.get('Authorization') in ['Bearer fixture-initial', 'Bearer fixture-renewed']
            if state['mode'] == 'proactive':
                assert release_post.wait(12), 'Browser did not release the pending operation'
            state['commits'] += 1
            if self.path.endswith('/quiz/submit'):
                state['balance'] = 1.2
                return self.reply(200, dict(score=2, total=2, reward_amount=.2, reward_type='balance'))
            state['balance'] = .7
            return self.reply(200, dict(cost_amount=.5, reward_amount=.2, net_amount=-.3, reward_type='balance',
                                       opens_today=1, pool_version='auth-fixture', server_date='2026-10-11'))
        self.reply(404, {})


server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()
origin = f'http://127.0.0.1:{server.server_port}'
reports = []
try:
    with sync_playwright() as pw:
        browser = pw.chromium.launch(executable_path='/usr/bin/chromium', headless=True, args=['--no-sandbox', '--disable-dev-shm-usage'])
        for game, mode, width, theme, language in [
            ('quiz', 'proactive', 360, 'dark', 'zh'), ('quiz', '401', 1280, 'light', 'en'),
            ('blindbox', 'proactive', 1280, 'light', 'zh'), ('blindbox', '401', 360, 'dark', 'en'),
        ]:
            state.clear(); state.update(mode=mode, balance=1, posts=0, commits=0, refreshes=0, keys=[])
            release_post.clear()
            context = browser.new_context(viewport=dict(width=width, height=900), reduced_motion='reduce')
            context.route('**/*', lambda route: route.continue_() if urlparse(route.request.url).netloc == urlparse(origin).netloc else route.abort())
            expiry = 125000 if mode == 'proactive' else 3600000
            context.add_init_script('window.__APP_CONFIG__=' + json.dumps(SETTINGS) + ';' +
                                    'localStorage.setItem("theme",' + json.dumps(theme) + ');' +
                                    'localStorage.setItem("auth_user",JSON.stringify(' + json.dumps(USER) + '));' +
                                    'localStorage.setItem("auth_token","fixture-initial");' +
                                    'localStorage.setItem("refresh_token","fixture-refresh-initial");' +
                                    f'localStorage.setItem("token_expires_at",String(Date.now()+{expiry}));')
            page = context.new_page(); errors = []; page.on('pageerror', lambda error: errors.append(str(error)))
            path = '/quiz-quest' if game == 'quiz' else '/blindbox'
            page.goto(origin + path + '?lang=' + language, wait_until='networkidle')
            if game == 'quiz':
                expect(page.locator('.quiz-question-card')).to_have_count(2)
                expect(page.locator('.play-page')).to_have_attribute('aria-busy', 'false')
                for question in page.locator('.quiz-question-card').all():
                    question.locator('label').first.click()
                button = page.locator('button.play-btn-primary')
            else:
                button = page.locator('.play-action-panel button.play-btn-primary')
            expect(button).to_be_enabled()
            assert state['refreshes'] == 0
            button.click()
            if mode == 'proactive':
                expect(button).to_be_disabled()
                page.wait_for_function(AUTH + '.token === "fixture-renewed"', timeout=8000)
                assert state['posts'] == 1 and state['commits'] == 0
                release_post.set()
            expected_balance = 1.2 if game == 'quiz' else .7
            page.wait_for_function('(' + AUTH + ').user.balance === ' + str(expected_balance))
            assert page.evaluate(AUTH + '.token === localStorage.getItem("auth_token") && ' + AUTH + '.token === "fixture-renewed"')
            assert state['refreshes'] == 1 and state['commits'] == 1
            assert state['posts'] == (1 if mode == 'proactive' else 2)
            if game == 'quiz':
                expect(page.locator('.quiz-question-card')).to_have_count(0)
                expect(page.locator('.play-page')).to_have_attribute('aria-busy', 'false')
            else:
                expect(page.get_by_role('dialog')).to_be_visible()
                assert page.evaluate('sessionStorage.getItem("blindbox.pending-open:50")') is None
                if mode == '401':
                    assert state['keys'][0] and state['keys'][0] == state['keys'][1]
            expect(page.get_by_role('banner')).to_contain_text(f'${expected_balance:.2f}')
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
            assert not errors, errors
            page.screenshot(path=str(OUT / f'{game}-{mode}-{width}.png'), full_page=True, animations='disabled')
            reports.append(dict(game=game, mode=mode, viewport=width, theme=theme, language=language,
                                refreshes=state['refreshes'], posts=state['posts'], fixture_commits=state['commits'],
                                pinia_matches_storage=True, displayed_balance=expected_balance, manual_restore=False,
                                js_errors=errors, overflow=False))
            context.close()
        browser.close()
finally:
    release_post.set()
    server.shutdown()
report = dict(reports=reports, index_sha256=hashlib.sha256((DIST / 'index.html').read_bytes()).hexdigest())
(OUT / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(report, ensure_ascii=False))
