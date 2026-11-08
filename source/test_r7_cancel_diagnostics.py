"""R7: no-solution is terminal diagnostics; cancel is cooperative and scoped."""
import json
import tempfile
import pathlib
import unittest
from test_warning_refinement_v52 import Fixture

class R7Controls(unittest.TestCase):
    def test_cancellation_before_act_mutation_is_terminal(self):
        with tempfile.TemporaryDirectory() as folder:
            f=Fixture()
            snap=f.marked(folder)
            phase=4
            jid=format(phase,'032x')
            (pathlib.Path(folder)/('cancel_'+jid+'.json')).write_text(json.dumps({'job_id':jid,'reason':'operator'}))
            r=f.run(folder,'warning_refine_publish',phase,snap)
            self.assertEqual(r['status'],'cancelled',r.get('error'))
            self.assertEqual(f.model.Mesh.count,0)
            self.assertFalse(f.model.Mesh.Children)
    def test_no_candidate_returns_useful_terminal_report(self):
        with tempfile.TemporaryDirectory() as folder:
            f=Fixture();snap=f.marked(folder)
            for node in f.meshdata.n.values():
                node.X+=30.0*node.Y+50.0*node.Z
            r=f.run(folder,'warning_refine_preview',4,snap)
            self.assertEqual(r['status'],'ok',r.get('error'))
            self.assertEqual(r['summary']['status'],'no_safe_candidates')
            self.assertEqual(r['summary']['group_count'],0)
            self.assertEqual(r['summary']['uncovered_count'],3)
            self.assertTrue(r['summary']['rejected_groups'])
            r2=f.run(folder,'warning_refine_publish',5,snap)
            self.assertEqual(r2['status'],'error')
            self.assertIn('没有可发布',r2.get('error',''))
            self.assertEqual(f.model.Mesh.count,0)
if __name__=='__main__':unittest.main()
