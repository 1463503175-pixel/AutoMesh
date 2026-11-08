# coding: utf-8
"""R8.0.4 continuous refinement across a remesh; ACT mock only, no ANSYS launch."""
import json
import pathlib
import tempfile
import unittest

from test_warning_refinement_v52 import Fixture, MockNode, MockElement
from test_r80_manual_selection import choose


class ContinuousRefinementTests(unittest.TestCase):
    def test_second_mesh_round_keeps_first_round_sizing_and_publishes_new_candidate(self):
        with tempfile.TemporaryDirectory() as d:
            f = Fixture()
            f.test_auto_review = False
            snap = f.marked(d)
            first_preview = f.run(d, 'warning_refine_preview', 10, snap)
            self.assertEqual(first_preview['status'], 'ok', first_preview.get('error'))
            first_plan = first_preview['summary']
            first_id = first_plan['clusters'][0]['candidate_id']
            choose(d, first_plan, [first_id])
            first_publish = f.run(d, 'warning_refine_publish', 11, snap)
            self.assertEqual(first_publish['status'], 'ok', first_publish.get('error'))
            self.assertEqual(len(f.model.Mesh.Children), 1)
            old_sizing = f.model.Mesh.Children[0]
            old_sizing_id = old_sizing.ObjectId
            old_cs = f.model.CoordinateSystems.Children[0]
            old_cs_id = old_cs.ObjectId
            old_mesh_count = f.model.Mesh.count

            # Simulate the operator generating a new mesh, then marking a
            # different high-skew element in a distant region of the same Body.
            f.model.Mesh.Elements += 1
            node_map = {}
            ids = []
            for node_id, (dx, dy, dz) in enumerate(((0, 0, 0), (.3, 0, 0), (0, .3, 0), (0, 0, .3)), 100):
                node_map[node_id] = MockNode(node_id, 50.0 + dx, 0.0 + dy, 0.0 + dz)
                ids.append(node_id)
            f.meshdata.n = node_map
            f.meshdata.e = {20001: MockElement(ids)}
            # Keep the same ACT Named Selection object, but refresh its element
            # scope and the marker receipt for the new mesh round.
            group = f.model.NamedSelections.Children[-1]
            group.Location.Ids = [20001]
            marker_path = pathlib.Path(d, 'warning_refinement_marker.json')
            marker = json.loads(marker_path.read_text('utf8'))
            marker['mesh_count'] = int(f.model.Mesh.Elements)
            marker['groups'][0]['ids'] = [20001]
            marker['worksheet_count'] = 1
            marker_path.write_text(json.dumps(marker), encoding='utf8')

            # Do not run a full geometry inspect: the read-only preview phase
            # must refresh the current Mesh/Sizing inventory itself.
            second_preview = f.run(d, 'warning_refine_preview', 13, snap)
            self.assertEqual(second_preview['status'], 'ok', second_preview.get('error'))
            refreshed_snapshot = json.loads(pathlib.Path(d, 'snapshot.json').read_text('utf8'))
            self.assertTrue(refreshed_snapshot['controls_complete'])
            self.assertEqual(len([x for x in refreshed_snapshot['controls'] if x['name'].startswith('AutoMesh_QR_')]), 1)
            self.assertEqual(int(refreshed_snapshot['mesh']['Elements']), int(f.model.Mesh.Elements))
            second_plan = second_preview['summary']
            self.assertEqual(len(second_plan['clusters']), 1)
            second_id = second_plan['clusters'][0]['candidate_id']
            self.assertNotEqual(first_id, second_id)
            choose(d, second_plan, [second_id])

            second_publish = f.run(d, 'warning_refine_publish', 14, snap)
            self.assertEqual(second_publish['status'], 'ok', second_publish.get('error'))
            self.assertEqual(second_publish['summary']['sizing_count'], 1)
            self.assertEqual(second_publish['summary']['existing_control_count'], 1)
            self.assertEqual(second_publish['summary']['cumulative_control_count'], 2)
            self.assertEqual(len(f.model.Mesh.Children), 2)
            self.assertEqual(f.model.Mesh.Children[0].ObjectId, old_sizing_id)
            self.assertEqual(f.model.CoordinateSystems.Children[0].ObjectId, old_cs_id)
            self.assertIs(f.model.Mesh.Children[0], old_sizing)
            self.assertIs(f.model.CoordinateSystems.Children[0], old_cs)
            self.assertEqual(f.model.Mesh.count, old_mesh_count)
            owner = json.loads(pathlib.Path(d, 'warning_refinement_managed.json').read_text('utf8'))
            self.assertEqual(owner['schema_version'], 3)
            self.assertEqual(len(owner['controls']), 2)
            self.assertEqual(len(owner['rounds']), 2)
            self.assertFalse(second_publish['summary']['mesh_generated'])
            self.assertFalse(second_publish['summary']['mesh_cleared'])

    def test_stale_manifest_is_adopted_only_from_exact_live_sizing_readback(self):
        with tempfile.TemporaryDirectory() as d:
            f = Fixture()
            f.test_auto_review = False
            snap = f.marked(d)
            preview = f.run(d, 'warning_refine_preview', 20, snap)
            self.assertEqual(preview['status'], 'ok', preview.get('error'))
            plan = preview['summary']
            chosen = plan['clusters'][0]['candidate_id']
            choose(d, plan, [chosen])
            published = f.run(d, 'warning_refine_publish', 21, snap)
            self.assertEqual(published['status'], 'ok', published.get('error'))
            original = f.model.Mesh.Children[0]
            manifest_path = pathlib.Path(d, 'warning_refinement_managed.json')
            manifest = json.loads(manifest_path.read_text('utf8'))
            manifest['project'] = 'E:/PreviousProject'
            manifest['fingerprint'] = 'old-fingerprint'
            manifest_path.write_text(json.dumps(manifest), encoding='utf8')

            # New marker on a far-away element; old live control remains intact.
            f.model.Mesh.Elements += 1
            nodes = {}
            node_ids = []
            for node_id, (dx, dy, dz) in enumerate(((0, 0, 0), (.3, 0, 0), (0, .3, 0), (0, 0, .3)), 200):
                nodes[node_id] = MockNode(node_id, 60.0 + dx, dy, dz)
                node_ids.append(node_id)
            f.meshdata.n = nodes
            f.meshdata.e = {30001: MockElement(node_ids)}
            group = f.model.NamedSelections.Children[-1]
            group.Location.Ids = [30001]
            marker = json.loads(pathlib.Path(d, 'warning_refinement_marker.json').read_text('utf8'))
            marker['mesh_count'] = int(f.model.Mesh.Elements)
            marker['groups'][0]['ids'] = [30001]
            marker['worksheet_count'] = 1
            pathlib.Path(d, 'warning_refinement_marker.json').write_text(json.dumps(marker), encoding='utf8')
            # The next read-only preview refreshes live Sizing/CS inventory;
            # no full geometry rescan is needed to adopt verified live controls.
            preview2 = f.run(d, 'warning_refine_preview', 23, snap)
            self.assertEqual(preview2['status'], 'ok', preview2.get('error'))
            plan2 = preview2['summary']
            choose(d, plan2, [plan2['clusters'][0]['candidate_id']])
            published2 = f.run(d, 'warning_refine_publish', 24, snap)
            self.assertEqual(published2['status'], 'ok', published2.get('error'))
            self.assertIs(f.model.Mesh.Children[0], original)
            self.assertEqual(len(f.model.Mesh.Children), 2)
            new_manifest = json.loads(manifest_path.read_text('utf8'))
            self.assertEqual(new_manifest['project'], snap['project'])
            self.assertEqual(new_manifest['fingerprint'], snap['fingerprint'])
            self.assertEqual(len(new_manifest['controls']), 2)


if __name__ == '__main__':
    unittest.main()
