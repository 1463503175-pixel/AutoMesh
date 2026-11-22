# coding: utf-8
"""Large-marked-selection regression: mock ACT only, never starts ANSYS.

Covers the user's 11,671 bad-element case. All mesh data is synthetic and is
not offered as a physical validation of local sizing or mesh-quality changes.
"""
import json
import pathlib
import tempfile
import unittest

from test_warning_refinement_v52 import Fixture, MockNode, MockElement


class LargeWarningBatchTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        self.fixture = Fixture()
        self.fixture.model.Mesh.Elements = 40000
        self.fixture.model.Mesh.Nodes = 60000
        self.snap = self.fixture.marked(self.root)

    def set_marker(self, count, populate=True):
        f = self.fixture
        ids = list(range(10001, 10001 + count))
        group = f.model.NamedSelections.Children[-1]
        group.Location.Ids = ids
        if populate:
            nodes = f.meshdata.n
            elements = f.meshdata.e
            next_id = max(nodes) + 1
            for k, eid in enumerate(ids):
                # Compact two-dimensional cluster, in real mm coordinate units.
                x = (k % 128) * 0.27
                y = (k // 128) * 0.27
                ids4 = []
                for dx, dy, dz in [(0, 0, 0), (.3, 0, 0), (0, .3, 0), (0, 0, .3)]:
                    nodes[next_id] = MockNode(next_id, x + dx, y + dy, dz)
                    ids4.append(next_id)
                    next_id += 1
                elements[eid] = MockElement(ids4)
        file = self.root / 'warning_refinement_marker.json'
        marker = json.loads(file.read_text('utf8'))
        marker['groups'][0]['ids'] = ids
        marker['worksheet_count'] = count
        file.write_text(json.dumps(marker), 'utf8')
        return ids

    def test_11671_elements_produce_complete_bounded_review_and_can_publish_one(self):
        self.set_marker(11671)
        f = self.fixture
        preview = f.run(self.root, 'warning_refine_preview', 3, self.snap)
        self.assertEqual(preview['status'], 'ok', preview.get('error'))
        plan = preview['summary']
        self.assertEqual(plan['warning_count'], 11671)
        self.assertTrue(plan['analysis_complete'])
        self.assertEqual(plan['analysis_processed_count'], 11671)
        self.assertEqual(plan['analysis_batch_size'], 1024)
        self.assertEqual(plan['analysis_batch_count'], 12)
        self.assertEqual(plan['status'], 'ready')
        self.assertGreater(plan['group_count'], 0)
        self.assertGreater(plan['covered_count'], 0)
        self.assertEqual(plan['covered_count'] + plan['uncovered_count'], 11671)
        self.assertLessEqual(plan['group_count'], 4096)
        self.assertEqual(sum(len(c['element_ids']) for c in plan['clusters']), plan['covered_count'])
        self.assertFalse(f.model.Mesh.Children, 'Preview must never create Sizing')
        f.test_review_limit = 1
        publish = f.run(self.root, 'warning_refine_publish', 4, self.snap)
        self.assertEqual(publish['status'], 'ok', publish.get('error'))
        self.assertEqual(publish['summary']['sizing_count'], 1)
        self.assertEqual(f.model.Mesh.count, 0, 'Never call GenerateMesh')
        self.assertEqual(len(f.model.Mesh.Children), 1)

    def test_very_large_total_guard_rejects_before_act_mesh_reads(self):
        self.set_marker(65537, populate=False)
        f = self.fixture
        r = f.run(self.root, 'warning_refine_preview', 3, self.snap)
        self.assertEqual(r['status'], 'error')
        self.assertIn('65536', r['error'])
        self.assertFalse(f.model.Mesh.Children)

    def test_batched_preview_rejects_changed_marker_without_publication(self):
        ids = self.set_marker(4110)
        f = self.fixture
        preview = f.run(self.root, 'warning_refine_preview', 3, self.snap)
        self.assertEqual(preview['status'], 'ok', preview.get('error'))
        f.model.NamedSelections.Children[-1].Location.Ids = ids[:-1]
        result = f.run(self.root, 'warning_refine_publish', 4, self.snap)
        self.assertEqual(result['status'], 'error')
        self.assertIn('作用域', result.get('error', ''))
        self.assertFalse(f.model.Mesh.Children)


if __name__ == '__main__':
    unittest.main()
