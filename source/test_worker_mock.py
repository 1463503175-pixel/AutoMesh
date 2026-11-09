# coding: utf-8
"""Boundary mock for the old ACT/IronPython worker. Does not validate actual ANSYS APIs."""
import os, json, pathlib, tempfile, unittest

WORKER = pathlib.Path(__file__).parent/'assets'/'meshing_worker.py'

class Face:
    def __init__(self,id,z,normal):
        self.Id=id;self.Area=10000;self.SurfaceType='Plane';self.Centroid=[0,0,z]
        self.normal=normal;self.Edges=[]
    def GetBoundingBox(self):return [-50,-50,self.Centroid[2],50,50,self.Centroid[2]]
    def ParamAtPoint(self,centroid):return [0,0]
    def NormalAtParam(self,u,v):return self.normal
class Body:
    Id=1;Name='fluid';Volume=100000;Faces=[Face(11,0,[0,0,1]),Face(12,3,[0,0,-1])]
class Part: Bodies=[Body()]
class Assy: Parts=[Part()]
class V:
    def __init__(self,n):self.Value=n
class Sizing:
    DataModelObjectCategory='Sizing'
    def __init__(self):self.Name='';self.Location=None;self.ElementSize=None
class Mesh:
    PhysicsPreference='CFD';SolverPreference='Fluent';ElementOrder='Linear'
    ElementSize='12mm';MaximumSize='24mm';GrowthRate=1.3
    def __init__(self):
        self.Children=[];self.Elements=0;self.Nodes=0;self.count=0
        self.Minimum=V(0.1);self.Maximum=V(0.8);self.Average=V(0.4)
        self.InternalObject=type('Exporter',(),{'WriteFluentInputFile':lambda self,p:pathlib.Path(p).write_bytes(b'mock-fluent-mesh')})()
    def AddSizing(self):
        x=Sizing();self.Children.append(x);return x
    def GenerateMesh(self):
        self.count+=1;self.Elements=1200;self.Nodes=1800
class NamedSelections:
    def __init__(self):
        self.Children=[Named('inlet',[11]),Named('outlet',[12])]
class Named:
    def __init__(self,name,ids):
        self.Name=name
        self.SendToSolver=True
        self.Location=type('Location',(),{'Ids':ids})()
class Geometry: LengthUnit='StandardNMM'
class ModelType:
    NamedSelections=NamedSelections()
    Geometry=Geometry()
    def __init__(self):self.Mesh=Mesh();self.NamedSelections=NamedSelections()
class Project: ProjectDirectory=r'E:\ALD\model_files'
class Data:
    Project=Project()
    GeoData=type('Geo',(),{'Assemblies':[Assy()]})()
class Manager:
    def CreateSelectionInfo(self,_):return type('Info',(),{'Ids':[]})()
class API:
    DataModel=Data()
    Application=type('App',(),{'ActiveUnitSystem':'StandardNMM'})()
    SelectionManager=Manager()
class QualityTypes:
    Skewness='Skewness';OrthogonalAngle='OrthogonalAngle'

