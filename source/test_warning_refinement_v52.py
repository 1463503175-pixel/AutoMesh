# coding: utf-8
"""R5.2 warning hotspot tests. Only mock ACT; never invokes ANSYS or remesh."""
import pathlib
import json
import tempfile
import unittest
from test_worker_mock import ModelType, API, QualityTypes, WORKER

class MockNode:
    def __init__(self, ident, x, y, z, body=1):
        self.Id=ident;self.X=x;self.Y=y;self.Z=z;self.BodyIds=[body]
class MockElement:
    def __init__(self, nodes):self.CornerNodeIds=nodes
class MockMeshData:
    Unit='mm'
    def __init__(self, n, e):self.n=n;self.e=e
    def NodeById(self, i):return self.n[i]
    def ElementById(self, i):return self.e[i]
class MockGroup:
    def __init__(self):
        self.Name='Official_Skewness_Warnings';self.ObjectId=987
        self.Location=type('Selected',(),{'Ids':[101,102,103]})()
class MockSizing:
    DataModelObjectCategory='Sizing'
    def __init__(self, parent, number):
        self.parent=parent;self.ObjectId=2000+number;self.Name='';self.Location=None
        self.Type=None;self.ElementSize=None
    def Delete(self):self.parent.Children.remove(self)
class MockCS:
    def __init__(self,parent,number):
        self.parent=parent;self.ObjectId=3000+number;self.Name=''
    def SetOriginLocation(self,*v):self.origin=v
    def Delete(self):self.parent.Children.remove(self)
class MockCSRoot:
    def __init__(self):self.Children=[]
    def AddCoordinateSystem(self):
        o=MockCS(self,len(self.Children)+1);self.Children.append(o);return o
class WarnModel(ModelType):
    def __init__(self):
        super().__init__()
        self.Mesh.Elements=1200;self.Mesh.Nodes=1800
        self.CoordinateSystems=MockCSRoot()
        self.Mesh.GetVolumeMeshQualityCountFailed=lambda metric:0
        self.Mesh.GetVolumeMeshQualityWarningCountFailed=lambda metric:3
        self.Mesh.CreateMQVolumeElementsNamedSelection=lambda metric,code:self.NamedSelections.Children.append(MockGroup())
        def add_sizing():
            obj=MockSizing(self.Mesh,len(self.Mesh.Children)+1)
            self.Mesh.Children.append(obj);return obj
        self.Mesh.AddSizing=add_sizing

class Fixture:
    def __init__(self):
        self.model=WarnModel();self.api=API()
        nodes={};elements={};nextid=1
        for eid,cx in [(101,0.),(102,1.2),(103,50.)]:
            ids=[]
            for x,y,z in [(cx,0,0),(cx+1,0,0),(cx,1,0),(cx,0,1)]:
                nodes[nextid]=MockNode(nextid,x,y,z)
                ids.append(nextid);nextid+=1
            elements[eid]=MockElement(ids)
        self.meshdata=MockMeshData(nodes,elements)
        self.api.DataModel=type('ModelData',(),{
            'Project':type('Project',(),{'ProjectDirectory':'E:/ALD/One'})(),
            'GeoData':type('Geo',(),{'Assemblies':API.DataModel.GeoData.Assemblies,'Unit':'mm'})(),
            'MeshDataByName':lambda _, name:self.meshdata})()
    def run(self,folder,phase,seq,identity=None):
        p=pathlib.Path(folder)
        job={'job_id':format(seq,'032x'),'phase':phase,'base_mm':12.0}
        if identity is not None:
            job['expected_project']=identity['project']
            job['geometry_fingerprint']=identity['fingerprint']
        (p/'job.json').write_text(json.dumps(job),'utf8')
        # Historical ACT-only tests explicitly simulate a UI-reviewed subset.
        # Production NEVER auto-selects; tests may disable this with
        # f.test_auto_review=False to assert the user authorization boundary.
        if phase=='warning_refine_publish' and getattr(self,'test_auto_review',True):
            plan_path=p/'warning_refinement_plan.json'
            if plan_path.is_file():
                plan=json.loads(plan_path.read_text('utf8'))
                if plan.get('status')=='ready':
                    limit=getattr(self,'test_review_limit',24)
                    selected=[x['candidate_id'] for x in plan['clusters'][:limit]]
                    (p/'warning_refinement_selection.json').write_text(json.dumps({
                        'schema_version':1,'plan_id':plan['plan_id'],
                        'project':plan['project'],'geometry_fingerprint':plan['geometry_fingerprint'],
                        'mesh_elements':plan['mesh_elements'],
                        'candidate_ids':selected,
                        'manually_reviewed_candidate_ids':selected,
                    }),'utf8')
        for file in ['geometry_rules.py','warning_refinement.py']:
            (p/file).write_bytes((WORKER.parent/file).read_bytes())
        globals={'JOB_FILE':str(p/'job.json'),'RESULT_FILE':str(p/'last_result.json'),
                 'SNAPSHOT_FILE':str(p/'snapshot.json'),'ExtAPI':self.api,
                 'Model':self.model,'MeshMetricType':QualityTypes,
                 'SizingType':type('ST',(),{'SphereOfInfluence':'SphereOfInfluence'}),
                 'Quantity':lambda s:s,'SelectionTypeEnum':type('S',(),{'GeometryEntities':1})(),
                 'unicode':str}
        globals['execfile']=lambda f:exec(compile(pathlib.Path(f).read_text('utf8'),str(f),'exec'),globals)
        globals['execfile'](str(WORKER))
        return json.loads((p/'last_result.json').read_text('utf8'))
    def marked(self,d):
        self.assertEqual(self.run(d,'inspect',1)['status'],'ok')
        snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
        self.assertEqual(self.run(d,'mark_warning',2,snap)['status'],'ok')
        return snap
    def assertEqual(self,a,b):
        if a!=b:raise AssertionError((a,b))

