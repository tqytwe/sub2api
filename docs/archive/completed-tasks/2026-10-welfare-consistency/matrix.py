"""Offline browser fixture: real production bundles, synthetic API data, no production requests."""
import argparse,json,threading
from pathlib import Path
from http.server import SimpleHTTPRequestHandler,ThreadingHTTPServer
from urllib.parse import urlparse,parse_qs
from playwright.sync_api import sync_playwright
p=argparse.ArgumentParser();p.add_argument('--dist',default=str(Path(__file__).resolve().parents[4] / 'backend/internal/web/dist'));p.add_argument('--phase',default='baseline');a=p.parse_args()
out=Path(__file__).resolve().parents[4] / 'docs/visual-reviews/assets/welfare-consistency';out.mkdir(parents=True,exist_ok=True)
user={'id':50,'email':'fixture@example.test','username':'Fixture user','role':'user','status':'active','balance':42.3,'concurrency':5,'is_email_verified':True}
settings={'site_name':'极速蹬 · TEST FIXTURE','site_logo':'','registration_enabled':True,'email_verify_enabled':False,'payment_enabled':True,'affiliate_enabled':True,**{f'play_{k}_enabled':True for k in ['checkin','arena','blindbox','quiz','agent_team']},'image_studio_enabled':False,'nextchat_enabled':False}
pool={'version':'fixture-v1','cost':.5,'rtp_cap':.9,'tiers':[{'amount':.05,'weight':4000},{'amount':.2,'weight':3000},{'amount':.5,'weight':1800},{'amount':1,'weight':800},{'amount':3,'weight':300},{'amount':10,'weight':90},{'amount':20,'weight':10}]}
blind={'enabled':True,'coupon_pool_ready':True,'coupon_weight_bp':3000,'redeem_code_weight_bp':3000,'balance_weight_bp':4000,'coupon_prizes':[{'template_id':1,'name':'Fixture coupon','weight_bp':10000,'tier':'standard'}],'cost_amount':.5,'pool':pool,'current_pool':pool,'pool_version':'fixture-v1','rtp_cap':.9,'daily_limit':3,'effective_limit':3,'opens_today':0,'can_open':True,'server_date':'2026-10-08'}
quests={'enabled':True,'energy':30,'level':2,'energy_to_next_level':70,'server_date':'2026-10-08','tasks':[{'key':'api_call','completed':True,'energy':20}]}
campaign={'campaign':{'id':1,'key':'fixture','name':'Fixture invitation campaign','status':'running','version':1,'registration_from':'2026-10-01T00:00:00Z','registration_to':'2026-10-31T00:00:00Z','starts_at':'2026-10-01T00:00:00Z','ends_at':'2026-10-31T00:00:00Z','qualification_to':'2026-11-02T00:00:00Z','claim_deadline':'2026-11-07T00:00:00Z','pay_threshold':100,'usage_threshold':20,'max_enrollments':100,'risk_hold_hours':48,'reward_mode':'additive','public_rules_md':'Fixture rules — not a live campaign.','invitee_notice_md':'','legacy_rebate_policy':'stack','rules_version':1,'rules_updated_at':'2026-10-01T00:00:00Z'},'enrollment':{'campaign_id':1,'user_id':50,'enrolled_at':'2026-10-01T00:00:00Z'},'tiers':[{'tier':1,'required_invites':3,'reward_amount':25,'currency':'CNY'}],'invited_count':5,'qualified_count':3,'rewards':[{'id':9,'campaign_id':1,'tier':1,'amount':25,'currency':'CNY','status':'claimable','version':1}],'leaderboard':[]}
hub={'any_enabled':True,'pending_actions':2,'growth':{'balance':42.3,'total_recharged':120,'first_recharge_eligible':False,'balance_low_warning':False,'recharge_multiplier':1,'payment_enabled':True,'vip':{'tier':1,'label':'V1','perks':['priority_support'],'next_tier':2,'next_label':'V2','amount_to_next':80}},'campaigns':[],'quests':quests,'arena':{'enabled':True,'rank':3,'token_sum':12000,'tokens_to_prev_rank':1000},'blindbox':blind,'checkin':{'enabled':True,'checked_in_today':False,'reward_amount':1.5,'streak_count':6},'quiz':{'enabled':True,'questions':[],'already_submitted':False,'reward_per_correct':.1},'team':{'enabled':False},'image_studio':{'enabled':False}}
def data_for(path,query):
 if path.endswith('/settings/public'):return settings
 if path.endswith('/auth/me'):return user
 if path.endswith('/play/hub'):return hub
 if path.endswith('/play/quests/today'):return quests
 if path.endswith('/play/arena/overview'):
  period={'id':1,'name':'Fixture daily board' if query.get('period',['daily'])[0]=='daily' else 'Fixture monthly board','status':'active','start_at':'2026-10-08T00:00:00Z','end_at':'2026-10-09T00:00:00Z'}
  return {'enabled':True,'period':period,'current':{**hub['arena'],'period':period,'estimated_reward':.2},'rows':[{'rank':i,'display_name':f'Fixture {i}','token_sum':25000-i*1000,'is_mine':i==3} for i in range(1,5)],'reward_tiers':[{'rank_max':1,'amount':.5},{'rank_max':3,'amount':.2}],'history':[]}
 if path.endswith('/play/blindbox/status'):return blind
 if path.endswith('/play/blindbox/pool'):return {'enabled':True}
 if path.endswith('/play/blindbox/recent'):return []
 if path.endswith('/user/aff/campaigns'):return [campaign]
 if path.endswith('/user/aff'):return {'user_id':50,'aff_code':'FIXTURE','aff_count':5,'aff_quota':0,'aff_frozen_quota':25,'aff_history_quota':25,'effective_rebate_rate_percent':20,'invitees':[]}
 if path.endswith('/play/teams/me'):return {'enabled':False}
 if path.endswith('/play/campaigns/active'):return []
 if 'announcements' in path:return {'items':[],'total':0}
 if 'subscriptions' in path:return []
 return {}
