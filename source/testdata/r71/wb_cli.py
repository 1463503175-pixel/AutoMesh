# -*- coding: utf-8 -*-
"""Optional PyWorkbench: only connect to explicitly supplied existing server."""
import argparse, json, sys, traceback
from workbench_bridge import connect_existing, list_systems, dispatch_job
p=argparse.ArgumentParser()
p.add_argument('mode',choices=['list','dispatch'])
p.add_argument('--port',type=int,required=True)
p.add_argument('--security',default='insecure',choices=['mtls','wnua','insecure'])
p.add_argument('--target')
p.add_argument('--runner')
a=p.parse_args()
try:
    wb=connect_existing(a.port,a.security)
    if a.mode=='list':
        # Do not put an auxiliary JSON probe in front of this read-only query.
        # PyWorkbench 0.14 can return a decoded bare string, so the v0.9.3
        # probe rejected a healthy 2024 R2 connection before listing FFF.
        systems=list_systems(wb)
        try:version=str(wb.server_version)
        except Exception:version='unknown'
        print(json.dumps({'status':'ok','systems':systems,'version':version},ensure_ascii=True))
    else:
        if not a.target or not a.runner: raise ValueError('Missing target/runner')
        ret=dispatch_job(wb,a.target,a.runner,explicitly_open_editor=True)
        print(json.dumps({'status':'ok','result':str(ret)},ensure_ascii=True))
except Exception as exc:
    print(json.dumps({'status':'error','error':str(exc),'details':traceback.format_exc()},ensure_ascii=True))
    sys.exit(1)