class WorkerTests(unittest.TestCase):
    def run_job(self, folder, job, model, expect_error=False):
        folder=pathlib.Path(folder)
        (folder/'job.json').write_text(json.dumps(job), encoding='utf8')
        (folder/'geometry_rules.py').write_bytes((WORKER.parent/'geometry_rules.py').read_bytes())
        (folder/'warning_refinement.py').write_bytes((WORKER.parent/'warning_refinement.py').read_bytes())
        scope={'JOB_FILE':str(folder/'job.json'),
               'RESULT_FILE':str(folder/'last_result.json'),
               'SNAPSHOT_FILE':str(folder/'snapshot.json'),
               'ExtAPI':API(),'Model':model,'MeshMetricType':QualityTypes,
               'Quantity':lambda x:x,
               'SelectionTypeEnum':type('S',(),{'GeometryEntities':1})(),
               'unicode':str}
        def execfile(path):
            exec(compile(pathlib.Path(path).read_text('utf8'),str(path),'exec'),scope)
        scope['execfile']=execfile
        if expect_error:
            with self.assertRaises(RuntimeError):execfile(WORKER)
        else:execfile(WORKER)
        return json.loads((folder/'last_result.json').read_text('utf8'))
    def test_auto_once_and_unit_gap(self):
        with tempfile.TemporaryDirectory() as d:
            model=ModelType()
            pre={'job_id':'c'*32,'logical_job_id':'d'*32,'attempt_id':'e'*32,
                 'attempt_number':1,'phase':'inspect'}
            pre_result=self.run_job(d,pre,model)
            self.assertEqual(pre_result['logical_job_id'],pre['logical_job_id'])
            self.assertEqual(pre_result['attempt_id'],pre['attempt_id'])
            self.assertEqual(pre_result['attempt_number'],1)
            progress=json.loads((pathlib.Path(d)/('job_progress_'+'c'*32+'.json')).read_text('utf8'))
            self.assertEqual(progress['logical_job_id'],pre['logical_job_id'])
            self.assertEqual(progress['attempt_id'],pre['attempt_id'])
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            j={'job_id':'a'*32,'phase':'auto','base_mm':12,'apply_global':True,
               'expected_project':snap['project'],'geometry_fingerprint':snap['fingerprint']}
            r=self.run_job(d,j,model)
            self.assertEqual(r['status'],'ok')
            self.assertEqual(model.Mesh.count,1)
            self.assertEqual(len(model.Mesh.Children),2)
            self.assertEqual(r['summary']['quality_gate'],'oq_unverified_manual_review')
            self.assertEqual(r['summary']['geometry_evidence']['gap_mm_min'],3)
            self.assertEqual(r['summary']['geometry_evidence']['explicit_boundaries']['inlet_faces'],1)
            self.run_job(d,j,model,True)
            self.assertEqual(model.Mesh.count,1)

    def test_lightweight_sync_does_not_write_or_replace_geometry_snapshot(self):
        with tempfile.TemporaryDirectory() as d:
            model=ModelType()
            snapshot=pathlib.Path(d)/'snapshot.json'
            sentinel='{"keep":"previous geometry snapshot"}'
            snapshot.write_text(sentinel,encoding='utf8')
            result=self.run_job(d,{'job_id':'8'*32,'phase':'sync_status'},model)
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(result['summary']['read_mode'],'lightweight_metadata_only')
            self.assertFalse(result['summary']['geometry_identity_verified'])
            self.assertEqual(result['summary']['body_count'],1)
            self.assertEqual(result['summary']['named_count'],2)
            self.assertEqual(result['summary']['control_count'],0)
            self.assertTrue(result['summary']['geometry_quick_signature'])
            self.assertEqual(snapshot.read_text('utf8'),sentinel)
            self.assertEqual(model.Mesh.count,0)

    def test_quality_check_without_snapshot_is_readonly_and_does_not_force_full_scan(self):
        with tempfile.TemporaryDirectory() as d:
            model=ModelType();model.Mesh.Elements=1200;model.Mesh.Nodes=1600
            result=self.run_job(d,{'job_id':'9'*32,'phase':'quality_check'},model)
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(result['summary']['mesh_elements'],1200)
            self.assertFalse(result['summary']['geometry_identity_verified'])
            self.assertTrue(result['summary']['mesh_signature'])
            self.assertIn('quality',result['summary'])
            self.assertFalse((pathlib.Path(d)/'snapshot.json').exists())
            self.assertEqual(model.Mesh.count,0)
    def test_identical_mesh_signature_in_sync_and_full_snapshot(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType();m.Mesh.Elements=1200;m.Mesh.Nodes=1600
            first=self.run_job(d,{'job_id':'a'*32,'phase':'inspect'},m)
            self.assertEqual(first['status'],'ok')
            full=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            quick=self.run_job(d,{'job_id':'b'*32,'phase':'sync_status'},m)
            self.assertEqual(quick['status'],'ok')
            self.assertEqual(quick['summary']['mesh_signature'],full['mesh_signature'])

    def test_no_identity_quality_ignores_unrelated_saved_snapshot(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType();m.Mesh.Elements=1200;m.Mesh.Nodes=1600
            fp=pathlib.Path(d)/'snapshot.json'
            fp.write_text('\x7b"unrelated":"outdated snapshot"\x7d',encoding='utf8')
            quality=self.run_job(d,{'job_id':'c'*32,'phase':'quality_check'},m)
            self.assertEqual(quality['status'],'ok', quality.get('error'))
            self.assertFalse(quality['summary']['geometry_identity_verified'])
            self.assertEqual(quality['summary']['mesh_elements'],1200)
            self.assertIn('unrelated',fp.read_text('utf8'))

    def test_no_mesh_status_is_not_measured_not_old_quality(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType()
            for phase in ('quality_check','diagnose'):
                job={'job_id':('e' if phase=='quality_check' else 'f')*32,'phase':phase}
                result=self.run_job(d,job,m)
                self.assertEqual(result['status'],'ok',result.get('error'))
                self.assertEqual(result['summary']['quality_gate'],'no_mesh')
                self.assertEqual(result['summary']['quality_status'],'not_measured')
                self.assertNotIn('quality',result['summary'])

    def test_explicit_export_uses_saved_verified_snapshot_without_full_rescan(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType();m.Mesh.Elements=1200;m.Mesh.Nodes=1600
            self.assertEqual(self.run_job(d,{'job_id':'a'*32,'phase':'inspect'},m)['status'],'ok')
            previous=(pathlib.Path(d)/'snapshot.json').read_bytes()
            snap=json.loads(previous)
            output=str(pathlib.Path(d)/'read_only_export.msh')
            # Full snapshot calls ParamAtPoint; export must only run the quick
            # live identity check against the existing verified snapshot.
            face=Body.Faces[0]
            method=face.ParamAtPoint
            def reject_full_scan(_): raise AssertionError('Unexpected full geometry scan')
            face.ParamAtPoint=reject_full_scan
            try:
                result=self.run_job(d,{'job_id':'b'*32,'phase':'export',
                    'expected_project':snap['project'],'geometry_fingerprint':snap['fingerprint'],
                    'export_path':output,'quality_review_acknowledged':True},m)
            finally:
                face.ParamAtPoint=method
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertTrue(pathlib.Path(output).is_file())
            self.assertEqual(previous,(pathlib.Path(d)/'snapshot.json').read_bytes())
            self.assertEqual(m.Mesh.count,0)

    def test_auto_onepass_single_dispatch(self):
        with tempfile.TemporaryDirectory() as d:
            model=ModelType()
            job={'job_id':'5'*32,'phase':'auto_onepass','base_mm':12,
                 'apply_global':False}
            result=self.run_job(d,job,model)
            self.assertEqual(result['status'],'ok')
            self.assertEqual(model.Mesh.count,1)
            self.assertEqual(result['summary']['geometry_evidence']['gap_mm_min'],3)
            self.assertTrue(all(x['native_sizing']['applied']=='uniform' for x in result['summary']['configured']))
            self.assertEqual(result['summary']['quality_gate'],'oq_unverified_manual_review')
            self.run_job(d,job,model,True)
            self.assertEqual(model.Mesh.count,1)

    def test_auto_does_not_modify_a_different_model(self):
        with tempfile.TemporaryDirectory() as d:
            model=ModelType()
            job={'job_id':'d'*32,'phase':'auto','base_mm':12,'apply_global':True,
                 'expected_project':'E:/different/project','geometry_fingerprint':'wrong'}
            result=self.run_job(d,job,model)
            self.assertEqual(result['status'],'error')
            self.assertEqual(model.Mesh.count,0)
            self.assertEqual(len(model.Mesh.Children),0)

    def test_exact_worksheet_failed_group_only_after_opt_in(self):
        with tempfile.TemporaryDirectory() as d:
            model=ModelType()
            model.Mesh.Elements=1200;model.Mesh.Nodes=1600
            model.NamedSelections.Children=[]
            model.Mesh.GetVolumeMeshQualityCountFailed=lambda metric:3
            called=[]
            class NewNamed:
                Name='MQ_Skewness_Failed';ObjectId=991
                Location=type('Location',(),{'Ids':[10,11,12]})()
            def marker(metric,failcode):
                called.append((metric,failcode))
                model.NamedSelections.Children.append(NewNamed())
            model.Mesh.CreateMQVolumeElementsNamedSelection=marker
            self.run_job(d,{'job_id':'1'*32,'phase':'inspect'},model)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            result=self.run_job(d,{'job_id':'2'*32,'phase':'mark_failed',
                'expected_project':snap['project'],'geometry_fingerprint':snap['fingerprint']},model)
            self.assertEqual(result['status'],'ok')
            self.assertEqual(called,[('Skewness',1)])
            self.assertEqual(result['summary']['worksheet_failed_count'],3)
            self.assertEqual(result['summary']['selected_element_ids_sample'],[10,11,12])
            self.assertEqual(model.Mesh.count,0)

    def test_readonly_element_probe_unavailable_is_honest(self):
        with tempfile.TemporaryDirectory() as d:
            model=ModelType()
            model.Mesh.Elements=1200;model.Mesh.Nodes=1600
            self.run_job(d,{'job_id':'3'*32,'phase':'inspect'},model)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            result=self.run_job(d,{'job_id':'4'*32,'phase':'diagnose',
                'expected_project':snap['project'],'geometry_fingerprint':snap['fingerprint']},model)
            self.assertEqual(result['status'],'ok')
            self.assertEqual(result['summary']['element_probe']['status'],'unavailable')
            self.assertEqual(model.Mesh.count,0)

    def test_explicit_export_is_provisional_when_oq_unavailable(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType()
            path=str(pathlib.Path(d)/'fluid.msh')
            j={'job_id':'6'*32,'phase':'auto_onepass','base_mm':12,
               'apply_global':False,'export_path':path,'quality_review_acknowledged':True}
            r=self.run_job(d,j,m)
            self.assertEqual(r['status'],'ok')
            self.assertEqual(r['summary']['quality_gate'],'oq_unverified_manual_review')
            self.assertTrue(pathlib.Path(path).is_file())
            self.assertEqual(r['summary']['export_validation'],
                             'preliminary_only__Fluent_quality_check_required')

    def test_high_skew_rejects_export(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType()
            m.Mesh.Maximum.Value=0.99
            path=str(pathlib.Path(d)/'bad.msh')
            j={'job_id':'7'*32,'phase':'auto_onepass','base_mm':12,
               'apply_global':False,'export_path':path,'quality_review_acknowledged':True}
            r=self.run_job(d,j,m)
            self.assertEqual(r['status'],'ok')
            self.assertFalse(pathlib.Path(path).exists())
            self.assertEqual(r['summary']['export'],'skipped_critical_quality_gate')

    def test_interrupted_autosizing_can_resume_only_exact_matching_scope_and_size(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType()
            self.run_job(d,{'job_id':'d'*32,'phase':'inspect'},m)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            previous=Sizing()
            previous.Name='AutoMesh_named_inlet_1'
            previous.Location=type('Selection',(),{'Ids':[11]})()
            previous.ElementSize='3.96 [mm]'
            m.Mesh.Children.append(previous)
            self.run_job(d,{'job_id':'9'*32,'phase':'inspect'},m)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            j={'job_id':'e'*32,'phase':'auto','base_mm':12,
               'expected_project':snap['project'],'geometry_fingerprint':snap['fingerprint'],
               'plan':{'resume_owned_controls':True}}
            r=self.run_job(d,j,m)
            self.assertEqual(r['status'],'ok',r.get('error'))
            self.assertEqual(r['summary']['reused_control_count'],1)
            self.assertEqual(m.Mesh.count,1)
            self.assertEqual(sum(c.Name=='AutoMesh_named_inlet_1' for c in m.Mesh.Children),1)

    def test_interrupted_autosizing_mismatch_is_blocked_before_new_sizing(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType()
            self.run_job(d,{'job_id':'d'*32,'phase':'inspect'},m)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            previous=Sizing()
            previous.Name='AutoMesh_named_inlet_1'
            previous.Location=type('Selection',(),{'Ids':[12]})()
            previous.ElementSize='3.96 [mm]'
            m.Mesh.Children.append(previous)
            self.run_job(d,{'job_id':'9'*32,'phase':'inspect'},m)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            j={'job_id':'e'*32,'phase':'auto','base_mm':12,
               'expected_project':snap['project'],'geometry_fingerprint':snap['fingerprint'],
               'plan':{'resume_owned_controls':True}}
            r=self.run_job(d,j,m)
            self.assertEqual(r['status'],'error')
            self.assertIn('scope differs',r['error'])
            self.assertEqual(m.Mesh.count,0)
            self.assertEqual(len(m.Mesh.Children),1)

    def test_oversmall_user_size_fails_before_creating_any_control(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType()
            self.run_job(d,{'job_id':'d'*32,'phase':'inspect'},m)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            # Larger real surface area than 32 GiB conservative surface budget.
            for face in Body.Faces: face.Area=1000000
            # Re-inspect because geometry fingerprint depends on area.
            self.run_job(d,{'job_id':'f'*32,'phase':'inspect'},m)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            j={'job_id':'e'*32,'phase':'auto','base_mm':12,
               'expected_project':snap['project'],'geometry_fingerprint':snap['fingerprint'],
               'plan':{'local_overrides':{'named_inlet':0.35,'named_outlet':0.35}}}
            r=self.run_job(d,j,m)
            self.assertEqual(r['status'],'error')
            self.assertIn('32GB',r['error'])
            self.assertEqual(m.Mesh.count,0)
            self.assertFalse(m.Mesh.Children)
            for face in Body.Faces: face.Area=10000

    def test_interrupted_autosizing_never_reused_blindly(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType()
            old=Sizing();old.Name='AutoMesh_narrow_parallel_1';m.Mesh.Children.append(old)
            j={'job_id':'8'*32,'phase':'auto_onepass','base_mm':12}
            r=self.run_job(d,j,m)
            self.assertEqual(r['status'],'error')
            self.assertIn('unfinished AutoMesh sizing',r['error'])
            self.assertEqual(m.Mesh.count,0)

    def test_inspect_without_side_effects(self):
        with tempfile.TemporaryDirectory() as d:
            model=ModelType()
            j={'job_id':'b'*32,'phase':'inspect'}
            r=self.run_job(d,j,model)
            self.assertEqual(r['status'],'ok')
            self.assertEqual(r['summary']['geometry_unit'],'StandardNMM')
            self.assertEqual(r['summary']['display_length_unit'],'mm')
            self.assertEqual(model.Mesh.count,0)
            self.assertEqual(len(model.Mesh.Children),0)

    def test_missing_outlet_blocks_before_any_mesh_changes(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType()
            m.NamedSelections.Children=[Named('inlet',[11])]
            result=self.run_job(d,{'job_id':'e'*32,'phase':'auto_onepass','base_mm':12},m)
            self.assertEqual(result['status'],'error')
            self.assertIn('outlet missing',result['error'])
            self.assertEqual(len(m.Mesh.Children),0)
            self.assertEqual(m.Mesh.count,0)

    def test_local_refinement_disable_and_override(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType()
            self.run_job(d,{'job_id':'1'*32,'phase':'inspect'},m)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            result=self.run_job(d,{'job_id':'9'*32,'phase':'auto','base_mm':12,
              'expected_project':snap['project'],'geometry_fingerprint':snap['fingerprint'],
              'plan':{'disabled_regions':['narrow_parallel'],
                'local_overrides':{'named_inlet':1.75,'named_outlet':1.75}}},m)
            self.assertEqual(result['status'],'ok')
            self.assertEqual(len(m.Mesh.Children),2)
            self.assertTrue(all('named_' in x['name'] for x in result['summary']['configured']))
            self.assertEqual({x['mm'] for x in result['summary']['configured']},{1.75})

if __name__=='__main__':unittest.main()
