# -*- coding: utf-8 -*-
"""Manual review, >24 candidates, real numeric skew priority, incremental commits.
Mock ACT only: NEVER launches ANSYS or regenerates a mesh.
"""
import json
import pathlib
import tempfile
import unittest
from test_warning_refinement_v52 import Fixture, MockNode, MockElement, MockSizing


def prepare_candidates(f, folder, count=200):
    snap=f.marked(folder)
    nodes={}; elems={}; nodeid=1
    for j in range(count):
        ids=[]
        # Spread into independent candidate regions in the SAME body.
        x=float(j)*30
        for dx,dy,dz in ((0,0,0),(.3,0,0),(0,.3,0),(0,0,.3)):
            nodes[nodeid]=MockNode(nodeid,x+dx,dy,dz)
            ids.append(nodeid);nodeid+=1
        e=MockElement(ids)
        # True scalar exposed by this *mock*. Real ACT 2024R2 may not expose it.
        e.Skewness = 0.9+(j+1)*0.0005
        elems[10000+j]=e
    f.meshdata.n=nodes;f.meshdata.e=elems
    group=f.model.NamedSelections.Children[-1]
    group.Location.Ids=sorted(elems)
    filename=pathlib.Path(folder,'warning_refinement_marker.json')
    marker=json.loads(filename.read_text('utf8'))
    marker['groups'][0]['ids']=group.Location.Ids
    marker['worksheet_count']=count
    filename.write_text(json.dumps(marker))
    result=f.run(folder,'warning_refine_preview',10,snap)
    assert result['status']=='ok',result.get('error')
    return snap,result['summary']


def choose(folder,plan,candidate_ids):
    record=dict(schema_version=1,plan_id=plan['plan_id'],project=plan['project'],
                geometry_fingerprint=plan['geometry_fingerprint'],mesh_elements=plan['mesh_elements'],
                candidate_ids=candidate_ids,manually_reviewed_candidate_ids=candidate_ids)
    pathlib.Path(folder,'warning_refinement_selection.json').write_text(json.dumps(record))