class WarningRefinementTests(unittest.TestCase):
    def test_preview_and_publish_only_after_review(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            preview=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(preview['status'],'ok',preview.get('error'))
            p=preview['summary'];self.assertEqual(p['warning_count'],3)
            self.assertGreaterEqual(p['group_count'],1)
            self.assertEqual(p['covered_count'],3)
            self.assertFalse(f.model.Mesh.Children)
            publish=f.run(d,'warning_refine_publish',4,snap)
            self.assertEqual(publish['status'],'ok',publish.get('error'))
            self.assertEqual(publish['summary']['status'],'published')
            self.assertEqual(len(f.model.Mesh.Children),p['group_count'])
            self.assertEqual(len(f.model.CoordinateSystems.Children),p['group_count'])
            self.assertEqual(f.model.Mesh.count,0)
            for o in f.model.Mesh.Children:
                self.assertEqual(o.Type,'SphereOfInfluence')
                self.assertEqual(o.Location.Ids,[1])
            again=f.run(d,'warning_refine_publish',5,snap)
            self.assertEqual(again['status'],'error')
            self.assertIn('已发布',again.get('error',''))
            self.assertEqual(len(f.model.Mesh.Children),p['group_count'])
    def test_requires_official_marker_not_guessed(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture()
            f.run(d,'inspect',1)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            r=f.run(d,'warning_refine_preview',2,snap)
            self.assertEqual(r['status'],'error')
            self.assertTrue('标记' in r['error'])
    def test_stale_warning_scope_refused(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            f.model.NamedSelections.Children[-1].Location.Ids.append(999)
            r=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(r['status'],'error')
            self.assertIn('作用域',r['error'])
            self.assertFalse(f.model.Mesh.Children)
    def test_unknown_mesh_units_fail_closed(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d);f.meshdata.Unit='unknown'
            r=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(r['status'],'error')
            self.assertIn('单位无法确认',r['error'])
    def test_publish_requires_preview(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            r=f.run(d,'warning_refine_publish',3,snap)
            self.assertEqual(r['status'],'error')
            self.assertIn('候选方案',r['error'])
    def test_no_second_mesh_generation_on_bad_warning(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            p=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(p['status'],'ok')
            f.meshdata.n[1].X+=0.5
            r=f.run(d,'warning_refine_publish',4,snap)
            self.assertEqual(r['status'],'error')
            self.assertIn('变化',r['error'])
            self.assertEqual(f.model.Mesh.count,0)
            self.assertFalse(f.model.Mesh.Children)
    def test_sphere_never_coarsens_existing_manual_sizing(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture()
            manual=MockSizing(f.model.Mesh,1)
            manual.Name='UserManual';manual.ElementSize='0.9 [mm]'
            manual.Location=type('Scope',(),{'Ids':[11]})()
            manual.Suppressed=False
            f.model.Mesh.Children.append(manual)
            snap=f.marked(d)
            r=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(r['status'],'ok',r.get('error'))
            for c in r['summary']['clusters']:
                self.assertLessEqual(c['element_size_mm'],0.81)
            self.assertEqual(len(f.model.Mesh.Children),1)

    def test_partial_publication_failure_rolls_back_new_objects(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            p=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(p['status'],'ok')
            original=f.model.Mesh.AddSizing
            calls=[0]
            def fails_second():
                calls[0]+=1
                if calls[0]>1: raise RuntimeError('Mock add failed')
                return original()
            f.model.Mesh.AddSizing=fails_second
            r=f.run(d,'warning_refine_publish',4,snap)
            self.assertEqual(r['status'],'error')
            self.assertFalse(f.model.Mesh.Children)
            self.assertFalse(f.model.CoordinateSystems.Children)
            self.assertEqual(f.model.Mesh.count,0)

    def test_changed_element_count_refuses_stale_marker(self):
        with tempfile.TemporaryDirectory() as d:
            f=Fixture();snap=f.marked(d)
            f.model.Mesh.Elements+=1
            r=f.run(d,'warning_refine_preview',3,snap)
            self.assertEqual(r['status'],'error')
            self.assertIn('数量已变化',r['error'])

if __name__=='__main__':unittest.main()
