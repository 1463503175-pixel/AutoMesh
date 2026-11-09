"""Python-only simulation of Workbench routing. Does not start or edit ANSYS."""
import json
import tempfile
from pathlib import Path
import unittest
from assets import workbench_bridge as bridge

class Model:
    Name = 'FFF'
    def __init__(self,mesh):self.mesh=mesh
    def GetContainer(self,ComponentName):
        assert ComponentName=='Mesh'
        return self.mesh

class Mesh:
    def __init__(self,path):self.path=path;self.calls=0
    def SendCommand(self,Language,Command):
        assert Language=='Python' and 'Run_In_Existing_Meshing.py' in Command
        self.calls+=1
        receipts=list(self.path.glob('wb_dispatch_*.json'))
        assert len(receipts)==1
        receipt=json.loads(receipts[0].read_text())
        assert receipt['status']=='wb_target_verified'

class FakeWB:
    def __init__(self,model):self.model=model;self.count=0
    def run_script_string(self,cmd):
        self.count+=1
        scope={'GetAllSystems':lambda:[self.model]}
        exec(cmd,scope)
        return scope['wb_script_result']

class ReceiptTests(unittest.TestCase):
    def test_workbench_receipt_before_send_and_return_after(self):
        with tempfile.TemporaryDirectory() as tmp:
            path=Path(tmp)/'Run_In_Existing_Meshing.py'
            path.write_text('pass')
            task = {'job_id':'a'*32, 'logical_job_id':'b'*32, 'attempt_id':'c'*32,
                    'attempt_number':2, 'retry_of_job_id':'d'*32}
            (path.parent/'job.json').write_text(json.dumps(task))
            mesh=Mesh(path.parent)
            wb=FakeWB(Model(mesh))
            orig=bridge.list_systems
            bridge.list_systems=lambda _:[{'name':'FFF','mesh':True}]
            try:result=bridge.dispatch_job(wb,'FFF',path,explicitly_open_editor=True)
            finally:bridge.list_systems=orig
            self.assertEqual(result['status'],'dispatched')
            self.assertEqual(mesh.calls,1)
            receipt=json.loads((path.parent/('wb_dispatch_'+'a'*32+'.json')).read_text())
            self.assertEqual(receipt['status'],'wb_sendcommand_returned')
            self.assertEqual(receipt['logical_job_id'], task['logical_job_id'])
            self.assertEqual(receipt['attempt_id'], task['attempt_id'])
            self.assertEqual(receipt['attempt_number'], 2)
            self.assertEqual(receipt['retry_of_job_id'], task['retry_of_job_id'])

    def test_must_have_valid_job_identity_before_send(self):
        with tempfile.TemporaryDirectory() as tmp:
            path=Path(tmp)/'Run_In_Existing_Meshing.py'
            path.write_text('pass')
            (path.parent/'job.json').write_text(json.dumps({'job_id':'invalid'}))
            mesh=Mesh(path.parent);wb=FakeWB(Model(mesh))
            orig=bridge.list_systems
            bridge.list_systems=lambda _:[{'name':'FFF','mesh':True}]
            try:
                with self.assertRaises(ValueError):
                    bridge.dispatch_job(wb,'FFF',path,explicitly_open_editor=True)
            finally:bridge.list_systems=orig
            self.assertEqual(mesh.calls,0)

if __name__=='__main__':unittest.main()