class ManualSelectionTests(unittest.TestCase):
    def test_200_candidates_sorted_real_skew_select_15_then_15(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();f.test_auto_review=False
            snap,plan=prepare_candidates(f,d)
            self.assertEqual(plan['group_count'],200)
            self.assertEqual(plan['covered_count'],200)
            self.assertEqual(plan['uncovered_count'],0)
            self.assertEqual(plan['candidate_max_skew_verified'],200)
            self.assertAlmostEqual(plan['clusters'][0]['skewness_max'],1.0)
            self.assertAlmostEqual(plan['clusters'][-1]['skewness_max'],.9005)
            chosen=[c['candidate_id'] for c in plan['clusters']]
            self.assertEqual(len(set(chosen)),200)
            self.assertEqual(len(f.model.Mesh.Children),0)
            # Critical: no selected receipt = zero ACT mutations.
            refuse=f.run(d,'warning_refine_publish',11,snap)
            self.assertEqual(refuse['status'],'error')
            self.assertIn('勾选',refuse.get('error',''))
            self.assertFalse(f.model.Mesh.Children)
            choose(d,plan,chosen[:15]);first=f.run(d,'warning_refine_publish',12,snap)
            self.assertEqual(first['status'],'ok',first.get('error'))
            self.assertEqual(first['summary']['sizing_count'],15)
            self.assertEqual(first['summary']['batch_count'],3)
            self.assertEqual(len(f.model.Mesh.Children),15)
            old_ids=[x.ObjectId for x in f.model.Mesh.Children]
            choose(d,plan,chosen[15:30]);second=f.run(d,'warning_refine_publish',13,snap)
            self.assertEqual(second['status'],'ok',second.get('error'))
            self.assertEqual(second['summary']['sizing_count'],15)
            self.assertEqual(second['summary']['remaining_count'],170)
            self.assertEqual(len(f.model.Mesh.Children),30)
            self.assertEqual([x.ObjectId for x in f.model.Mesh.Children][:15],old_ids)
            owner=json.loads(pathlib.Path(d,'warning_refinement_managed.json').read_text('utf8'))
            self.assertEqual(len(owner['controls']),30)
            self.assertEqual(len(set(x['candidate_id'] for x in owner['controls'])),30)
            choose(d,plan,chosen[0:1]);again=f.run(d,'warning_refine_publish',14,snap)
            self.assertEqual(again['status'],'error')
            self.assertIn('已发布',again.get('error',''))
            self.assertEqual(len(f.model.Mesh.Children),30)
            self.assertEqual(f.model.Mesh.count,0)

    def test_no_scalar_metric_is_honestly_unknown(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();f.test_auto_review=False
            snap=f.marked(d)
            result=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(result['status'],'ok')
            p=result['summary']
            self.assertEqual(p['candidate_max_skew_verified'],0)
            self.assertTrue(all(c['skewness_max'] is None and not c['skewness_verified'] for c in p['clusters']))

    def test_batch_failure_preserves_prior_commits_without_ghosts(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();f.test_auto_review=False
            snap,plan=prepare_candidates(f,d,20)
            ids=[c['candidate_id'] for c in plan['clusters']]
            choose(d,plan,ids[:13])
            old=f.model.Mesh.AddSizing
            calls=[0]
            def fail_seventh():
                calls[0]+=1
                if calls[0]==7:raise RuntimeError('simulated batch 2 failure')
                return old()
            f.model.Mesh.AddSizing=fail_seventh
            result=f.run(d,'warning_refine_publish',20,snap)
            self.assertEqual(result['status'],'error')
            self.assertIn('第 2 批',result.get('error',''))
            self.assertEqual(len(f.model.Mesh.Children),6)
            self.assertEqual(len(f.model.CoordinateSystems.Children),6)
            history=json.loads(pathlib.Path(d,'warning_refinement_managed.json').read_text('utf8'))
            self.assertEqual(len(history['controls']),6)
            f.model.Mesh.AddSizing=old
            choose(d,plan,ids[6:13])
            retry=f.run(d,'warning_refine_publish',21,snap)
            self.assertEqual(retry['status'],'ok',retry.get('error'))
            self.assertEqual(len(f.model.Mesh.Children),13)
            self.assertEqual(f.model.Mesh.count,0)


class CrossProjectManifestTests(unittest.TestCase):
    def test_preview_ignores_stale_manifest_when_no_managed_controls_are_live(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            pathlib.Path(d,'warning_refinement_managed.json').write_text(json.dumps({
                'schema_version':2,'project':'E:/ALD/Previous','fingerprint':'old-geometry',
                'controls':[{'name':'AutoMesh_QR_old','object_id':777,'body_id':1,
                             'coordinate_system':'AutoMesh_QR_CS_old','cs_object_id':888,
                             'size_mm':1.0,'radius_mm':3.0,'centre_mm':[0,0,0]}]
            }))
            result=f.run(d,'warning_refine_preview',30,snap)
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertGreaterEqual(result['summary']['group_count'],1)
            self.assertFalse(f.model.Mesh.Children)

    def test_preview_refuses_stale_manifest_if_reserved_qr_control_is_live(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            live=MockSizing(f.model.Mesh,99);live.Name='AutoMesh_QR_old'
            f.model.Mesh.Children.append(live)
            pathlib.Path(d,'warning_refinement_managed.json').write_text(json.dumps({
                'schema_version':2,'project':'E:/ALD/Previous','fingerprint':'old-geometry',
                'controls':[{'name':'AutoMesh_QR_old','object_id':777,'body_id':1,
                             'coordinate_system':'AutoMesh_QR_CS_old','cs_object_id':888,
                             'size_mm':1.0,'radius_mm':3.0,'centre_mm':[0,0,0]}]
            }))
            result=f.run(d,'warning_refine_preview',31,snap)
            self.assertEqual(result['status'],'error')
            self.assertTrue('Sphere of Influence' in result.get('error','') or '其他工程/几何' in result.get('error',''), result.get('error',''))

    def test_publish_can_start_fresh_after_old_project_controls_are_absent(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();f.test_auto_review=False;snap=f.marked(d)
            preview=f.run(d,'warning_refine_preview',32,snap)
            self.assertEqual(preview['status'],'ok',preview.get('error'))
            plan=preview['summary'];chosen=[plan['clusters'][0]['candidate_id']]
            choose(d,plan,chosen)
            pathlib.Path(d,'warning_refinement_managed.json').write_text(json.dumps({
                'schema_version':2,'project':'E:/ALD/Previous','fingerprint':'old-geometry',
                'controls':[{'name':'AutoMesh_QR_old','object_id':777,'body_id':1,
                             'coordinate_system':'AutoMesh_QR_CS_old','cs_object_id':888,
                             'size_mm':1.0,'radius_mm':3.0,'centre_mm':[0,0,0]}]
            }))
            published=f.run(d,'warning_refine_publish',33,snap)
            self.assertEqual(published['status'],'ok',published.get('error'))
            self.assertEqual(len(f.model.Mesh.Children),1)


if __name__=='__main__':unittest.main()

class NearDuplicateTests(unittest.TestCase):
    def test_selected_near_coincident_spheres_are_rejected_before_mutation(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();f.test_auto_review=False
            snap,plan=prepare_candidates(f,d,3)
            # Directly move two geometry locations in the mock model so the
            # next ACT preview produces near-duplicate SOIs from distinct IDs.
            ids=sorted(f.meshdata.e)
            source=f.meshdata.e[ids[1]].CornerNodeIds
            for n in source: f.meshdata.n[n].X-=29.8
            refreshed=f.run(d,'warning_refine_preview',11,snap)
            self.assertEqual(refreshed['status'],'ok',refreshed.get('error'))
            p=refreshed['summary']
            selected=[c['candidate_id'] for c in p['clusters']]
            # They may cluster into a single influence area. Never fabricate
            # duplicates from separate candidates merely to hit the condition.
            if len(selected)>=2:
                choose(d,p,selected[:2])
                result=f.run(d,'warning_refine_publish',12,snap)
                if result['status']=='error':
                    self.assertIn('重合',result.get('error',''))
                    self.assertEqual(len(f.model.Mesh.Children),0)

class R79CompatibilityTests(unittest.TestCase):
    def test_old_published_sizing_without_candidate_id_is_identified(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();f.test_auto_review=False
            snap,plan=prepare_candidates(f,d,10)
            ids=[c['candidate_id'] for c in plan['clusters']]
            choose(d,plan,ids[:2]);r=f.run(d,'warning_refine_publish',12,snap)
            self.assertEqual(r['status'],'ok',r.get('error'))
            manifest=pathlib.Path(d,'warning_refinement_managed.json')
            legacy=json.loads(manifest.read_text('utf8'))
            for control in legacy['controls']:
                del control['candidate_id']
                del control['cell_budget_proxy']
            legacy['schema_version']=1
            manifest.write_text(json.dumps(legacy))
            choose(d,plan,ids[2:4]);r=f.run(d,'warning_refine_publish',13,snap)
            self.assertEqual(r['status'],'ok',r.get('error'))
            self.assertEqual(len(f.model.Mesh.Children),4)
            reconciled=json.loads(manifest.read_text('utf8'))
            self.assertEqual(reconciled['schema_version'],3)
            self.assertEqual(len(set(x['candidate_id'] for x in reconciled['controls'])),4)

    def test_changed_mesh_element_count_invalidates_pending_candidates(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();f.test_auto_review=False
            snap,plan=prepare_candidates(f,d,10)
            choose(d,plan,[plan['clusters'][0]['candidate_id']])
            f.model.Mesh.Elements+=1
            r=f.run(d,'warning_refine_publish',12,snap)
            self.assertEqual(r['status'],'error')
            self.assertEqual(len(f.model.Mesh.Children),0)
            self.assertEqual(f.model.Mesh.count,0)
