import http.server, http.client, os
os.chdir('/workspace/account-edit-fix/backend/internal/web/dist')
class Handler(http.server.SimpleHTTPRequestHandler):
 def log_message(self,*args): pass
 def do_GET(self):
  if self.path.startswith('/api/') or self.path.startswith('/health'): return self.proxy()
  if not os.path.exists(self.path.lstrip('/').split('?')[0]): self.path='/index.html'
  super().do_GET()
 def do_POST(self): self.proxy()
 def do_PUT(self): self.proxy()
 def do_DELETE(self): self.proxy()
 def do_PATCH(self): self.proxy()
 def proxy(self):
  conn=http.client.HTTPConnection('127.0.0.1',8082,timeout=40)
  body=self.rfile.read(int(self.headers.get('content-length',0)))
  conn.request(self.command,self.path,body,dict(self.headers))
  res=conn.getresponse(); data=res.read()
  self.send_response(res.status)
  for k,v in res.getheaders():
   if k.lower() not in ['content-length','transfer-encoding','connection','content-encoding']: self.send_header(k,v)
  self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data);conn.close()
http.server.ThreadingHTTPServer(('127.0.0.1',4176),Handler).serve_forever()
