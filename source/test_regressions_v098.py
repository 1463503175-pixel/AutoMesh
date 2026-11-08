# coding: utf-8
"""Regression for confirmed release blockers; mock only, not an ANSYS acceptance."""
import json
import pathlib
import tempfile
import unittest
from test_editable_groups import EditableModel, Fixture, opt_in_reviewed_area_groups


class BoundaryAndExportRegression(unittest.TestCase):
    def setup_published(self,folder, disabled=None):
        m=EditableModel();fixture=Fixture(m)
        first=fixture.run(folder,'inspect',1)
        self.assertEqual(first['status'],'ok',first.get('error'))
        s=json.loads((pathlib.Path(folder)/'snapshot.json').read_text('utf8'))
        plan={'local_overrides':{},'disabled_regions':disabled or [],
              'enabled_groups':opt_in_reviewed_area_groups(s)}
        prepared=fixture.run(folder,'prepare',2,s,plan)
        self.assertEqual(prepared['status'],'ok',prepared.get('error'))
        return m,fixture,s

    def test_missing_inlet_after_prepare_blocks_generation_even_if_port_sizing_disabled(self):
        with tempfile.TemporaryDirectory() as folder:
            m,f,s=self.setup_published(folder,['named_inlet','named_outlet'])
            m.NamedSelections.Children[:]=[x for x in m.NamedSelections.Children if x.Name!='inlet']
            out=f.run(folder,'generate_reviewed',3,s)
            self.assertEqual(out['status'],'error')
            self.assertIn('inlet',out['error'])
            self.assertEqual(m.Mesh.count,0)

    def test_boundary_solver_transfer_off_blocks_generation(self):
        with tempfile.TemporaryDirectory() as folder:
            m,f,s=self.setup_published(folder)
            target=next(x for x in m.NamedSelections.Children if x.Name=='inlet')
            target.SendToSolver=False
            out=f.run(folder,'generate_reviewed',3,s)
            self.assertEqual(out['status'],'error')
            self.assertIn('Send to Solver',out['error'])
            self.assertEqual(m.Mesh.count,0)

    def test_boundary_solver_transfer_off_blocks_publication(self):
        with tempfile.TemporaryDirectory() as folder:
            m=EditableModel();f=Fixture(m)
            f.run(folder,'inspect',1)
            s=json.loads((pathlib.Path(folder)/'snapshot.json').read_text('utf8'))
            m.NamedSelections.Children[0].SendToSolver=False
            out=f.run(folder,'prepare',2,s,{'disabled_regions':[]})
            self.assertEqual(out['status'],'error')
            self.assertEqual(m.Mesh.Children,[])

    def test_duplicate_inlet_name_blocks_before_generation(self):
        with tempfile.TemporaryDirectory() as folder:
            m,f,s=self.setup_published(folder)
            m.NamedSelections.Children.append(type(m.NamedSelections.Children[0])('inlet',[11]))
            out=f.run(folder,'generate_reviewed',3,s)
            self.assertEqual(out['status'],'error')
            self.assertIn('Duplicate named boundary',out['error'])
            self.assertEqual(m.Mesh.count,0)

    def test_report_has_three_state_quality_and_no_confused_pass(self):
        with tempfile.TemporaryDirectory() as folder:
            m,f,s=self.setup_published(folder)
            out=f.run(folder,'generate_reviewed',3,s)
            self.assertEqual(out['status'],'ok',out.get('error'))
            self.assertEqual(out['summary']['quality_status'],'warning')
            self.assertNotEqual(out['summary']['quality_gate'],'preliminary_pass')
            self.assertEqual(out['summary']['boundary']['send_to_solver_verified'],True)

    def test_preliminary_pass_when_both_real_metrics_are_available(self):
        with tempfile.TemporaryDirectory() as folder:
            m,f,s=self.setup_published(folder)
            m.Mesh.Maximum.Value=0.86
            m.Mesh.Minimum.Value=0.30
            f.quality_types=type('QualityTypesVerified',(),{
                'Skewness':'Skewness','OrthogonalQuality':'OrthogonalQuality'})
            result=f.run(folder,'generate_reviewed',3,s)
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(result['summary']['quality_status'],'pass')
            self.assertEqual(result['summary']['quality_gate'],'preliminary_pass')

    def test_quality_fail_is_exposed_in_reviewed_generation_result(self):
        with tempfile.TemporaryDirectory() as folder:
            m,f,s=self.setup_published(folder)
            m.Mesh.Maximum.Value=0.98
            result=f.run(folder,'generate_reviewed',3,s)
            self.assertEqual(result['status'],'ok',result.get('error'))
            self.assertEqual(result['summary']['quality_status'],'fail')
            self.assertEqual(result['summary']['quality_gate'],'critical_skewness')

    def test_explicit_export_only_and_fluent_warning(self):
        with tempfile.TemporaryDirectory() as folder:
            m,f,s=self.setup_published(folder)
            generated=f.run(folder,'generate_reviewed',3,s)
            self.assertEqual(generated['status'],'ok',generated.get('error'))
            outpath=str(pathlib.Path(folder)/'final.msh')
            noack=f.run(folder,'export',4,s,export_path=outpath)
            self.assertEqual(noack['status'],'error')
            self.assertFalse(pathlib.Path(outpath).exists())
            allowed=f.run(folder,'export',5,s,export_path=outpath,
                          quality_review_acknowledged=True)
            self.assertEqual(allowed['status'],'ok',allowed.get('error'))
            self.assertEqual(allowed['summary']['quality_status'],'warning')
            self.assertEqual(allowed['summary']['boundary']['inlet_faces'],1)
            self.assertTrue(pathlib.Path(outpath).is_file())
            original=pathlib.Path(outpath).read_bytes()
            duplicate=f.run(folder,'export',6,s,export_path=outpath,
                            quality_review_acknowledged=True)
            self.assertEqual(duplicate['status'],'error')
            self.assertEqual(pathlib.Path(outpath).read_bytes(),original)
            self.assertEqual(m.Mesh.count,1)

    def test_critical_skew_blocks_export(self):
        with tempfile.TemporaryDirectory() as folder:
            m,f,s=self.setup_published(folder)
            generated=f.run(folder,'generate_reviewed',3,s)
            self.assertEqual(generated['status'],'ok',generated.get('error'))
            m.Mesh.Maximum.Value=0.98
            outpath=str(pathlib.Path(folder)/'bad.msh')
            refused=f.run(folder,'export',4,s,export_path=outpath,
                          quality_review_acknowledged=True)
            self.assertEqual(refused['status'],'error')
            self.assertIn('quality gate',refused['error'])
            self.assertFalse(pathlib.Path(outpath).exists())

    def test_boundary_after_generation_is_checked_at_export(self):
        with tempfile.TemporaryDirectory() as folder:
            m,f,s=self.setup_published(folder)
            self.assertEqual(f.run(folder,'generate_reviewed',3,s)['status'],'ok')
            next(x for x in m.NamedSelections.Children if x.Name=='outlet').SendToSolver=False
            outpath=str(pathlib.Path(folder)/'bad_boundary.msh')
            out=f.run(folder,'export',4,s,export_path=outpath,
                      quality_review_acknowledged=True)
            self.assertEqual(out['status'],'error')
            self.assertIn('Send to Solver',out['error'])
            self.assertFalse(pathlib.Path(outpath).exists())

if __name__=='__main__':unittest.main()

from assets.geometry_rules import evaluate_refinement_budget

class VolumeBudgetRegression(unittest.TestCase):
    def test_requested_global_sizing_has_secondary_volume_guard(self):
        model={'geometry_unit':'mm','faces':[{'id':1,'area':10}],
               'bodies':[{'volume':4e10}]}
        group=[{'category':'circular_cylindrical','mm':10,'ids':[1]}]
        with self.assertRaisesRegex(ValueError,'global volume budget'):
            evaluate_refinement_budget(model,group,base_mm=10,enforce_global=True)
        # Existing user global settings are not assumed to equal requested base.
        r=evaluate_refinement_budget(model,group,base_mm=10,enforce_global=False)
        self.assertFalse(r['global_guard_applied'])
        self.assertGreater(r['global_volume_proxy'],r['volume_proxy_limit'])
