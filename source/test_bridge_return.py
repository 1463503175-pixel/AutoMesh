import json
import pathlib
import tempfile
import unittest
from assets import workbench_bridge as bridge


class FakeSystem:
    def __init__(self, name, mesh=True, has_label=False):
        self.Name=name
        self._mesh=mesh
        if has_label:
            self.DisplayText='Fluid flow - '+name
    def GetContainer(self, ComponentName):
        assert ComponentName == 'Mesh'
        return object() if self._mesh else None


class ScriptWorkbench:
    def __init__(self, systems, no_result=False):
        self.systems=systems
        self.no_result=no_result
        self.commands=[]
    def run_script_string(self, code):
        self.commands.append(code)
        if self.no_result:
            return None
        scope={'GetAllSystems':lambda: self.systems}
        exec(code,scope)
        return scope.get('wb_script_result')


class BridgeReturnTests(unittest.TestCase):
    def test_missing_display_text_does_not_abort_entire_list(self):
        wb=ScriptWorkbench([FakeSystem('FFF',True,False),FakeSystem('FFF2',False,True)])
        self.assertEqual(bridge.read_only_probe(wb),'AUTOMESH_PROBE_OK')
        self.assertEqual(bridge.list_systems(wb),[
            {'name':'FFF','label':'FFF','mesh':True},
            {'name':'FFF2','label':'Fluid flow - FFF2','mesh':False}])
    def test_probe_accepts_raw_and_json_string(self):
        class BareReply:
            def run_script_string(self,code):
                return 'AUTOMESH_PROBE_OK'
        class EncodedReply:
            def run_script_string(self,code):
                return '"AUTOMESH_PROBE_OK"'
        self.assertEqual(bridge.read_only_probe(BareReply()), 'AUTOMESH_PROBE_OK')
        self.assertEqual(bridge.read_only_probe(EncodedReply()), 'AUTOMESH_PROBE_OK')

    def test_none_is_friendly_error_not_typeerror(self):
        wb=ScriptWorkbench([],no_result=True)
        with self.assertRaisesRegex(RuntimeError, 'did not write the local fallback'):
            bridge.list_systems(wb)
        self.assertEqual(bridge.read_only_probe(wb),'AUTOMESH_PROBE_NO_RETURN')
    def test_none_return_uses_local_file_without_relaunching(self):
        class DropReturnWB(ScriptWorkbench):
            def run_script_string(self, code):
                self.commands.append(code)
                scope={'GetAllSystems':lambda:self.systems}
                exec(code,scope)
                return None
        wb=DropReturnWB([FakeSystem('FFF',True)])
        self.assertEqual(bridge.list_systems(wb),[{'name':'FFF','label':'FFF','mesh':True}])
        self.assertEqual(len(wb.commands),2)
        self.assertNotIn('CreateSystem',str(wb.commands))

    def test_legacy_server_none_text_uses_fallback(self):
        class LiteralNoneWB(ScriptWorkbench):
            def run_script_string(self,code):
                scope={'GetAllSystems':lambda:self.systems}
                exec(code,scope)
                return 'None'
        wb=LiteralNoneWB([FakeSystem('FFF')])
        self.assertEqual(bridge.list_systems(wb)[0]['name'],'FFF')

    def test_return_supports_double_encoded_json(self):
        self.assertEqual(bridge._decode_script_result(json.dumps('[{"name":"FFF"}]'),'test'), [{'name':'FFF'}])
        self.assertEqual(bridge._decode_script_result([{'name':'FFF'}],'test'), [{'name':'FFF'}])
    def test_none_dispatch_does_not_retry(self):
        with tempfile.TemporaryDirectory() as d:
            path=pathlib.Path(d)/'Run_In_Existing_Meshing.py'
            path.write_text('pass',encoding='utf-8')
            (path.parent/'job.json').write_text(json.dumps({'job_id':'a'*32}),encoding='utf-8')
            class FakeWB:
                commands=[]
                def run_script_string(self,command):
                    self.commands.append(command)
                    return None if 'SendCommand' in command else '[]'
            wb=FakeWB()
            old=bridge.list_systems
            bridge.list_systems=lambda wb:[{'name':'FFF','mesh':True}]
            try:
                outcome=bridge.dispatch_job(wb,'FFF',path,explicitly_open_editor=True)
                self.assertEqual(outcome['status'],'dispatched_return_unconfirmed')
                self.assertEqual(len(wb.commands),1)
                self.assertIn('wb_script_result=json.dumps',wb.commands[0])
            finally:
                bridge.list_systems=old

if __name__=='__main__':unittest.main()
