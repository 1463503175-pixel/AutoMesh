"""Regression guard: the published AutoMesh.exe must never be a Console binary."""
from pathlib import Path
import struct
import tempfile
import unittest

from build_windows_gui import pe_subsystem


class WindowsGUISubsystemTest(unittest.TestCase):
    def test_candidate_is_gui_executable(self):
        exe = Path(__file__).resolve().parent.parent / 'AutoMesh.exe'
        self.assertEqual(pe_subsystem(exe), 2, 'Build with -ldflags=-H=windowsgui')

    def test_parser_detects_a_console_subsystem(self):
        with tempfile.TemporaryDirectory() as folder:
            fake = Path(folder) / 'console.exe'
            payload = bytearray(512)
            payload[0:2] = b'MZ'
            struct.pack_into('<I', payload, 0x3c, 0x80)
            payload[0x80:0x84] = b'PE\0\0'
            struct.pack_into('<H', payload, 0x84, 0x8664)
            struct.pack_into('<H', payload, 0x98, 0x20b)
            struct.pack_into('<H', payload, 0x98 + 68, 3)
            fake.write_bytes(payload)
            self.assertEqual(pe_subsystem(fake), 3)


if __name__ == '__main__':
    unittest.main()
