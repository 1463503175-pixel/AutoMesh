# -*- coding: utf-8 -*-
"""PyWorkbench integration. Never launch, exit, open projects, or create systems.
Connection and list_systems are read-only. dispatch_job is explicitly opt-in.
"""
import json
import re
import os
import uuid
from pathlib import Path

# Run inside Workbench IronPython, NOT in desktop CPython. Prefer a single
# machine-readable response even if an optional system property is missing.
# The prior version read DisplayText unguarded, so one unsupported system could
# abort the whole script and PyWorkbench would return None.
LIST_SCRIPT = r'''import json
items=[]
for s in GetAllSystems():
    name=''
    try:
        name=str(s.Name)
    except Exception:
        continue
    try:
        label=str(s.DisplayText)
    except Exception:
        label=name
    if not label:
        label=name
    has_mesh=False
    try:
        has_mesh=(s.GetContainer(ComponentName='Mesh') is not None)
    except Exception:
        try:
            has_mesh=(s.GetComponent(Name='Mesh') is not None)
        except Exception:
            pass
    items.append({'name':name,'label':label,'mesh':has_mesh})
wb_script_result=json.dumps(items)
'''

# Test whether the server actually returns values before diagnosing model APIs.
PROBE_SCRIPT = "import json\nwb_script_result=json.dumps('AUTOMESH_PROBE_OK')"


def _missing_return_value(raw):
    return raw is None or (isinstance(raw,str) and raw.strip().lower() in ('','none','null'))


def _decode_script_result(raw, purpose):
    """WorkbenchClient may return JSON text or an already-decoded object.

    None must never reach json.loads. It indicates a script result transfer
    problem or an exception on the Workbench side, not a user security error.
    """
    if raw is None:
        raise RuntimeError('%s did not return wb_script_result (received None); check the Workbench command log / unsupported script properties' % purpose)
    if isinstance(raw, (list, dict)):
        return raw
    if isinstance(raw, bytes):
        raw=raw.decode('utf-8-sig','replace')
    if not isinstance(raw, str):
        raise RuntimeError('%s returned unexpected response type %s' % (purpose,type(raw).__name__))
    if not raw.strip():
        raise RuntimeError('%s returned empty text instead of a JSON value' % purpose)
    try:
        obj=json.loads(raw)
        # Some external transports double-encode JSON; handle once only.
        if isinstance(obj,str) and obj[:1] in ('[','{'):
            obj=json.loads(obj)
        return obj
    except (TypeError,ValueError) as exc:
        raise RuntimeError('%s returned non-JSON output (first 100 chars: %r): %s' % (purpose,raw[:100],exc))



def valid_port(port):
    p = int(port)
    if not 1 <= p <= 65535:
        raise ValueError('Workbench server port must be 1..65535')
    return p


def connect_existing(port, security='insecure'):
    if security not in ('mtls', 'wnua', 'insecure'):
        raise ValueError('Unsupported security')
    from ansys.workbench.core import connect_workbench  # lazy: optional dependency
    # Do not use launch_workbench(): user requires the exact existing session.
    return connect_workbench(port=valid_port(port), host='127.0.0.1', security=security,
                             client_workdir=str(Path(__file__).resolve().parent))


def _fallback_systems_from_local_file(wb):
    """Read-only local fallback if a Workbench 2024 return channel yields None.

    Both processes are on localhost. We do NOT create a new WB or new model;
    the only side effect is a tiny JSON file in the dedicated app data folder.
    The path is unpredictable and cleaned after each attempt to reject stale data.
    """
    target=Path(__file__).resolve().parent/('wb_systems_'+uuid.uuid4().hex+'.json')
    # The Workbench script runs under IronPython; use a Unicode absolute path.
    # Make sure errors don't silently lead to stale reports.
    raw_path=json.dumps(str(target).replace('\\','/'),ensure_ascii=True)
    script=LIST_SCRIPT+"\nwith open(u%s,'w') as _f:\n    _f.write(wb_script_result)\n" % raw_path
    try:
        wb.run_script_string(script)   # return may still be None
        if not target.is_file():
            raise RuntimeError('Workbench did not write the local fallback list file; inspect Workbench command-window errors')
        return _decode_script_result(target.read_text(encoding='utf-8-sig'),'Workbench local fallback file')
    finally:
        try:target.unlink()
        except OSError:pass


def list_systems(wb):
    raw=wb.run_script_string(LIST_SCRIPT)
    if _missing_return_value(raw):
        data=_fallback_systems_from_local_file(wb)
    else:
        data=_decode_script_result(raw,'List systems')
    if not isinstance(data, list):
        raise RuntimeError('Workbench returned %s, expected system list' % type(data).__name__)
    result=[]
    for n,item in enumerate(data):
        if not isinstance(item,dict):
            raise RuntimeError('System row %d is not a JSON object' % n)
        name=str(item.get('name') or '')
        if not name:
            continue
        result.append({'name':name,'label':str(item.get('label') or name),
                       'mesh':bool(item.get('mesh',False))})
    return result


