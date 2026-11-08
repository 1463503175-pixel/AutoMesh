import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT=Path(__file__).resolve().parent

class CLIIntegrationTests(unittest.TestCase):
    def test_none_result_uses_workbench_local_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            fake=root/'ansys'/'workbench'
            fake.mkdir(parents=True)
            (root/'ansys'/'__init__.py').write_text('')
            (fake/'__init__.py').write_text('')
            (fake/'core.py').write_text('''
class FFF:
    Name='FFF'
    def GetContainer(self,ComponentName):
        if ComponentName=='Mesh':return object()
class FakeWB:
    server_version='2024 R2'
    def run_script_string(self,code):
        scope={'GetAllSystems':lambda:[FFF()]}
        exec(code,scope)
        return None

def connect_workbench(**kwargs):
    assert kwargs['host']=='127.0.0.1' and kwargs['port']==10605
    assert kwargs['security']=='insecure'
    return FakeWB()
''')
            env=os.environ.copy()
            env['PYTHONPATH']=str(root)
            run=subprocess.run([sys.executable,str(ROOT/'assets'/'wb_cli.py'),
                'list','--port','10605','--security','insecure'],env=env,
                stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
            self.assertEqual(run.returncode,0,run.stderr+' '+run.stdout)
            output=json.loads(run.stdout.strip().splitlines()[-1])
            self.assertEqual(output['status'],'ok')
            self.assertEqual(output['systems'][0]['name'],'FFF')
            self.assertTrue(output['systems'][0]['mesh'])

    def test_transport_auto_handles_unimplemented_incorrect_mode(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp); fake=root/'ansys'/'workbench'
            fake.mkdir(parents=True)
            (root/'ansys'/'__init__.py').write_text('')
            (fake/'__init__.py').write_text('')
            (fake/'core.py').write_text("""
class FFF:
    Name='FFF'
    DisplayText='Fluent'
    def GetContainer(self,ComponentName): return object() if ComponentName=='Mesh' else None
class WB:
    server_version='2024 R2'
    def run_script_string(self,code):
        scope={'GetAllSystems':lambda:[FFF()]}
        exec(code,scope)
        return scope.get('wb_script_result')
def connect_workbench(**kw):
    if kw['security']=='mtls': raise RuntimeError('StatusCode.UNIMPLEMENTED')
    if kw['security']!='insecure': raise RuntimeError('unsupported')
    return WB()
""")
            env=os.environ.copy();env['PYTHONPATH']=str(root)
            run=subprocess.run([sys.executable,str(ROOT/'assets'/'wb_cli.py'),
                'list','--port','49271','--security','auto'],env=env,
                stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
            self.assertEqual(run.returncode,0,run.stdout+run.stderr)
            result=json.loads(run.stdout.strip().splitlines()[-1])
            self.assertEqual(result['security'],'insecure')
            self.assertEqual(result['systems'][0]['name'],'FFF')

    def test_service_bare_probe_echo_does_not_block_readonly_list(self):
        """Reproduces the user's 2024 R2 + PyWorkbench 0.14 service reply."""
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            fake=root/'ansys'/'workbench'
            fake.mkdir(parents=True)
            (root/'ansys'/'__init__.py').write_text('')
            (fake/'__init__.py').write_text('')
            (fake/'core.py').write_text("""
class Fluid:
    Name='FFF'
    DisplayText='流体流动 (Fluent)'
    def GetContainer(self,ComponentName):
        return object() if ComponentName=='Mesh' else None
class WB:
    server_version='2024 R2'
    def run_script_string(self,code):
        if 'AUTOMESH_PROBE_OK' in code:
            # This was the user's failure: plain string is NOT JSON.
            return 'AUTOMESH_PROBE_OK'
        scope={'GetAllSystems':lambda:[Fluid()]}
        exec(code,scope)
        return scope.get('wb_script_result')
def connect_workbench(**kwargs):
    assert kwargs['port']==10605 and kwargs['security']=='insecure'
    return WB()
""")
            env=os.environ.copy()
            env['PYTHONPATH']=str(root)
            run=subprocess.run([sys.executable,str(ROOT/'assets'/'wb_cli.py'),
                'list','--port','10605','--security','insecure'],env=env,
                stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
            self.assertEqual(run.returncode,0,run.stderr+' '+run.stdout)
            self.assertEqual(json.loads(run.stdout.strip().splitlines()[-1])['systems'],
                [{'name':'FFF','label':'流体流动 (Fluent)','mesh':True}])

    def test_list_existing_system_fff_handles_missing_displaytext(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            fake=root/'ansys'/'workbench'
            fake.mkdir(parents=True)
            (root/'ansys'/'__init__.py').write_text('')
            (fake/'__init__.py').write_text('')
            (fake/'core.py').write_text('''\nclass FFF:\n    Name='FFF'\n    def GetContainer(self,ComponentName):\n        if ComponentName=='Mesh':return object()\nclass FakeWB:\n    server_version='2024 R2'\n    def run_script_string(self,code):\n        scope={'GetAllSystems':lambda:[FFF()]}\n        exec(code,scope)\n        return scope.get('wb_script_result')\ndef connect_workbench(**kwargs):\n    assert kwargs['host']=='127.0.0.1' and kwargs['port']==10605\n    assert kwargs['security']=='insecure'\n    return FakeWB()\n''')
            env=os.environ.copy()
            env['PYTHONPATH']=str(root)
            run=subprocess.run([sys.executable,str(ROOT/'assets'/'wb_cli.py'),
                'list','--port','10605','--security','insecure'],env=env,
                stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
            self.assertEqual(run.returncode,0,run.stderr+' '+run.stdout)
            parsed=json.loads(run.stdout.strip().splitlines()[-1])
            self.assertEqual(parsed['status'],'ok')
            self.assertEqual(parsed['systems'],[{'name':'FFF','label':'FFF','mesh':True}])
            self.assertEqual(parsed['version'],'2024 R2')

if __name__=='__main__':unittest.main()
