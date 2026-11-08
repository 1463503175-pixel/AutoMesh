# coding: utf-8
"""Mock-only regression for editable ACT controls. Not an ANSYS 2024R2 integration test."""
import json
import pathlib
import tempfile
import unittest
from test_worker_mock import Face, Named, NamedSelections, ModelType, WORKER, QualityTypes, Manager, API

class MockNamed(Named):
    def __init__(self,name,ids):
        Named.__init__(self,name,ids)
        self.SendToSolver=True

class MockSizing:
    DataModelObjectCategory='Sizing'
    def __init__(self):
        self.Name='';self.ScopingMethod=None;self.NamedSelection=None;self.ElementSize=None;self.Suppressed=False
    @property
    def Location(self):
        return self.NamedSelection if self.NamedSelection else None
    @Location.setter
    def Location(self, ns):
        # 2024 R2 ACT: Sizing.Location accepts a native Named Selection.
        self.NamedSelection = ns
        self.ScopingMethod = 'Component'

class EditableModel(ModelType):
    def __init__(self):
        ModelType.__init__(self)
        self.NamedSelections.Children=[MockNamed('inlet',[11]),MockNamed('outlet',[12])]
        self.Mesh.AddSizing=self._add
    def _add(self):
        obj=MockSizing();self.Mesh.Children.append(obj);return obj
    def AddNamedSelection(self):
        obj=MockNamed('',[]);self.NamedSelections.Children.append(obj);return obj

class Fixture:
    def __init__(self, model):
        self.model=model
        faces=[Face(11,0,[0,0,1]),Face(12,3,[0,0,-1]),
               Face(31,60,[0,0,1]),Face(32,63,[0,0,-1])]
        body=type('Body',(),{'Id':1,'Name':'fluid','Volume':100000,'Faces':faces})()
        part=type('Part',(),{'Bodies':[body]})()
        assy=type('Assy',(),{'Parts':[part]})()
        self.API=API()
        self.API.DataModel=type('Data',(),{'Project':type('Project',(),{'ProjectDirectory':'E:/test/FFF'})(),
          'GeoData':type('Geo',(),{'Unit':'StandardNMM','Assemblies':[assy]})()})()
    def run(self,folder,phase,id_num,expected=None,plan=None,**options):
        folder=pathlib.Path(folder)
        job={'job_id':format(id_num,'032x'),'phase':phase,'base_mm':12.0}
        if expected is not None:
            job['expected_project']=expected['project']
            job['geometry_fingerprint']=expected['fingerprint']
        if plan is not None:job['plan']=plan
        job.update(options)
        (folder/'job.json').write_text(json.dumps(job),'utf8')
        (folder/'geometry_rules.py').write_bytes((WORKER.parent/'geometry_rules.py').read_bytes())
        (folder/'warning_refinement.py').write_bytes((WORKER.parent/'warning_refinement.py').read_bytes())
        context={'JOB_FILE':str(folder/'job.json'),'RESULT_FILE':str(folder/'last_result.json'),
         'SNAPSHOT_FILE':str(folder/'snapshot.json'),'ExtAPI':self.API,
         'Model':self.model,'MeshMetricType':getattr(self,'quality_types',QualityTypes),
         'Quantity':lambda s:s,'SelectionTypeEnum':type('S',(),{'GeometryEntities':1})(),
         'GeometryDefineByType':type('E',(),{'Component':'Component'})(),
         'unicode':str}
        def execfile(filename):
            exec(compile(pathlib.Path(filename).read_text('utf8'),str(filename),'exec'),context)
        context['execfile']=execfile
        context.update(getattr(self,'native_namespace',{}))
        execfile(WORKER)
        return json.loads((folder/'last_result.json').read_text('utf8'))

def opt_in_reviewed_area_groups(snapshot, base_mm=12):
    from assets.geometry_rules import classify_face_areas
    groups, _ = classify_face_areas(snapshot, base_mm)
    return [g['group_key'] for g in groups if g['category'] != 'named_existing']

