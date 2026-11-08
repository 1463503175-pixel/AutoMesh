# coding: utf-8
"""R7.5 hotspot regression: 297 current mesh elements, partial safety, no mesh mutation."""
import tempfile, unittest, json, pathlib, runpy
from test_warning_refinement_v52 import Fixture, MockNode, MockElement
BASE=pathlib.Path(__file__).parent

class ClusteredWarningTests(unittest.TestCase):
    def test_297_marked_elements_do_not_trigger_64_global_rejection(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture(); snap=f.marked(d)
            nodes={}; elems={}; nid=1
            # Three geographically separated defects; no shared nodal IDs;
            # the spatial check should still group close element centroids.
            for k in range(297):
                x=(k//99)*75+(k%99)*0.09
                ids=[]
                for dx,dy,dz in ((0,0,0),(.9,0,0),(0,.9,0),(0,0,.9)):
                    nodes[nid]=MockNode(nid,x+dx,dy,dz)
                    ids.append(nid);nid+=1
                elems[1000+k]=MockElement(ids)
            f.meshdata.n=nodes; f.meshdata.e=elems
            f.model.Mesh.Elements=10055480
            grp=f.model.NamedSelections.Children[-1]
            grp.Location.Ids=list(range(1000,1297))
            # Real marker registration stores this same count and mesh identity.
            mark=pathlib.Path(d,'warning_refinement_marker.json')
            info=json.loads(mark.read_text())
            info['groups'][0]['ids']=grp.Location.Ids
            info['worksheet_count']=297
            info['mesh_count']=10055480
            mark.write_text(json.dumps(info))
            preview=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(preview['status'],'ok',preview.get('error'))
            p=preview['summary']
            self.assertEqual(p['warning_count'],297)
            self.assertGreater(p['covered_count'],0)
            self.assertLessEqual(p['group_count'],4096)
            self.assertEqual(p['covered_count']+p['uncovered_count'],297)
            self.assertNotIn('selection_exceeds_safe_hotspot_limit',str(p))
            self.assertTrue(all(c['body_id']==1 for c in p['clusters']))
            self.assertTrue(all(c['radius_mm']<=12 for c in p['clusters']))
            self.assertTrue(all(len(c['element_ids'])<=64 for c in p['clusters']))
            self.assertEqual(f.model.Mesh.count,0)
            self.assertEqual(f.model.Mesh.Children,[])
            pub=f.run(d,'warning_refine_publish',4,snap)
            self.assertEqual(pub['status'],'ok',pub.get('error'))
            self.assertEqual(len(f.model.Mesh.Children),min(p['group_count'],24))
            self.assertEqual(f.model.Mesh.count,0)

    def test_rejected_one_cluster_does_not_poison_others(self):
        ns=runpy.run_path(str(BASE/'assets'/'warning_refinement.py'),
                          init_globals={'cancellation_checkpoint':lambda:None})
        func=ns['_hotspot_candidates']
        members=[]
        for i in range(20):
            members.append({'id':i+1,'body':1,'xyz_mm':[i*.08,0,0],
                            'span_mm':1.4,'node_ids':[100+i*4,101+i*4,102+i*4,103+i*4]})
        members.append({'id':200,'body':2,'xyz_mm':[200,0,0],
                        'span_mm':20.0,'node_ids':[2000,2001,2002,2003]})
        denied=[]
        c,proxy=func(members,{},denied)
        self.assertGreater(len(c),0)
        self.assertTrue(all(x['body_id']==1 for x in c))
        self.assertTrue(any(x['element_ids']==[200] for x in denied))
        self.assertTrue(all(x['radius_mm']<=12.0 for x in c))

    def test_hard_memory_guard_is_separate_from_hotspot_size(self):
        ns=runpy.run_path(str(BASE/'assets'/'warning_refinement.py'),
                          init_globals={'cancellation_checkpoint':lambda:None})
        self.assertEqual(ns['MAX_WARNINGS'],64)
        self.assertGreater(ns['MAX_ANALYSIS_ELEMENTS'],297)
        self.assertEqual(ns['MAX_SPHERES'],4096)
        self.assertEqual(ns['MAX_BATCH_SPHERES'],6)
        self.assertEqual(ns['MAX_RADIUS_MM'],12.)

if __name__=='__main__':unittest.main()

class SafeReuseTests(unittest.TestCase):
    def test_existing_owned_sizing_is_reused_without_duplicate_objects(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            p=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(p['status'],'ok',p.get('error'))
            a=f.run(d,'warning_refine_publish',4,snap)
            self.assertEqual(a['status'],'ok',a.get('error'))
            old_ids=[x.ObjectId for x in f.model.Mesh.Children]
            old_cs=[x.ObjectId for x in f.model.CoordinateSystems.Children]
            # A new *read-only* preview can have updated coordinates without
            # changing the original geometry or stale element NS membership.
            f.meshdata.n[1].X+=0.3
            p2=f.run(d,'warning_refine_preview',5,snap)
            self.assertEqual(p2['status'],'ok',p2.get('error'))
            self.assertNotEqual(p['summary']['plan_id'],p2['summary']['plan_id'])
            self.assertEqual(p['summary']['group_count'],p2['summary']['group_count'])
            b=f.run(d,'warning_refine_publish',6,snap)
            self.assertEqual(b['status'],'error')
            self.assertTrue('已发布' in b.get('error','') or '旧版' in b.get('error','') or '叠加' in b.get('error',''), b.get('error',''))
            self.assertEqual([x.ObjectId for x in f.model.Mesh.Children],old_ids)
            self.assertEqual([x.ObjectId for x in f.model.CoordinateSystems.Children],old_cs)
            self.assertEqual(f.model.Mesh.count,0)

    def test_manual_change_to_owned_sizing_is_not_overwritten(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            self.assertEqual(f.run(d,'warning_refine_preview',3,snap)['status'],'ok')
            self.assertEqual(f.run(d,'warning_refine_publish',4,snap)['status'],'ok')
            f.model.Mesh.Children[0].ElementSize='0.25 [mm]'
            old=f.model.Mesh.Children[0].ElementSize
            r=f.run(d,'warning_refine_publish',5,snap)
            self.assertEqual(r['status'],'error')
            self.assertIn('人工修改',r.get('error',''))
            self.assertEqual(f.model.Mesh.Children[0].ElementSize,old)
