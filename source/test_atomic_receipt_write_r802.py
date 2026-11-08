# -*- coding: utf-8 -*-
"""Regression tests for the R8.0.2 ACT receipt writer."""
import json
import os
import pathlib
import sys
import tempfile
import time
import types
import unittest

WORKER = pathlib.Path(__file__).parent / 'assets' / 'meshing_worker.py'


class FakeOS(object):
    name = 'nt'
    path = os.path

    @staticmethod
    def close(fd):
        os.close(fd)

    @staticmethod
    def fsync(fd):
        os.fsync(fd)

    @staticmethod
    def isfile(path):
        return os.path.isfile(path)

    @staticmethod
    def remove(path):
        return os.remove(path)

    @staticmethod
    def rename(source, destination):
        if FakeOS.fail_temp_rename and '.amwrite_' in os.path.basename(source):
            FakeOS.fail_temp_rename = False
            raise IOError('simulated Windows rename failure')
        return os.rename(source, destination)



def load_writer():
    source = WORKER.read_text(encoding='utf-8')
    start = source.index('def write(path, data):')
    end = source.index('\n\ndef read(path):', start)
    namespace = {
        'os': FakeOS,
        'json': json,
        'tempfile': tempfile,
        'time': time,
        'utext': lambda value: str(value),
    }
    exec(compile(source[start:end], str(WORKER), 'exec'), namespace)
    return namespace['write']


class ReceiptWriterTests(unittest.TestCase):
    def setUp(self):
        self.tempdir = tempfile.TemporaryDirectory()
        self.folder = pathlib.Path(self.tempdir.name)
        self.old_system = sys.modules.get('System')
        self.old_io = sys.modules.get('System.IO')
        sys.modules.pop('System', None)
        sys.modules.pop('System.IO', None)
        FakeOS.fail_temp_rename = False
        self.write = load_writer()

    def tearDown(self):
        if self.old_system is None:
            sys.modules.pop('System', None)
        else:
            sys.modules['System'] = self.old_system
        if self.old_io is None:
            sys.modules.pop('System.IO', None)
        else:
            sys.modules['System.IO'] = self.old_io
        self.tempdir.cleanup()

    def test_new_job_receipt_uses_same_directory_rename(self):
        target = self.folder / ('job_' + 'b' * 32 + '.json')
        self.write(str(target), {'status': 'ok', 'job_id': 'b' * 32})
        self.assertEqual(json.loads(target.read_text(encoding='utf-8'))['status'], 'ok')

    def test_existing_receipt_is_replaced_without_dotnet_file_overloads(self):
        target = self.folder / 'last_result.json'
        target.write_text('{"status":"started"}', encoding='utf-8')
        self.write(str(target), {'status': 'ok', 'job_id': 'a' * 32})
        result = json.loads(target.read_text(encoding='utf-8'))
        self.assertEqual(result['status'], 'ok')
        self.assertEqual(result['job_id'], 'a' * 32)
        self.assertEqual(sorted(p.name for p in self.folder.iterdir()), ['last_result.json'])

    def test_failed_temp_rename_restores_previous_receipt(self):
        target = self.folder / 'last_result.json'
        target.write_text('{"status":"old"}', encoding='utf-8')
        FakeOS.fail_temp_rename = True
        # The first fallback attempt fails to move the new temp file; the
        # writer restores the old receipt and retries safely.
        self.write(str(target), {'status': 'new'})
        self.assertEqual(json.loads(target.read_text(encoding='utf-8'))['status'], 'new')



if __name__ == '__main__':
    unittest.main()
