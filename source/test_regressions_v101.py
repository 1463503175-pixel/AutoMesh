# coding: utf-8
"""v0.10.1 regression checks. Pure geometry & mocked ACT, NOT ANSYS acceptance."""
import json
import pathlib
import tempfile
import unittest
from assets.geometry_rules import classify_face_areas
from test_editable_groups import EditableModel, Fixture

class AreaGroupTests(unittest.TestCase):
    def test_area_order_type_partition_and_no_large_plane_auto_refine(self):
        faces=[{'id':1,'body':1,'type':'Plane','area':40000},
               {'id':2,'body':1,'type':'Cylinder','area':20},
               {'id':3,'body':1,'type':'Cylinder','area':20},
               {'id':4,'body':1,'type':'Plane','area':20}]
        groups,ev=classify_face_areas({'geometry_unit':'mm','faces':faces},8)
        self.assertEqual(ev['valid_area_faces'],4)
        self.assertEqual(len(set(x for g in groups for x in g['ids'])),4)
        big=next(g for g in groups if 1 in g['ids'])
        self.assertEqual(big['category'],'area_review')
        self.assertTrue(any(g['surface_family']=='Cylinder' and set(g['ids'])=={2,3} for g in groups))
        self.assertTrue(any(g['surface_family']=='Plane' and g['ids']==[4] for g in groups))

    def test_larger_repeated_faces_not_lost_to_small_face_buckets(self):
        faces=[{'id':1+i,'body':1,'type':'Cylinder','area':float(2**(i%16))} for i in range(32)]
        faces.extend({'id':100+j,'body':2,'type':'Plane','area':15000} for j in range(320))
        groups,ev=classify_face_areas({'geometry_unit':'mm','faces':faces},8,max_groups=28)
        self.assertTrue(any(g['category']=='area_repeated' and len(g['ids'])==320 for g in groups))
        self.assertTrue(all('wafer' not in g['reason'].lower() for g in groups))

    def test_honors_user_named_groups_without_renaming_inlet_outlet(self):
        state={'geometry_unit':'mm', 'faces':[{'id':1,'body':1,'type':'Plane','area':100},
               {'id':2,'body':1,'type':'Plane','area':100},{'id':3,'body':1,'type':'Plane','area':100}],
               'named':[{'name':'inlet','ids':[1]},{'name':'outlet','ids':[2]},{'name':'4inch','ids':[3]}]}
        groups,_=classify_face_areas(state,8)
        self.assertTrue(any(g['category']=='named_existing' and g['region_key']=='4inch' for g in groups))
        self.assertFalse(any(g['region_key'] in ('inlet','outlet') for g in groups))

class ProgressAndDiagnosisTests(unittest.TestCase):
    def test_meshing_emits_job_matched_progress_on_inspect(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel();f=Fixture(m)
            result=f.run(d,'inspect',1)
            self.assertEqual(result['status'],'ok',result.get('error'))
            data=json.loads((pathlib.Path(d)/('job_progress_'+format(1,'032x')+'.json')).read_text('utf8'))
            self.assertEqual(data['job_id'],format(1,'032x'))
            self.assertEqual(data['percent'],100)
            self.assertEqual(data['stage'],'任务完成')
    def test_diagnose_no_mesh_returns_explicit_status(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel();f=Fixture(m)
            f.run(d,'inspect',1)
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            result=f.run(d,'diagnose',2,snap)
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(result['summary']['element_probe']['status'],'no_mesh')
            data=json.loads((pathlib.Path(d)/('job_progress_'+format(2,'032x')+'.json')).read_text('utf8'))
            self.assertEqual(data['percent'],100)

class StaticUIWiringTests(unittest.TestCase):
    def test_group_labels_show_area_categories_not_internal_keys(self):
        source=pathlib.Path(__file__).with_name('main_windows.go').read_text('utf8')
        self.assertIn('func groupLabel(',source)
        self.assertIn('"小面积"',source)
        self.assertIn('area_mm2_min',source)
        self.assertIn('case "diagnose":',source)
        self.assertIn('case "diagnose":',source)
        self.assertIn('job_progress_',pathlib.Path(__file__).with_name('core.go').read_text('utf8'))

if __name__=='__main__':unittest.main()
