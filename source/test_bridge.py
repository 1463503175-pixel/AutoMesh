import pathlib,sys,types,unittest
from unittest.mock import patch
from assets.workbench_bridge import connect_existing, valid_port
class BridgeTests(unittest.TestCase):
    def test_existing_only_ascii_client_directory(self):
        calls=[]
        ansys=types.ModuleType('ansys');ansys.__path__=[]
        workbench=types.ModuleType('ansys.workbench');workbench.__path__=[]
        core=types.ModuleType('ansys.workbench.core')
        def connect_workbench(**kwargs):
            calls.append(kwargs);return 'success'
        core.connect_workbench=connect_workbench
        with patch.dict(sys.modules,{'ansys':ansys,'ansys.workbench':workbench,'ansys.workbench.core':core}):
            self.assertEqual(connect_existing(53200,'mtls'),'success')
        self.assertEqual(calls[0]['host'],'127.0.0.1')
        self.assertTrue(pathlib.Path(calls[0]['client_workdir']).is_dir())
        self.assertNotIn('launch_workbench',str(calls[0]))
    def test_rejects_bad_port_security(self):
        with self.assertRaises(ValueError): valid_port(70000)
        with self.assertRaises(ValueError): connect_existing(53200,'bogus')
if __name__=='__main__':unittest.main()
