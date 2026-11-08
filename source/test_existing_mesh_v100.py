# coding: utf-8
"""Local ACT mock regression: preserve existing mesh and user controls."""
import tempfile
import pathlib
import unittest
from test_editable_groups import EditableModel, Fixture, opt_in_reviewed_area_groups

class ExistingMeshPublicationTests(unittest.TestCase):
    def inspect(self, f, folder, ident):
        result=f.run(folder,'inspect',ident)
        self.assertEqual(result['status'],'ok',result.get('error'))
        import json
        return json.loads((pathlib.Path(folder)/'snapshot.json').read_text('utf8'))

    def test_names_only_with_existing_mesh_no_sizing_or_generate(self):
        with tempfile.TemporaryDirectory() as folder:
            model=EditableModel();model.Mesh.Elements=9500;model.Mesh.Nodes=13500
            f=Fixture(model);snap=self.inspect(f,folder,1)
            boundary=list(model.NamedSelections.Children)
            result=f.run(folder,'publish_names',2,snap,{'local_overrides':{},'disabled_regions':[],
                      'enabled_groups':opt_in_reviewed_area_groups(snap)})
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(result['summary']['created_sizing_count'],0)
            self.assertTrue(result['summary']['mesh_preserved'])
            self.assertEqual((model.Mesh.Elements,model.Mesh.Nodes,model.Mesh.count),(9500,13500,0))
            self.assertEqual(len(model.Mesh.Children),0)
            self.assertEqual(model.NamedSelections.Children[:2],boundary)
            created=[n for n in model.NamedSelections.Children if n.Name.startswith('AM_')]
            self.assertTrue(created)
            self.assertTrue(all(not n.SendToSolver for n in created))
            original_scopes={n.Name:list(n.Location.Ids) for n in created}
            # Before another publication the source must be re-scanned.
            stale=f.run(folder,'publish_names',3,snap,{'local_overrides':{},'disabled_regions':[],
                      'enabled_groups':opt_in_reviewed_area_groups(snap)})
            self.assertEqual(stale['status'],'error')
            self.assertIn('Named selections changed',stale.get('error',''))
            snap2=self.inspect(f,folder,4)
            repeat=f.run(folder,'publish_names',5,snap2,{'local_overrides':{},'disabled_regions':[],
                      'enabled_groups':opt_in_reviewed_area_groups(snap)})
            self.assertEqual(repeat['status'],'ok',repeat.get('error'))
            self.assertEqual(repeat['summary']['created_named_count'],0)
            self.assertEqual({n.Name:list(n.Location.Ids) for n in created},original_scopes)

    def test_meshed_sizing_explicit_consent_and_no_clear_or_generate(self):
        with tempfile.TemporaryDirectory() as folder:
            model=EditableModel();model.Mesh.Elements=8000;model.Mesh.Nodes=11000
            f=Fixture(model);snap=self.inspect(f,folder,11)
            rejected=f.run(folder,'prepare',12,snap,{'local_overrides':{},'disabled_regions':[],
                      'enabled_groups':opt_in_reviewed_area_groups(snap)})
            self.assertEqual(rejected['status'],'error')
            self.assertIn('explicit Sizing publication consent required',rejected['error'])
            self.assertEqual(model.Mesh.Children,[])
            global_reject=f.run(folder,'prepare',13,snap,{'local_overrides':{},'disabled_regions':[],
                      'enabled_groups':opt_in_reviewed_area_groups(snap)},
                                allow_existing_mesh_sizing=True,apply_global=True)
            self.assertEqual(global_reject['status'],'error')
            self.assertIn('global size overwrite prohibited',global_reject['error'])
            ok=f.run(folder,'prepare',14,snap,{'local_overrides':{},'disabled_regions':[],
                      'enabled_groups':opt_in_reviewed_area_groups(snap)},
                     allow_existing_mesh_sizing=True)
            self.assertEqual(ok['status'],'ok',ok.get('error'))
            self.assertEqual(ok['summary']['mesh_elements_before'],8000)
            self.assertTrue(ok['summary']['mesh_preserved'])
            self.assertFalse(ok['summary']['generate_mesh_called'])
            self.assertEqual(model.Mesh.count,0)
            self.assertTrue(model.Mesh.Children)

    def test_generated_mesh_requires_real_ansys_confirmation_and_preserves_old_on_unavailable_dialog(self):
        with tempfile.TemporaryDirectory() as folder:
            model=EditableModel();model.Mesh.Elements=1200
            f=Fixture(model);snap=self.inspect(f,folder,21)
            result=f.run(folder,'prepare',22,snap,{'local_overrides':{},'disabled_regions':[],
                      'enabled_groups':opt_in_reviewed_area_groups(snap)},
                         allow_existing_mesh_sizing=True)
            self.assertEqual(result['status'],'ok',result.get('error'))
            result=f.run(folder,'generate_reviewed',23,snap)
            self.assertEqual(result['status'],'error')
            self.assertEqual(model.Mesh.count,0)
            self.assertIn('Cannot obtain interactive remesh confirmation',result['error'])

if __name__=='__main__': unittest.main()
