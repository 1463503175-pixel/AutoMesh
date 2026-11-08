# coding: utf-8
"""The R5 real-world regression: Mesh.ElementSize returned '0.0 [mm]'.
Mock-only; never touches a live ANSYS project.
"""
import json
import pathlib
import tempfile
import unittest
from test_editable_groups import EditableModel, Fixture, MockNamed, opt_in_reviewed_area_groups

class GlobalZeroTests(unittest.TestCase):
    def test_publish_existing_names_with_zero_auto_global(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel()
            m.Mesh.ElementSize='0.0 [mm]'
            ns=MockNamed('AM_Cylinder_1',[31]);ns.SendToSolver=False
            m.NamedSelections.Children.append(ns)
            f=Fixture(m)
            self.assertEqual(f.run(d,'inspect',1)['status'],'ok')
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf-8'))
            r=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'AM_Cylinder_1':2.0}})
            self.assertEqual(r['status'],'ok',r.get('error'))
            self.assertEqual(r['summary']['resource_preflight']['global_size_status'],'automatic_or_unresolved')
            self.assertFalse(r['summary']['resource_preflight'].get('global_guard_applied',False))
            self.assertEqual(m.Mesh.ElementSize,'0.0 [mm]')
            self.assertEqual(m.Mesh.count,0)
            self.assertEqual(len(m.Mesh.Children),1)
            self.assertEqual(m.Mesh.Children[0].ElementSize,'2 [mm]')
            r2=f.run(d,'publish_existing_sizing',3,snap,{'existing_named_sizes':{'AM_Cylinder_1':2.5}})
            self.assertEqual(r2['status'],'ok',r2.get('error'))
            self.assertEqual(m.Mesh.Children[0].ElementSize,'2.5 [mm]')
            self.assertEqual(m.Mesh.count,0)

    def test_automatic_zero_global_cannot_silently_remesh(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel()
            m.Mesh.ElementSize='0.0 [mm]'
            f=Fixture(m)
            self.assertEqual(f.run(d,'inspect',1)['status'],'ok')
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf-8'))
            r=f.run(d,'prepare',2,snap,{'enabled_groups':opt_in_reviewed_area_groups(snap)})
            self.assertEqual(r['status'],'ok',r.get('error'))
            result=f.run(d,'generate_reviewed',3,snap)
            self.assertEqual(result['status'],'error')
            self.assertIn('审核生成前无法确认',result['error'])
            self.assertEqual(m.Mesh.count,0)
            self.assertEqual(m.Mesh.ElementSize,'0.0 [mm]')

    def test_invalid_positive_global_still_blocks_publishing(self):
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel()
            m.Mesh.ElementSize='0.05 [mm]'
            ns=MockNamed('AM_Cylinder_1',[31]);ns.SendToSolver=False
            m.NamedSelections.Children.append(ns)
            f=Fixture(m)
            self.assertEqual(f.run(d,'inspect',1)['status'],'ok')
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf-8'))
            r=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'AM_Cylinder_1':2.0}})
            self.assertEqual(r['status'],'error')
            self.assertIn('Actual Meshing global ElementSize outside',r['error'])
            self.assertFalse(m.Mesh.Children)
            self.assertEqual(m.Mesh.count,0)

if __name__=='__main__': unittest.main()
