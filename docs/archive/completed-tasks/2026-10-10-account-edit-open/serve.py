import os,secrets,subprocess,signal
from pathlib import Path
root=Path('/workspace/edit-fix-evidence')
env=dict(os.environ,AUTO_SETUP='true',DATA_DIR=str(root/'app-data'),DATABASE_HOST='127.0.0.1',DATABASE_PORT='55434',DATABASE_USER='postgres',DATABASE_DBNAME='sub2api_edit_test',REDIS_HOST='127.0.0.1',REDIS_PORT='56381',SERVER_HOST='127.0.0.1',SERVER_PORT='8082',JWT_SECRET=secrets.token_hex(32),ADMIN_EMAIL='fixture-admin@example.invalid',ADMIN_PASSWORD=secrets.token_urlsafe(40),GOMAXPROCS='2',GOMEMLIMIT='3GiB',LOG_LEVEL='warn',TZ='UTC')
with (root/'server.log').open('w') as log:
 server=subprocess.Popen([str(root/'server-latest')],env=env,stdout=log,stderr=subprocess.STDOUT)
 (root/'server.pid').write_text(str(server.pid))
 try: server.wait()
 finally: server.terminate()
