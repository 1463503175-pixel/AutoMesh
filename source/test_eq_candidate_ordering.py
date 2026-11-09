# coding: utf-8
"""Narrow mock tests for verifiable Element Quality candidate ranking."""
import json
import pathlib
import tempfile
import unittest

from test_warning_refinement_v52 import Fixture


class ElementQualityRankingTests(unittest.TestCase):
    def preview(self, have_metrics):
        f = Fixture()
        d = tempfile.TemporaryDirectory()
        snap = f.marked(d.name)
        marker_file = pathlib.Path(d.name, 'warning_refinement_marker.json')
        marker = json.loads(marker_file.read_text('utf8'))
        marker['metric'] = 'ElementQuality'
        marker['selection_source'] = 'named_selection_worksheet_element_quality_range'
        marker['range_min'] = 0.0
        marker['range_max'] = 0.1
        marker_file.write_text(json.dumps(marker), 'utf8')
        if have_metrics:
            for eid, value in [(101, .025), (102, .034), (103, .009)]:
                f.meshdata.e[eid].ElementQuality = value
        result = f.run(d.name, 'warning_refine_preview', 4, snap)
        self.assertEqual(result['status'], 'ok', result.get('error'))
        return d, result['summary']

    def test_verified_eq_minimum_ranks_first(self):
        d, summary = self.preview(True)
        try:
            self.assertEqual(summary['quality_metric'], 'ElementQuality')
            clusters = summary['clusters']
            self.assertGreaterEqual(len(clusters), 2)
            self.assertEqual(clusters[0]['element_quality_min'], .009)
            self.assertTrue(all(c['element_quality_verified'] for c in clusters))
            self.assertEqual(clusters[0]['element_quality_source'], 'ACT:IElement.ElementQuality')
        finally:
            d.cleanup()

    def test_missing_actual_eq_not_fabricated(self):
        d, summary = self.preview(False)
        try:
            self.assertTrue(summary['clusters'])
            for candidate in summary['clusters']:
                self.assertIsNone(candidate['element_quality_min'])
                self.assertFalse(candidate['element_quality_verified'])
                self.assertEqual(candidate['element_quality_source'], 'unavailable')
        finally:
            d.cleanup()


if __name__ == '__main__':
    unittest.main()
