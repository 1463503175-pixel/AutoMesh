# -*- coding: utf-8 -*-
"""Start only the scripting server of an already-open Workbench GUI.

Never call launch_workbench(), start a Workbench process, or use visual
coordinates/screenshot matching. PowerShell uses Windows UI Automation.
Port candidates are ONLY hints: caller must verify with PyWorkbench and
list_systems() before reporting connected. Failed/unknown commands are never
retried automatically.
"""
import argparse
import json
import os
import pathlib
import re
import subprocess
import sys
import time


def parse_listening_ports(output):
    """Parse netstat -ano -p tcp output across English/Chinese Windows."""
    found = set()
    for line in output.splitlines():
        columns = line.strip().split()
        if len(columns) < 5 or columns[0].upper() != 'TCP':
            continue
        if columns[3].upper() not in ('LISTENING', '侦听'):
            continue
        address = columns[1]
        if address.startswith('['):
            match = re.search(r'\]:(\d+)$', address)
            port = match.group(1) if match else ''
        else:
            port = address.rsplit(':', 1)[-1]
        try:
            value = int(port)
            pid = int(columns[4])
            if 1024 <= value <= 65535:
                found.add((value, pid))
        except ValueError:
            continue
    return found


def listeners():
    cp = subprocess.run(['netstat', '-ano', '-p', 'tcp'],
                        stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                        timeout=9, check=True)
    # Windows netstat uses OEM/console encoding; ASCII protocol words survive.
    return (parse_listening_ports(cp.stdout.decode('utf-8', errors='replace')) |
            parse_listening_ports(cp.stdout.decode('gbk', errors='replace')))


def invoke_uia(helper, mode='start', timeout=18):
    exe = os.path.join(os.environ.get('SystemRoot', r'C:\Windows'),
                       'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe')
    if not os.path.isfile(exe):
        exe = 'powershell.exe'
    out = subprocess.run([exe, '-NoLogo', '-NoProfile', '-NonInteractive', '-STA',
                          '-ExecutionPolicy', 'Bypass', '-File', str(helper),
                          '-Mode', mode], stdout=subprocess.PIPE,
                         stderr=subprocess.PIPE, timeout=timeout)
    for line in reversed(out.stdout.decode('utf-8-sig', errors='replace').splitlines()):
        if line.strip().startswith('{'):
            obj = json.loads(line.strip())
            if isinstance(obj, dict) and 'status' in obj:
                return obj
    raise RuntimeError('UIAutomation helper produced no result: ' +
                       out.stderr.decode('utf-8', errors='replace')[-300:])


def bootstrap(helper, invoke=invoke_uia, get_listeners=listeners, sleep=time.sleep,
              deadline=8):
    before = get_listeners()
    outcome = invoke(helper, mode='start')
    if outcome.get('status') != 'submitted':
        return {'status': 'unavailable', 'detail': outcome.get('detail', '启动失败'),
                'candidate_ports': []}
    target = int(outcome.get('process_id') or 0)
    candidates = set()
    # Some versions launch the server on another PID. The new listener diff is
    # only a hint; wb_cli list_systems must validate the actual Workbench.
    for _ in range(max(1, int(deadline * 2))):
        current = get_listeners()
        new = current - before
        candidates = {port for port, pid in new if pid == target} or {p for p, _ in new}
        if candidates:
            break
        sleep(.5)
    return {'status': 'submitted', 'detail': outcome.get('detail', ''),
            'candidate_ports': sorted(candidates), 'process_id': target}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--mode', choices=('start','probe'), default='start')
    opt = parser.parse_args()
    if os.name != 'nt':
        print(json.dumps({'status': 'unavailable', 'detail': 'Only Windows is supported', 'candidate_ports': []}))
        return
    helper = pathlib.Path(__file__).with_name('start_wb_server_uia.ps1')
    try:
        if opt.mode == 'probe':
            result = invoke_uia(helper, mode='probe')
            if result.get('status') == 'identified':
                process_id = int(result.get('process_id') or 0)
                result['candidate_ports'] = sorted(
                    {port for port, pid in listeners() if pid == process_id})
        else:
            result = bootstrap(helper)
        print(json.dumps(result, ensure_ascii=True))
    except Exception as exc:
        print(json.dumps({'status': 'unavailable', 'detail': str(exc),
                          'candidate_ports': []}, ensure_ascii=True))


if __name__ == '__main__':
    main()
