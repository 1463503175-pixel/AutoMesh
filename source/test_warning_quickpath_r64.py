"""Verify preview & publication reuse read-only verified snapshot rather than
costly full surface/edge recrawl. ACT mocks cover the real worker behavior."""
import pathlib
import unittest
from test_warning_refinement_v52 import Fixture

BASE=pathlib.Path(__file__).parent
WORKER=(BASE/'assets'/'meshing_worker.py').read_text()

class WarningQuickPathTests(unittest.TestCase):
    def test_verified_snapshot_used_for_warning_actions(self):
        self.assertIn("'mark_warning','mark_high_skew','mark_low_element_quality','warning_refine_preview','warning_refine_publish'",WORKER)
        self.assertIn('state=read_only_verified_snapshot(job)',WORKER)

    def test_preview_publish_after_mark_use_current_same_mesh(self):
        import tempfile,json
        with tempfile.TemporaryDirectory() as folder:
            f=Fixture()
            snap=f.marked(folder)
            first=f.run(folder,'warning_refine_preview',3,snap)
            self.assertEqual(first['status'],'ok',first.get('error'))
            pub=f.run(folder,'warning_refine_publish',4,snap)
            self.assertEqual(pub['status'],'ok',pub.get('error'))
            self.assertEqual(pub['summary']['status'],'published')
            self.assertEqual(f.model.Mesh.count,0)  # never GenerateMesh

if __name__=='__main__':unittest.main()