class Handler(SimpleHTTPRequestHandler):
 def __init__(self,*args,**kw):super().__init__(*args,directory=a.dist,**kw)
 def do_GET(self):
  if not Path(self.translate_path(self.path)).is_file():self.path='/index.html'
  super().do_GET()
 def log_message(self,*args):pass
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start();origin=f'http://127.0.0.1:{server.server_port}'
report=[]
cases=[(width,lang,theme,'default',False,1) for width,lang,theme in [(360,'zh','dark'),(768,'en','light'),(1280,'zh','light'),(1920,'en','dark')]]
cases += [(640,'en','light','zoom',False,2)]
cases += [(360,'zh','light',state,False,1) for state in ['loading','error','empty']]
with sync_playwright() as pw:
 browser=pw.chromium.launch(executable_path='/usr/bin/chromium',headless=True,args=['--no-sandbox','--disable-dev-shm-usage','--disable-background-networking'])
 for route in ['play','arena','blindbox','affiliate']:
  for width,lang,theme,state,guest,scale in cases:
   context=browser.new_context(viewport={'width':width,'height':900},device_scale_factor=scale,locale=lang,reduced_motion='reduce')
   context.add_init_script('window.__APP_CONFIG__='+json.dumps({**settings, 'affiliate_enabled':False} if state=='empty' and route=='play' else settings)+';localStorage.setItem("auth_token","fixture-only");localStorage.setItem("auth_user",JSON.stringify('+json.dumps(user)+'));localStorage.setItem("theme",'+json.dumps(theme)+');')
   errors=[];requests=[];held=[]
   targets=['/play/hub','/play/arena/overview','/play/blindbox/status','/user/aff/campaigns']
   def intercept(r):
    u=urlparse(r.request.url)
    if u.netloc != urlparse(origin).netloc:return r.abort()
    if '/api/' in u.path:
     requests.append({'path':u.path,'method':r.request.method})
     target=any(u.path.endswith(x) for x in targets)
     if target and state=='loading':held.append(r);return
     if target and state=='error':return r.fulfill(status=503,content_type='application/json',body=json.dumps({'code':503,'message':'Fixture unavailable'}))
     data=data_for(u.path,parse_qs(u.query))
     if target and state=='empty':
      if route=='play':data={**hub,'any_enabled':False,'campaigns':[],'quests':None,'arena':None,'blindbox':None,'checkin':None,'quiz':None,'team':None,'growth':{**hub['growth'],'vip':None}}
      elif route=='arena':data={**data,'rows':[],'reward_tiers':[],'current':{'enabled':True},'history':[]}
      elif route=='blindbox':data={**blind,'enabled':False}
      elif route=='affiliate':data=[]
     return r.fulfill(status=200,content_type='application/json',body=json.dumps({'code':0,'data':data}))
    r.continue_()
   context.route('**/*',intercept)
   page=context.new_page();page.on('pageerror',lambda e:errors.append(str(e)))
   page.goto(origin+'/'+route+'?lang='+lang,wait_until='domcontentloaded');page.wait_for_timeout(900)
   if state!='loading':page.wait_for_load_state('networkidle')
   name=f'updated-{route}-{width}-{lang}-{theme}-{state}.png'
   dimensions=page.evaluate('({width:innerWidth,scroll:document.documentElement.scrollWidth,height:innerHeight,theme:document.documentElement.className,language:document.documentElement.lang})')
   page.keyboard.press('Tab');focus=page.evaluate('({tag:document.activeElement.tagName,outline:getComputedStyle(document.activeElement).outlineStyle,name:document.activeElement.getAttribute("aria-label")||document.activeElement.textContent?.slice(0,80)})')
   page.screenshot(path=str(out/name),full_page=True)
   report.append({'route':route,'state':state,'viewport':str(width)+'x900','scale':scale,'lang':lang,'theme':theme,'errors':errors,'dimensions':dimensions,'focus':focus,'requests':requests,'screenshot':name})
   for pending in held:pending.abort()
   context.close()
 browser.close()
server.shutdown();Path(__file__).with_name('matrix-report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2));print(json.dumps({'captures':len(report),'page_errors':[r for r in report if r['errors']],'overflow':[r for r in report if r['dimensions']['scroll']>r['dimensions']['width']]},ensure_ascii=False,indent=2))
