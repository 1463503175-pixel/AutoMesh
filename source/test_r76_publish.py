# coding: utf-8
"""Safety regression for multi-batch preview and ACT publishing."""
import pathlib
import tempfile
import unittest
import runpy
import json
from test_warning_refinement_v52 import Fixture, MockNode, MockElement

BASE=pathlib.Path(__file__).parent

class BatchDiagnosticsTests(unittest.TestCase):
    def test_24_candidates_in_one_review_with_six_per_batch(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture(); snap=f.marked(d)
            nodes={}; elements={}; nid=1
            # 297 separated elements, cheap 1.5-mm spheres. This exercises
            # batch count, not a claim about the user's real complex geometry.
            for i in range(297):
                x=i*30.0
                ids=[]
                for dx,dy,dz in ((0,0,0),(.3,0,0),(0,.3,0),(0,0,.3)):
                    nodes[nid]=MockNode(nid,x+dx,dy,dz)
                    ids.append(nid);nid+=1
                elements[10000+i]=MockElement(ids)
            f.meshdata.n=nodes;f.meshdata.e=elements
            g=f.model.NamedSelections.Children[-1]
            g.Location.Ids=list(elements)
            marker=pathlib.Path(d,'warning_refinement_marker.json')
            data=json.loads(marker.read_text())
            data['groups'][0]['ids']=sorted(g.Location.Ids)
            data['worksheet_count']=297
            marker.write_text(json.dumps(data))
            r=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(r['status'],'ok',r.get('error'))
            plan=r['summary']
            self.assertEqual(plan['group_count'],297)
            self.assertEqual(plan['batch_count'],50)
            self.assertEqual(len(plan['clusters']),297)
            self.assertEqual(plan['clusters'][0]['batch_index'],1)
            self.assertEqual(plan['clusters'][23]['batch_index'],4)
            self.assertEqual(plan['clusters'][-1]['batch_index'],50)
            self.assertEqual(plan['covered_count']+plan['uncovered_count'],297)
            self.assertEqual(plan['uncovered_count'],0)
            self.assertTrue(plan['total_cell_proxy']<=120000)
            pub=f.run(d,'warning_refine_publish',4,snap)
            self.assertEqual(pub['status'],'ok',pub.get('error'))
            self.assertEqual(pub['summary']['sizing_count'],24)
            self.assertEqual(pub['summary']['batch_count'],4)
            self.assertEqual(len(f.model.Mesh.Children),24)
            self.assertEqual(f.model.Mesh.count,0)

    def test_partial_act_error_rolls_back_new_objects(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            f.run(d,'warning_refine_preview',3,snap)
            original=f.model.Mesh.AddSizing
            calls=[0]
            def broken():
                calls[0]+=1
                if calls[0]==2:raise RuntimeError('simulated ACT add sizing failed')
                return original()
            f.model.Mesh.AddSizing=broken
            r=f.run(d,'warning_refine_publish',4,snap)
            self.assertEqual(r['status'],'error')
            self.assertIn('simulated ACT',r['error'])
            self.assertEqual(f.model.Mesh.Children,[])
            self.assertEqual(f.model.CoordinateSystems.Children,[])
            self.assertEqual(f.model.Mesh.count,0)

    def test_wrong_origin_enum_never_assigned(self):
        ns=runpy.run_path(str(BASE/'assets'/'warning_refinement.py'),
            init_globals={'Quantity':lambda text:text,'utext':str})
        class MockCS:
            OriginDefineBy='Geometry Selection'
            def __init__(self): self.set_origin=False
            def SetOriginLocation(self,*values): self.set_origin=values
        c=MockCS()
        ns['_set_origin_mm'](c,[1,2,3])
        self.assertEqual(c.OriginDefineBy,'Geometry Selection')
        self.assertEqual(c.set_origin,('1 [mm]','2 [mm]','3 [mm]'))

    def test_json_atomic_replaces_existing_receipt(self):
        # Extract and test the real write function without launching ACT.
        import ast
        src=(BASE/'assets'/'meshing_worker.py').read_text()
        fn=next(n for n in ast.parse(src).body if isinstance(n,ast.FunctionDef) and n.name=='write')
        code=compile(ast.Module(body=[fn],type_ignores=[]),'write_only','exec')
        import json, os, time, tempfile
        ctx={'json':json,'os':os,'time':time,'tempfile':tempfile}
        exec(code,ctx)
        with tempfile.TemporaryDirectory() as d:
            f=str(pathlib.Path(d,'job_a.json'))
            ctx['write'](f,{'status':'started'})
            ctx['write'](f,{'status':'ok','n':297})
            self.assertEqual(json.loads(pathlib.Path(f).read_text())['n'],297)
            self.assertEqual(list(pathlib.Path(d).glob('*.tmp')),[])

if __name__=='__main__':unittest.main()
