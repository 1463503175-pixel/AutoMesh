# coding: utf-8
"""ACT-mocked small-feature link regression; never connects to ANSYS."""
import pathlib
import tempfile
import unittest
import json
from test_warning_refinement_v52 import Fixture


class ReadOnlyFaceEvidenceTest(unittest.TestCase):
    def test_live_element_face_link_is_preserved_and_publish_still_checks_review(self):
        with tempfile.TemporaryDirectory() as d:
            f = Fixture()
            snap = f.marked(d)
            face = snap['faces'][0]
            eid = f.model.NamedSelections.Children[-1].Location.Ids[0]
            f.meshdata.e[eid].FaceId = face['id']
            preview = f.run(d, 'warning_refine_preview', 3, snap)
            self.assertEqual(preview['status'], 'ok', preview.get('error'))
            groups = preview['summary']['clusters']
            self.assertTrue(groups)
            matches = [g for g in groups if eid in g['element_ids']]
            self.assertEqual(len(matches), 1)
            self.assertEqual(matches[0]['mapped_face_ids'], [face['id']])
            # The read-only plan must not create or delete any Sizing.
            self.assertEqual(f.model.Mesh.Children, [])
            f.test_auto_review = False
            denied = f.run(d, 'warning_refine_publish', 4, snap)
            self.assertEqual(denied['status'], 'error')
            self.assertEqual(f.model.Mesh.Children, [])
            # The same exact preview can still pass the existing reviewed
            # publication protocol; no new auto-face publishing path exists.
            f.test_auto_review = True
            accepted = f.run(d, 'warning_refine_publish', 5, snap)
            self.assertEqual(accepted['status'], 'ok', accepted.get('error'))
            self.assertEqual(f.model.Mesh.count, 0)


if __name__ == '__main__':
    unittest.main()
