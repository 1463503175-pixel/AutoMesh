import ast, pathlib, unittest
from types import SimpleNamespace as NS
class OfficialMarkerIsolation(unittest.TestCase):
 def test_partial_native_creation_is_isolated_on_exception(self):
  children=[]
  selection=NS(Name='new_quality_group',ObjectId=10,SendToSolver=True,Location=NS(Ids=[1]))
  def create(*args): children.append(selection); raise RuntimeError('native partial failure')
  mesh=NS(Elements=10,GetVolumeMeshQualityCountFailed=lambda m:1,CreateMQVolumeElementsNamedSelection=create)
  context=dict(Model=NS(Mesh=mesh,NamedSelections=NS(Children=children)),MeshMetricType=NS(Skewness='skew'),utext=str)
  path=pathlib.Path(__file__).parent/'assets'/'meshing_worker.py'
  tree=ast.parse(path.read_text('utf8')); exec(compile(ast.Module(body=[n for n in tree.body if isinstance(n,ast.FunctionDef) and n.name=='mark_official_quality_elements'],type_ignores=[]),str(path),'exec'),context)
  with self.assertRaisesRegex(RuntimeError,'native partial failure'):context['mark_official_quality_elements']('failed')
  self.assertFalse(selection.SendToSolver)
if __name__=='__main__':unittest.main()
