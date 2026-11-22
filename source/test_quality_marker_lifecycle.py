import ast, pathlib, tempfile, unittest, json, os
from types import SimpleNamespace as NS
W=pathlib.Path(__file__).parent/'assets'/'warning_refinement.py'
class MarkerLifecycleTests(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
  self.deleted=[]; self.refs=[]
  self.obj=NS(Name='AutoMesh_EQ_000_500_12345678',ObjectId=8,SendToSolver=False,Location=NS(Ids=[1,2]),GetDependentObjects=lambda:self.refs,Delete=lambda:(self.deleted.append(8),self.ctx["Model"].NamedSelections.Children.remove(self.obj)))
  self.state={'project':'test-project','fingerprint':'geometry'}
  self.ctx=dict(Model=NS(NamedSelections=NS(Children=[self.obj]),Mesh=NS(Elements=2)),os=os,json=json,time=__import__('time'),utext=str,JOB_FILE=self.tmp.name+'/job.json',job={'job_id':'a'*32},meshing_process_identity=lambda:{'process_pid':1,'process_started_filetime':'now'},read=lambda p:json.loads(pathlib.Path(p).read_text('utf8')),write=lambda p,d:pathlib.Path(p).write_text(json.dumps(d),'utf8'))
  names=('_warning_path','quality_marker_register','quality_marker_prepare')
  tree=ast.parse(W.read_text('utf8')); exec(compile(ast.Module(body=[n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name in names],type_ignores=[]),str(W),'exec'),self.ctx)
 def register(self): return self.ctx['quality_marker_register']({'status':'created','created_groups':[self.obj.Name]},self.state)
 def test_owned_unreferenced_marker_can_be_retired(self):
  self.register(); self.ctx['quality_marker_prepare'](self.state); self.assertEqual(self.deleted,[8])
 def test_reference_blocks_cleanup_and_new_marker(self):
  self.register(); self.refs.append(NS(Name='report'))
  with self.assertRaises(RuntimeError): self.ctx['quality_marker_prepare'](self.state)
  self.assertEqual(self.deleted,[])
 def test_changed_session_and_scope_never_deleted(self):
  self.register(); self.obj.Location.Ids=[3]
  with self.assertRaises(RuntimeError): self.ctx['quality_marker_prepare'](self.state)
  self.assertEqual(self.deleted,[])
 def test_missing_dependency_api_fails_closed(self):
  self.register(); del self.obj.GetDependentObjects
  with self.assertRaises(RuntimeError): self.ctx['quality_marker_prepare'](self.state)
  self.assertEqual(self.deleted,[])
 def test_prefix_without_ledger_is_never_deleted(self):
  self.ctx['quality_marker_prepare'](self.state); self.assertEqual(self.deleted,[])
if __name__=='__main__': unittest.main()
