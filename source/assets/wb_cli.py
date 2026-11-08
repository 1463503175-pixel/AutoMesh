# -*- coding: utf-8 -*-
"""Optional PyWorkbench: only connect to explicitly supplied existing server."""
import argparse, json, sys, traceback
from workbench_bridge import connect_existing, list_systems, dispatch_job
p=argparse.ArgumentParser()
p.add_argument('mode',choices=['list','dispatch'])
p.add_argument('--port',type=int,required=True)
p.add_argument('--security',default='insecure',choices=['auto','mtls','wnua','insecure'])
p.add_argument('--target')
p.add_argument('--runner')
a=p.parse_args()
try:
    # StartServer() may use mTLS (official default) or an explicitly selected
    # transport. Never assume 'insecure' for a server we did not launch.
    if a.mode == 'list':
        modes = ('mtls', 'insecure', 'wnua') if a.security == 'auto' else (a.security,)
        attempts = []
        for selected in modes:
            try:
                wb = connect_existing(a.port, selected)
                systems = list_systems(wb)
                try: version = str(wb.server_version)
                except Exception: version = 'unknown'
                print(json.dumps({'status':'ok', 'systems':systems,
                                  'version':version, 'security':selected},
                                 ensure_ascii=True))
                break
            except Exception as exc:
                attempts.append('%s: %s' % (selected, str(exc)[:240]))
        else:
            raise RuntimeError('Port %d did not pass Workbench script verification; '
                               'tried transports: %s' % (a.port, '; '.join(attempts)))
    else:
        if a.security == 'auto':
            raise ValueError('Dispatch requires the verified transport; refusing to guess')
        wb=connect_existing(a.port,a.security)
        if not a.target or not a.runner: raise ValueError('Missing target/runner')
        ret=dispatch_job(wb,a.target,a.runner,explicitly_open_editor=True)
        print(json.dumps({'status':'ok','result':str(ret)},ensure_ascii=True))
except Exception as exc:
    print(json.dumps({'status':'error','error':str(exc),'details':traceback.format_exc()},ensure_ascii=True))
    sys.exit(1)
