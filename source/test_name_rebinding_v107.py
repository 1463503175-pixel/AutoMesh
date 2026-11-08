# -*- coding: utf-8 -*-
"""Regression: manual/Chinese rename after scan must never create sizing on the wrong faces."""
import json
import pathlib
import tempfile
import unittest
from test_editable_groups import EditableModel, Fixture, MockNamed

class RebindingTests(unittest.TestCase):
    def scene(self):
        m = EditableModel()
        ns = MockNamed('AM_Area_Cylinder_AP12_B7225', [31]); ns.SendToSolver = False
        m.NamedSelections.Children.append(ns)
        f = Fixture(m)
        return m, ns, f

    def snap(self, f, d):
        result=f.run(d,'inspect',1)
        self.assertEqual(result['status'],'ok',result.get('error'))
        return json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf-8'))

    def test_chinese_rename_exact_face_scope_preserves_requested_mm(self):
        with tempfile.TemporaryDirectory() as d:
            m, ns, f = self.scene()
            snap = self.snap(f,d)
            old = ns.Name
            ns.Name = 'AM_小面积圆柱面_Cylinder_AP12_B7225'
            plan={'existing_named_sizes':{old:1.5},'existing_named_scope_ids':{old:[31]}}
            result=f.run(d,'publish_existing_sizing',2,snap,plan)
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(len(m.Mesh.Children),1)
            size=m.Mesh.Children[0]
            self.assertIs(size.NamedSelection,ns)
            self.assertEqual(size.ElementSize,'1.5 [mm]')
            item=result['summary']['existing_named_sizing'][0]
            self.assertEqual(item['remapped_from'],old)
            self.assertEqual(item['named_selection'],ns.Name)
            self.assertFalse(result['summary']['generate_mesh_called'])
            self.assertEqual(m.Mesh.count,0)

    def test_exact_name_but_changed_scope_must_not_bind_stale_selection(self):
        with tempfile.TemporaryDirectory() as d:
            m, ns, f=self.scene(); snap=self.snap(f,d)
            old=ns.Name; ns.Location.Ids=[32]
            r=f.run(d,'publish_existing_sizing',6,snap,{
                'existing_named_sizes':{old:1.5},'existing_named_scope_ids':{old:[31]}})
            self.assertEqual(r['status'],'error')
            self.assertIn('命名组作用面已变化',r['error'])
            self.assertEqual(m.Mesh.Children,[])

    def test_no_matching_face_scope_rejected_without_modification(self):
        with tempfile.TemporaryDirectory() as d:
            m, ns, f=self.scene(); snap=self.snap(f,d); old=ns.Name
            ns.Name='AM_新名字'; ns.Location.Ids=[32]
            r=f.run(d,'publish_existing_sizing',3,snap,{
                'existing_named_sizes':{old:1.5}, 'existing_named_scope_ids':{old:[31]}})
            self.assertEqual(r['status'],'error')
            self.assertIn('命名组校验未通过',r['error'])
            self.assertIn(old,r['error'])
            self.assertEqual(m.Mesh.Children,[])

    def test_duplicate_identical_scope_is_ambiguous(self):
        with tempfile.TemporaryDirectory() as d:
            m, ns, f=self.scene(); snap=self.snap(f,d); old=ns.Name
            ns.Name='AM_中文_01'
            other=MockNamed('AM_中文_02',[31]); other.SendToSolver=False
            m.NamedSelections.Children.append(other)
            r=f.run(d,'publish_existing_sizing',4,snap,{
                'existing_named_sizes':{old:1.5}, 'existing_named_scope_ids':{old:[31]}})
            self.assertEqual(r['status'],'error')
            self.assertIn('same-scope candidates',r['error'])
            self.assertEqual(m.Mesh.Children,[])

    def test_report_all_missing_groups_before_write(self):
        with tempfile.TemporaryDirectory() as d:
            m, ns, f=self.scene(); snap=self.snap(f,d); old=ns.Name
            r=f.run(d,'publish_existing_sizing',5,snap,{
                'existing_named_sizes':{'AM_missing_A':1.5,'AM_missing_B':2},
                'existing_named_scope_ids':{}})
            self.assertEqual(r['status'],'error')
            self.assertIn('AM_missing_A',r['error'])
            self.assertIn('AM_missing_B',r['error'])
            self.assertEqual(m.Mesh.Children,[])

if __name__=='__main__': unittest.main()