def read_only_probe(wb):
    """Optional diagnostics only, never a mandatory connection gate.

    PyWorkbench 0.14 can return either an already-decoded string or the JSON
    serialization itself. A healthy session returning the bare sentinel must
    not be mistaken for a malformed JSON result.
    """
    raw=wb.run_script_string(PROBE_SCRIPT)
    if _missing_return_value(raw):
        return 'AUTOMESH_PROBE_NO_RETURN'
    if isinstance(raw, bytes):
        raw=raw.decode('utf-8-sig','replace')
    if raw == 'AUTOMESH_PROBE_OK':
        return raw
    result=_decode_script_result(raw,'Workbench return-value probe')
    if result != 'AUTOMESH_PROBE_OK':
        raise RuntimeError('Workbench returned an unexpected probe value: %r' % result)
    return result


def dispatch_job(wb, system_name, worker_runner, *, explicitly_open_editor=False):
    """Sends a command to a selected WB system; only with explicit GUI-open affirmation.

    Important: Workbench may launch a *hidden* Meshing instance if editor isn't open.
    This cannot be excluded via documented API. Never silently dispatch.
    """
    if not explicitly_open_editor:
        raise PermissionError('Refusing SendCommand: editor-open approval required')
    if not system_name or len(system_name)>128:
        raise ValueError('Invalid system name')
    path = Path(worker_runner).resolve()
    if not path.is_file() or path.name != 'Run_In_Existing_Meshing.py':
        raise ValueError('Runner file missing or unexpected')
    if system_name not in [s['name'] for s in list_systems(wb) if s['mesh']]:
        raise RuntimeError('System no longer exists or does not have a Mesh cell')
    # JScript/WB IronPython runner -> ACT IronPython; no subprocess/GUI clicks.
    # Enumerate to get the exact selected container, never create a new system.
    runner = "execfile(u%s)" % json.dumps(str(path).replace('\\', '/'), ensure_ascii=True)
    # A durable receipt written BEFORE SendCommand separates WB routing from
    # actual ACT execution. Never infer Meshing started from this receipt.
    job_data = json.loads((path.parent/'job.json').read_text(encoding='utf-8'))
    job_id = str(job_data.get('job_id',''))
    if not re.fullmatch(r'[0-9a-f]{32}',job_id):
        raise ValueError('Invalid job ID in submission')
    receipt_path = str(path.parent/('wb_dispatch_'+job_id+'.json')).replace('\\','/')
    receipt_py = json.dumps(receipt_path, ensure_ascii=True)
    job_py = json.dumps(job_id, ensure_ascii=True)
    command = ("target=None\n"
               "target_count=0\n"
               "for sys in GetAllSystems():\n"
               "    if str(sys.Name)==%s:\n" % json.dumps(system_name, ensure_ascii=True) +
               "        target=sys\n"
               "        target_count+=1\n"
               "if target_count != 1: raise Exception('Target system missing or duplicate')\n"
               "mesh=target.GetContainer(ComponentName='Mesh')\n"
               "if mesh is None: raise Exception('Mesh container missing')\n"
               "import os, time, json\n"
               "_receipt={'job_id':%s,'status':'wb_target_verified','stage':'Workbench target Mesh container verified','timestamp':time.time()}\n" % job_py +
               "_receipt_path=u%s\n" % receipt_py +
               "_receipt_tmp=_receipt_path+'.tmp'\n"
               "with open(_receipt_tmp,'w') as _r: _r.write(json.dumps(_receipt))\n"
               "os.rename(_receipt_tmp,_receipt_path)\n"
               "mesh.SendCommand(Language='Python',Command=%s)\n" % repr(runner) +
               "_receipt['status']='wb_sendcommand_returned'\n"
               "_receipt['timestamp']=time.time()\n"
               "with open(_receipt_tmp,'w') as _r: _r.write(json.dumps(_receipt))\n"
               "try: os.remove(_receipt_path)\n"
               "except OSError: pass\n"
               "os.rename(_receipt_tmp,_receipt_path)\n"
               "wb_script_result=json.dumps({'status':'dispatched'})\n")
    # Command may have reached Meshing even if the Workbench return channel is
    # unavailable. Never automatically retry a dispatched job because doing so
    # could run the same geometry mutation twice. Go side waits by job UUID.
    raw=wb.run_script_string(command)
    if raw is None:
        return {'status':'dispatched_return_unconfirmed'}
    return _decode_script_result(raw,'Dispatch acknowledgment')
