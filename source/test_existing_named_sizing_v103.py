# coding: utf-8
"""Regression of exact existing-NS sizing. All tests use local ACT mocks, not ANSYS."""
import pathlib
import tempfile
import unittest
from test_editable_groups import EditableModel, Fixture, MockNamed

class ExistingNamedSizingTests(unittest.TestCase):
    def setup_scene(self):
        m=EditableModel()
        a=MockNamed('AM_Hole_01',[31]);a.SendToSolver=False
        b=MockNamed('4inch',[32])
        m.NamedSelections.Children.extend([a,b])
        return m, Fixture(m)

    def inspect(self,f,d,n=1):
        import json
        r=f.run(d,'inspect',n)
        self.assertEqual(r['status'],'ok',r.get('error'))
        return json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))

    def test_publish_only_selected_existing_names_no_new_names_or_mesh(self):
        with tempfile.TemporaryDirectory() as d:
            m,f=self.setup_scene();snap=self.inspect(f,d)
            before=[(x.Name,list(x.Location.Ids)) for x in m.NamedSelections.Children]
            result=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'AM_Hole_01':1.8}})
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(result['summary']['created_named_count'],0)
            self.assertEqual(result['summary']['created_sizing_count'],1)
            self.assertEqual(m.Mesh.count,0)
            self.assertEqual([(x.Name,list(x.Location.Ids)) for x in m.NamedSelections.Children],before)
            sizing=m.Mesh.Children[0]
            self.assertIs(sizing.NamedSelection,m.NamedSelections.Children[2])
            self.assertAlmostEqual(float(sizing.ElementSize.split(' ')[0]),1.8)
            self.assertTrue(sizing.Name.startswith('AutoMesh_NS_'))
            self.assertEqual(m.Mesh.ElementSize,'12mm')
            self.assertFalse(result['summary']['global_sizing_changed'])
            # R3: previously verified AutoMesh-owned controls update on explicit republish.
            # Unknown/manual controls remain protected by the manifest ownership check.
            sizing.ElementSize='2.5 [mm]'
            result2=f.run(d,'publish_existing_sizing',3,snap,{'existing_named_sizes':{'AM_Hole_01':1.5}})
            self.assertEqual(result2['status'],'ok',result2.get('error'))
            self.assertEqual(result2['summary']['updated_sizing_count'],1)
            self.assertEqual(sizing.ElementSize,'1.5 [mm]')

    def test_exclude_inlet_and_reject_overlap_and_missing_names(self):
        with tempfile.TemporaryDirectory() as d:
            m,f=self.setup_scene();snap=self.inspect(f,d)
            for n,plan,fragment in [
                (2,{'inlet':2},'inlet/outlet'),
                (3,{'AM_Hole_01':1,'missing':2},'命名组校验未通过'),
                (4,{'AM_Hole_01':0.1},'0.35-500'),
            ]:
                r=f.run(d,'publish_existing_sizing',n,snap,{'existing_named_sizes':plan})
                self.assertEqual(r['status'],'error')
                self.assertIn(fragment,r['error'])
            self.assertEqual(len(m.Mesh.Children),0)
            other=MockNamed('AM_Overlap',[31]);other.SendToSolver=False
            m.NamedSelections.Children.append(other)
            r=f.run(d,'publish_existing_sizing',5,snap,{'existing_named_sizes':{'AM_Hole_01':1.2,'AM_Overlap':1.3}})
            self.assertEqual(r['status'],'error')
            self.assertIn('overlap',r['error'].lower())
            self.assertEqual(len(m.Mesh.Children),0)

    def test_preexisting_mesh_requires_permission_no_rebuild(self):
        with tempfile.TemporaryDirectory() as d:
            m,f=self.setup_scene();m.Mesh.Elements=1000;snap=self.inspect(f,d)
            no=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'4inch':3.5}})
            self.assertEqual(no['status'],'error')
            ok=f.run(d,'publish_existing_sizing',3,snap,{'existing_named_sizes':{'4inch':3.5}},allow_existing_mesh_sizing=True)
            self.assertEqual(ok['status'],'ok',ok.get('error'))
            self.assertEqual(m.Mesh.Elements,1000)
            self.assertEqual(m.Mesh.count,0)
            self.assertTrue(ok['summary']['mesh_preserved'])

    def test_wrong_scope_does_not_silently_select_nearby_faces(self):
        with tempfile.TemporaryDirectory() as d:
            m,f=self.setup_scene();snap=self.inspect(f,d)
            m.NamedSelections.Children[2].Location.Ids=[99999]
            r=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'AM_Hole_01':1.5}})
            self.assertEqual(r['status'],'error')
            self.assertIn('valid geometric face scope',r['error'])
            self.assertEqual(m.Mesh.Children,[])

    def test_high_refinement_budget_blocks_before_any_new_sizing(self):
        with tempfile.TemporaryDirectory() as d:
            m,f=self.setup_scene()
            body=f.API.DataModel.GeoData.Assemblies[0].Parts[0].Bodies[0]
            body.Faces[2].Area=30000000  # extremely large target face, 0.35 mm is unsafe
            snap=self.inspect(f,d)
            r=f.run(d,'publish_existing_sizing',2,snap,{'existing_named_sizes':{'AM_Hole_01':0.35}})
            self.assertEqual(r['status'],'error')
            self.assertIn('surface refinement budget exceeded',r['error'])
            self.assertEqual(m.Mesh.Children,[])
            self.assertEqual(m.Mesh.count,0)

if __name__=='__main__':unittest.main()

class LegacySizingReuseTests(unittest.TestCase):
    def test_legacy_sizing_reused_not_duplicated(self):
        import json
        with tempfile.TemporaryDirectory() as d:
            m=EditableModel();f=Fixture(m)
            ns=MockNamed('AM_OldHole',[31]);ns.SendToSolver=False
            m.NamedSelections.Children.append(ns)
            old=m.Mesh.AddSizing();old.Name='AutoMesh_Hole_Old';old.NamedSelection=ns
            old.ElementSize='2 [mm]'
            initial=f.run(d,'inspect',31)
            self.assertEqual(initial['status'],'ok',initial.get('error'))
            snap=json.loads((pathlib.Path(d)/'snapshot.json').read_text('utf8'))
            r=f.run(d,'publish_existing_sizing',32,snap,{'existing_named_sizes':{'AM_OldHole':1.5}})
            self.assertEqual(r['status'],'ok',r.get('error'))
            self.assertEqual(len(m.Mesh.Children),1)
            self.assertEqual(r['summary']['created_sizing_count'],0)
            self.assertEqual(old.ElementSize,'2 [mm]')
            self.assertEqual(r['summary']['existing_named_sizing'][0]['sizing'],'AutoMesh_Hole_Old')
