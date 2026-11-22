# coding: utf-8
"""Regression for R8.1 reviewed generation with stale/invalid QR ledgers.

Local ACT mocks only; they do not replace Windows/ANSYS integration tests.
"""
import json
import pathlib
import tempfile
import unittest

from test_editable_groups import EditableModel, Fixture, opt_in_reviewed_area_groups


class ReviewedGenerationQRManifestRegression(unittest.TestCase):
    def prepare(self, folder):
        model = EditableModel()
        fixture = Fixture(model)
        scanned = fixture.run(folder, 'inspect', 1)
        self.assertEqual(scanned['status'], 'ok', scanned.get('error'))
        snap = json.loads((pathlib.Path(folder) / 'snapshot.json').read_text('utf8'))
        setup = fixture.run(folder, 'prepare', 2, snap, {
            'local_overrides': {}, 'disabled_regions': [],
            'enabled_groups': opt_in_reviewed_area_groups(snap),
        })
        self.assertEqual(setup['status'], 'ok', setup.get('error'))
        return model, fixture, snap

    def write_ledger(self, folder, snap, controls):
        (pathlib.Path(folder) / 'warning_refinement_managed.json').write_text(
            json.dumps({'schema_version': 3, 'project': snap['project'],
                        'fingerprint': snap['fingerprint'], 'controls': controls,
                        'rounds': []}), 'utf8')

    def test_empty_stale_qr_ledger_allows_named_sizing_generation(self):
        with tempfile.TemporaryDirectory() as directory:
            model, fixture, snap = self.prepare(directory)
            self.write_ledger(directory, snap, [])
            outcome = fixture.run(directory, 'generate_reviewed', 3, snap)
            self.assertEqual(outcome['status'], 'ok', outcome.get('error'))
            self.assertEqual(model.Mesh.count, 1)
            self.assertEqual(outcome['summary']['warning_hotspot_controls'], [])

    def test_duplicate_qr_names_still_block_before_generate(self):
        with tempfile.TemporaryDirectory() as directory:
            model, fixture, snap = self.prepare(directory)
            repeated = {'name': 'AutoMesh_QR_collision', 'object_id': 812}
            self.write_ledger(directory, snap, [repeated, repeated])
            outcome = fixture.run(directory, 'generate_reviewed', 3, snap)
            self.assertEqual(outcome['status'], 'error')
            self.assertIn('Duplicate quality-hotspot names', outcome.get('error', ''))
            self.assertEqual(model.Mesh.count, 0)

    def test_recorded_qr_missing_from_live_mesh_still_blocks(self):
        with tempfile.TemporaryDirectory() as directory:
            model, fixture, snap = self.prepare(directory)
            self.write_ledger(directory, snap, [
                {'name': 'AutoMesh_QR_missing', 'object_id': 812}])
            outcome = fixture.run(directory, 'generate_reviewed', 3, snap)
            self.assertEqual(outcome['status'], 'error')
            self.assertIn('Previously published quality-hotspot control missing',
                          outcome.get('error', ''))
            self.assertEqual(model.Mesh.count, 0)


if __name__ == '__main__':
    unittest.main()