class EditableRegression(unittest.TestCase):
    def inspect(self,fixture,dir,n):
        result=fixture.run(dir,'inspect',n)
        self.assertEqual(result['status'],'ok',result.get('error'))
        return json.loads((pathlib.Path(dir)/'snapshot.json').read_text('utf8'))

    def test_publish_then_user_edits_preserved_then_generates(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel();f=Fixture(m)
            snap=self.inspect(f,d,1)
            result=f.run(d,'prepare',2,snap,{'local_overrides':{},'disabled_regions':[],
                                    'enabled_groups':opt_in_reviewed_area_groups(snap)})
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertFalse(result['summary']['generate_mesh_called'])
            self.assertEqual(m.Mesh.count,0)
            created=[n for n in m.NamedSelections.Children if n.Name.startswith('AM_')]
            self.assertTrue(created,result)
            self.assertTrue(all(n.SendToSolver is False for n in created))
            linked=[c for c in m.Mesh.Children if c.Name.startswith('AutoMesh_')]
            self.assertTrue(any(c.NamedSelection in created for c in linked))
            # User reduces the named scope and adjusts the local size in ANSYS.
            selected=next(c for c in linked if c.NamedSelection in created)
            selected.NamedSelection.Location.Ids=[31]
            selected.ElementSize='1 [mm]'
            snap2=self.inspect(f,d,3)
            repeat=f.run(d,'prepare',4,snap2,{'local_overrides':{},'disabled_regions':[],
                                    'enabled_groups':opt_in_reviewed_area_groups(snap)})
            self.assertEqual(repeat['status'],'ok',repeat.get('error'))
            self.assertEqual(selected.ElementSize,'1 [mm]')
            self.assertEqual(selected.NamedSelection.Location.Ids,[31])
            self.assertEqual(repeat['summary']['created_sizing_count'],0)
            reviewed=f.run(d,'generate_reviewed',5,snap2)
            self.assertEqual(reviewed['status'],'ok',reviewed.get('error'))
            self.assertEqual(m.Mesh.count,1)
            self.assertTrue(reviewed['summary']['manual_sizing_preserved'])
            self.assertEqual(selected.ElementSize,'1 [mm]')
            # Generated mesh cannot be overwritten by another execution.
            second=f.run(d,'generate_reviewed',6,snap2)
            self.assertEqual(second['status'],'error')
            self.assertEqual(m.Mesh.count,1)

    def test_out_of_range_manual_group_size_is_rejected_before_generation(self):
        # v0.10.1 groups by area, not by wafer-gap guesses. Ensure that
        # editing a published Sizing to an invalid value still blocks a mesh.
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel();f=Fixture(m)
            snap=self.inspect(f,d,31)
            first=f.run(d,'prepare',32,snap,{'local_overrides':{},'disabled_regions':[],
                         'enabled_groups':opt_in_reviewed_area_groups(snap)})
            self.assertEqual(first['status'],'ok',first.get('error'))
            group=next(c for c in m.Mesh.Children if c.Name.startswith('AutoMesh_'))
            group.ElementSize='501 [mm]'
            rejected=f.run(d,'generate_reviewed',33,snap)
            self.assertEqual(rejected['status'],'error')
            self.assertIn('Manual Sizing exceeds safe range',rejected['error'])
            self.assertEqual(m.Mesh.count,0)

    def test_prepare_without_named_api_fails_before_mesh_change(self):
        with tempfile.TemporaryDirectory() as d:
            m=ModelType();f=Fixture(m)
            snap=self.inspect(f,d,11)
            result=f.run(d,'prepare',12,snap,{'local_overrides':{},'disabled_regions':[]})
            self.assertEqual(result['status'],'error')
            self.assertIn('AddNamedSelection',result['error'])
            self.assertEqual(m.Mesh.count,0)
            self.assertEqual(m.Mesh.Children,[])

    def test_prepare_cannot_use_old_fingerprint(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel();f=Fixture(m)
            snap=self.inspect(f,d,21)
            snap['fingerprint']='wrong'
            result=f.run(d,'prepare',22,snap,{'local_overrides':{},'disabled_regions':[]})
            self.assertEqual(result['status'],'error')
            self.assertEqual(len(m.Mesh.Children),0)

if __name__=='__main__':unittest.main()
